package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/Brook-sys/senai-courses-track/internal/notifier"
	"github.com/Brook-sys/senai-courses-track/internal/scraper"
	"github.com/Brook-sys/senai-courses-track/internal/storage"
	"github.com/Brook-sys/senai-courses-track/internal/telegramclient"
)

type Scheduler struct {
	db        *storage.DB
	scraper   *scraper.Scraper
	notifiers *notifier.NotifierManager
	wake      chan struct{}

	mu          sync.Mutex
	running     bool
	lastRun     time.Time
	lastSuccess time.Time
	lastError   error
	nextRun     time.Time
}

func New(db *storage.DB, s *scraper.Scraper) *Scheduler {
	return &Scheduler{
		db:        db,
		scraper:   s,
		notifiers: notifier.NewManager(),
		wake:      make(chan struct{}, 1),
	}
}

func (sch *Scheduler) Start(ctx context.Context, client telegramclient.Client) {
	notifier.RegisterFromConfig(sch.notifiers, sch.db, client)
	go sch.loop(ctx)
	log.Printf("Scheduler started with interval: %s", sch.GetInterval().String())
}

func (sch *Scheduler) loop(ctx context.Context) {
	for {
		interval := sch.GetInterval()

		sch.mu.Lock()
		sch.nextRun = time.Now().Add(interval)
		sch.mu.Unlock()

		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			log.Println("Running scheduled update...")
			sch.RunUpdate(ctx)
		case <-sch.wake:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			log.Printf("Scheduler interval reloaded: %s", sch.GetInterval().String())
		}
	}
}

func (sch *Scheduler) GetInterval() time.Duration {
	value, err := sch.db.GetConfig("sync_interval_minutes")
	if err != nil || value == "" {
		return 24 * time.Hour
	}
	minutes, err := strconv.Atoi(value)
	if err != nil || minutes < 5 {
		return 24 * time.Hour
	}
	return time.Duration(minutes) * time.Minute
}

func (sch *Scheduler) SetIntervalMinutes(minutes string) error {
	m, err := strconv.Atoi(minutes)
	if err != nil {
		return err
	}
	if m < 5 || m > 525600 {
		return fmt.Errorf("interval must be between 5 and 525600 minutes")
	}
	if err := sch.db.SetConfig("sync_interval_minutes", minutes); err != nil {
		return err
	}
	select {
	case sch.wake <- struct{}{}:
	default:
	}
	return nil
}

func (sch *Scheduler) RunUpdate(ctx context.Context) {
	sch.mu.Lock()
	if sch.running {
		sch.mu.Unlock()
		log.Println("RunUpdate skipped: already running")
		return
	}
	sch.running = true
	sch.lastRun = time.Now()
	sch.mu.Unlock()

	defer func() {
		sch.mu.Lock()
		sch.running = false
		sch.mu.Unlock()
	}()

	var runErr error
	defer func() {
		sch.mu.Lock()
		if runErr != nil {
			sch.lastError = runErr
		} else {
			sch.lastError = nil
			sch.lastSuccess = time.Now()
		}
		sch.mu.Unlock()
	}()

	subs, err := sch.db.GetActiveSubscriptions()
	if err != nil {
		runErr = err
		log.Println("error getting subs:", err)
		return
	}

	for _, sub := range subs {
		if ctx.Err() != nil {
			runErr = ctx.Err()
			return
		}

		var filters map[string]string
		if err := json.Unmarshal([]byte(sub.Filters), &filters); err != nil {
			runErr = err
			log.Printf("invalid filters for %s: %v", sub.Name, err)
			continue
		}

		filterKey := buildFilterKey(filters)
		courses, err := sch.scraper.FetchCourses(filters)
		if err != nil {
			runErr = err
			log.Printf("fetch error for %s: %v", sub.Name, err)
			continue
		}

		for _, c := range courses {
			if c.ID == "" {
				continue
			}
			if !sch.db.CourseExists(c.ID, filterKey) {
				sc := storage.StoredCourse{
					ID:        c.ID,
					Title:     c.Title,
					URL:       c.URL,
					FilterKey: filterKey,
					FirstSeen: time.Now(),
				}
				if err := sch.notifiers.NotifyAll(ctx, c, sub.Name); err != nil {
					runErr = err
					log.Printf("Failed to notify %s on any channel, skipping database save to retry later", c.ID)
					continue
				}

				if err := sch.db.SaveCourse(sc); err != nil {
					runErr = err
					log.Printf("save error for %s: %v", c.ID, err)
					continue
				}
				log.Printf("NEW COURSE: %s (sub: %s)", c.Title, sub.Name)
			}
		}
		log.Printf("Updated subscription %s: %d courses", sub.Name, len(courses))
	}
}

func buildFilterKey(f map[string]string) string {
	b, _ := json.Marshal(f)
	return string(b)
}

func (sch *Scheduler) AddSubscription(name string, filters map[string]string) (int64, error) {
	b, _ := json.Marshal(filters)
	return sch.db.CreateSubscription(name, string(b))
}

func (sch *Scheduler) TriggerNow() {
	go sch.RunUpdate(context.Background())
}

type SchedulerStatus struct {
	Running     bool      `json:"running"`
	LastRun     time.Time `json:"lastRun"`
	LastSuccess time.Time `json:"lastSuccess"`
	LastError   string    `json:"lastError,omitempty"`
	NextRun     time.Time `json:"nextRun"`
}

func (sch *Scheduler) Status() SchedulerStatus {
	sch.mu.Lock()
	defer sch.mu.Unlock()

	errStr := ""
	if sch.lastError != nil {
		errStr = sch.lastError.Error()
	}

	return SchedulerStatus{
		Running:     sch.running,
		LastRun:     sch.lastRun,
		LastSuccess: sch.lastSuccess,
		LastError:   errStr,
		NextRun:     sch.nextRun,
	}
}
