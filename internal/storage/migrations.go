package storage

import (
	"context"
	"database/sql"
	"fmt"
)

func runMigrations(db *sql.DB) error {
	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() // Rollback if not committed

	// Ensure migrations table exists
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create schema_migrations table: %w", err)
	}

	var currentVersion int
	err = tx.QueryRowContext(ctx, "SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&currentVersion)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("failed to get current migration version: %w", err)
	}

	migrations := []struct {
		Version int
		Query   string
	}{
		{
			Version: 1,
			Query: `
				CREATE TABLE IF NOT EXISTS courses (
					id TEXT,
					title TEXT,
					url TEXT,
					filter_key TEXT,
					first_seen DATETIME DEFAULT CURRENT_TIMESTAMP,
					PRIMARY KEY (id, filter_key)
				);
				CREATE TABLE IF NOT EXISTS subscriptions (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					name TEXT,
					filters TEXT,
					active BOOLEAN DEFAULT 1,
					created_at DATETIME DEFAULT CURRENT_TIMESTAMP
				);
				CREATE TABLE IF NOT EXISTS notifier_config (
					key TEXT PRIMARY KEY,
					value TEXT
				);
			`,
		},
		{
			Version: 2,
			Query: `
				CREATE TABLE IF NOT EXISTS telegram_recipients (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					chat_id TEXT UNIQUE NOT NULL,
					label TEXT,
					enabled BOOLEAN DEFAULT 1,
					created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
					last_test_at DATETIME,
					last_test_ok BOOLEAN,
					last_test_error TEXT,
					last_delivery_at DATETIME,
					last_delivery_ok BOOLEAN,
					last_delivery_error TEXT
				);

				-- Migrate existing chat_id if exists
				INSERT OR IGNORE INTO telegram_recipients (chat_id, label)
				SELECT value, 'Default Chat'
				FROM notifier_config
				WHERE key = 'telegram_chat_id' AND value != '';
			`,
		},
	}

	for _, m := range migrations {
		if m.Version > currentVersion {
			if _, err := tx.ExecContext(ctx, m.Query); err != nil {
				return fmt.Errorf("migration %d failed: %w", m.Version, err)
			}
			if _, err := tx.ExecContext(ctx, "INSERT INTO schema_migrations (version) VALUES (?)", m.Version); err != nil {
				return fmt.Errorf("failed to record migration %d: %w", m.Version, err)
			}
		}
	}

	return tx.Commit()
}
