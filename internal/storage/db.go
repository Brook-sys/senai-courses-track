package storage

import (
	"database/sql"
	"time"

	_ "modernc.org/sqlite"
)

type DB struct {
	*sql.DB
}

type StoredCourse struct {
	ID        string
	Title     string
	URL       string
	FilterKey string
	FirstSeen time.Time
}

type Subscription struct {
	ID        int64
	Name      string
	Filters   string // JSON string of filters
	Active    bool
	CreatedAt time.Time
}

func New(path string) (*DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	if err := runMigrations(db); err != nil {
		db.Close()
		return nil, err
	}

	return &DB{db}, nil
}

func (db *DB) CourseExists(id, filterKey string) bool {
	var count int
	db.QueryRow("SELECT COUNT(*) FROM courses WHERE id = ? AND filter_key = ?", id, filterKey).Scan(&count)
	return count > 0
}

func (db *DB) SaveCourse(c StoredCourse) error {
	_, err := db.Exec(`
		INSERT OR IGNORE INTO courses (id, title, url, filter_key, first_seen)
		VALUES (?, ?, ?, ?, ?)`,
		c.ID, c.Title, c.URL, c.FilterKey, c.FirstSeen)
	return err
}

func (db *DB) GetAllCourses() ([]StoredCourse, error) {
	rows, err := db.Query("SELECT id, title, url, filter_key, first_seen FROM courses ORDER BY first_seen DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []StoredCourse
	for rows.Next() {
		var c StoredCourse
		rows.Scan(&c.ID, &c.Title, &c.URL, &c.FilterKey, &c.FirstSeen)
		list = append(list, c)
	}
	return list, nil
}

func (db *DB) GetAllCoursesForFilter(filterKey string) ([]StoredCourse, error) {
	rows, err := db.Query("SELECT id, title, url, filter_key, first_seen FROM courses WHERE filter_key = ? ORDER BY first_seen DESC", filterKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []StoredCourse
	for rows.Next() {
		var c StoredCourse
		rows.Scan(&c.ID, &c.Title, &c.URL, &c.FilterKey, &c.FirstSeen)
		list = append(list, c)
	}
	return list, nil
}

func (db *DB) CreateSubscription(name, filters string) (int64, error) {
	res, err := db.Exec("INSERT INTO subscriptions (name, filters) VALUES (?, ?)", name, filters)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (db *DB) ClearAllCourses() error {
	_, err := db.Exec("DELETE FROM courses")
	return err
}

func (db *DB) DeleteSubscription(id string) error {
	_, err := db.Exec("DELETE FROM subscriptions WHERE id = ?", id)
	return err
}

func (db *DB) SetSubscriptionActive(id string, active bool) error {
	_, err := db.Exec("UPDATE subscriptions SET active = ? WHERE id = ?", active, id)
	return err
}

func (db *DB) GetSubscriptions() ([]Subscription, error) {
	rows, err := db.Query("SELECT id, name, filters, active, created_at FROM subscriptions ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []Subscription
	for rows.Next() {
		var s Subscription
		rows.Scan(&s.ID, &s.Name, &s.Filters, &s.Active, &s.CreatedAt)
		subs = append(subs, s)
	}
	return subs, nil
}

func (db *DB) GetActiveSubscriptions() ([]Subscription, error) {
	rows, err := db.Query("SELECT id, name, filters, active, created_at FROM subscriptions WHERE active = 1 ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []Subscription
	for rows.Next() {
		var s Subscription
		rows.Scan(&s.ID, &s.Name, &s.Filters, &s.Active, &s.CreatedAt)
		subs = append(subs, s)
	}
	return subs, nil
}

func (db *DB) SetConfig(key, value string) error {
	_, err := db.Exec("INSERT OR REPLACE INTO notifier_config (key, value) VALUES (?, ?)", key, value)
	return err
}

func (db *DB) GetConfig(key string) (string, error) {
	var val string
	err := db.QueryRow("SELECT value FROM notifier_config WHERE key = ?", key).Scan(&val)
	return val, err
}
