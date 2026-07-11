package scheduler

import (
	"encoding/json"
	"log"
	"strconv"
	"time"

	"github.com/Brook-sys/senai-courses-track/internal/notifier"
	"github.com/Brook-sys/senai-courses-track/internal/scraper"
	"github.com/Brook-sys/senai-courses-track/internal/storage"
)

type Scheduler struct {
	db        *storage.DB
	scraper   *scraper.Scraper
	notifiers *notifier.NotifierManager
	wake      chan struct{}
}

func New(db *storage.DB, s *scraper.Scraper) *Scheduler {
	return &Scheduler{
		db:        db,
		scraper:   s,
		notifiers: notifier.NewManager(),
		wake:      make(chan struct{}, 1),
	}
}

func (sch *Scheduler) Start(spec string) {
	notifier.RegisterFromConfig(sch.notifiers, sch.db.GetConfig)
	go sch.loop()
	log.Printf("Scheduler started with interval: %s", sch.GetInterval().String())
}

func (sch *Scheduler) loop() {
	for {
		interval := sch.GetInterval()
		timer := time.NewTimer(interval)
		select {
		case <-timer.C:
			log.Println("Running scheduled update...")
			sch.RunUpdate()
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
	if _, err := strconv.Atoi(minutes); err != nil {
		return err
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

func (sch *Scheduler) RunUpdate() {
	subs, err := sch.db.GetActiveSubscriptions()
	if err != nil {
		log.Println("error getting subs:", err)
		return
	}

	for _, sub := range subs {
		var filters map[string]string
		json.Unmarshal([]byte(sub.Filters), &filters)

		filterKey := buildFilterKey(filters)
		courses, err := sch.scraper.FetchCourses(filters)
		if err != nil {
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
				// Notify
				if err := sch.notifiers.NotifyAll(c, sub.Name); err != nil {
					log.Printf("Failed to notify %s, skipping database save to retry later", c.ID)
					continue
				}

				sch.db.SaveCourse(sc)
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
	go sch.RunUpdate()
}
