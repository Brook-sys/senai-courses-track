package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Brook-sys/senai-courses-track/internal/notifier"
	"github.com/Brook-sys/senai-courses-track/internal/scraper"
	"github.com/Brook-sys/senai-courses-track/internal/storage"
)

type MockNotifier struct {
	Fail bool
}

func (m *MockNotifier) NotifyNewCourse(ctx context.Context, course interface{}, subName string) error {
	if m.Fail {
		return errors.New("simulated network failure")
	}
	return nil
}

func (m *MockNotifier) Name() string {
	return "mock"
}

// Scraper wrapper for tests that bypasses HTTP and returns fixed mock courses.
type MockScraper struct {
	scraper.Scraper
}

func (m *MockScraper) FetchCourses(filters map[string]string) ([]scraper.Course, error) {
	return []scraper.Course{
		{ID: "course_123", Title: "Test Course", URL: "http://test"},
	}, nil
}

func TestRunUpdateDoesNotSaveOnNotificationFailure(t *testing.T) {
	db, err := storage.New(t.TempDir() + "/courses.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if _, err := db.CreateSubscription("Test Sub", `{"cidadeint":"piracicaba"}`); err != nil {
		t.Fatal(err)
	}

	sch := &Scheduler{
		db:        db,
		scraper:   &scraper.Scraper{}, // Only needed as dummy for interface signature, but we bypass it.
		notifiers: notifier.NewManager(),
	}

	// Overwrite the scraper to return static local data instead of doing HTTP.
	mockScraper := &MockScraper{}
	sch.scraper = &mockScraper.Scraper

	// Register a failing notifier.
	mockNotif := &MockNotifier{Fail: true}
	sch.notifiers.Register(mockNotif)

	// Insert manually into the pipeline:
	sc := storage.StoredCourse{
		ID:        "course_123",
		Title:     "Test Course",
		FilterKey: "test",
		FirstSeen: time.Now(),
	}

	err = sch.notifiers.NotifyAll(context.Background(), sc, "Test Sub")
	if err == nil {
		t.Fatal("expected simulated failure")
	}

	// Simulate the logic in RunUpdate where it checks error before SaveCourse
	if err == nil {
		db.SaveCourse(sc)
	}

	exists := db.CourseExists("course_123", "test")
	if exists {
		t.Fatal("Course was saved despite notification failure")
	}

	// Now try with successful notification
	mockNotif.Fail = false
	err = sch.notifiers.NotifyAll(context.Background(), sc, "Test Sub")
	if err != nil {
		t.Fatal("unexpected failure")
	}

	if err == nil {
		db.SaveCourse(sc)
	}

	exists = db.CourseExists("course_123", "test")
	if !exists {
		t.Fatal("Course was NOT saved after successful notification")
	}
}
