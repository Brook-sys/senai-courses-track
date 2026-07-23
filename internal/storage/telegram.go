package storage

import (
	"context"
	"database/sql"
	"time"
)

type TelegramRecipient struct {
	ID                int64
	ChatID            string
	Label             string
	Enabled           bool
	CreatedAt         time.Time
	LastTestAt        *time.Time
	LastTestOk        *bool
	LastTestError     *string
	LastDeliveryAt    *time.Time
	LastDeliveryOk    *bool
	LastDeliveryError *string
}

func (db *DB) AddTelegramRecipient(ctx context.Context, chatID, label string) (int64, error) {
	res, err := db.ExecContext(ctx, "INSERT INTO telegram_recipients (chat_id, label) VALUES (?, ?)", chatID, label)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (db *DB) RemoveTelegramRecipient(ctx context.Context, id int64) error {
	_, err := db.ExecContext(ctx, "DELETE FROM telegram_recipients WHERE id = ?", id)
	return err
}

func (db *DB) SetTelegramRecipientEnabled(ctx context.Context, id int64, enabled bool) error {
	_, err := db.ExecContext(ctx, "UPDATE telegram_recipients SET enabled = ? WHERE id = ?", enabled, id)
	return err
}

func (db *DB) GetTelegramRecipients(ctx context.Context) ([]TelegramRecipient, error) {
	rows, err := db.QueryContext(ctx, "SELECT id, chat_id, label, enabled, created_at, last_test_at, last_test_ok, last_test_error, last_delivery_at, last_delivery_ok, last_delivery_error FROM telegram_recipients ORDER BY id ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []TelegramRecipient
	for rows.Next() {
		var r TelegramRecipient
		err := rows.Scan(&r.ID, &r.ChatID, &r.Label, &r.Enabled, &r.CreatedAt, &r.LastTestAt, &r.LastTestOk, &r.LastTestError, &r.LastDeliveryAt, &r.LastDeliveryOk, &r.LastDeliveryError)
		if err != nil {
			return nil, err
		}
		list = append(list, r)
	}
	return list, nil
}

func (db *DB) GetEnabledTelegramRecipients(ctx context.Context) ([]TelegramRecipient, error) {
	rows, err := db.QueryContext(ctx, "SELECT id, chat_id, label, enabled, created_at, last_test_at, last_test_ok, last_test_error, last_delivery_at, last_delivery_ok, last_delivery_error FROM telegram_recipients WHERE enabled = 1 ORDER BY id ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []TelegramRecipient
	for rows.Next() {
		var r TelegramRecipient
		err := rows.Scan(&r.ID, &r.ChatID, &r.Label, &r.Enabled, &r.CreatedAt, &r.LastTestAt, &r.LastTestOk, &r.LastTestError, &r.LastDeliveryAt, &r.LastDeliveryOk, &r.LastDeliveryError)
		if err != nil {
			return nil, err
		}
		list = append(list, r)
	}
	return list, nil
}

func (db *DB) UpdateRecipientTestStatus(ctx context.Context, id int64, ok bool, errStr *string) error {
	_, err := db.ExecContext(ctx, "UPDATE telegram_recipients SET last_test_at = CURRENT_TIMESTAMP, last_test_ok = ?, last_test_error = ? WHERE id = ?", ok, errStr, id)
	return err
}

func (db *DB) UpdateRecipientDeliveryStatus(ctx context.Context, id int64, ok bool, errStr *string) error {
	_, err := db.ExecContext(ctx, "UPDATE telegram_recipients SET last_delivery_at = CURRENT_TIMESTAMP, last_delivery_ok = ?, last_delivery_error = ? WHERE id = ?", ok, errStr, id)
	return err
}

// Ensure interface compatibility for Telegram client
type TelegramConfigStore interface {
	GetTelegramToken(ctx context.Context) (string, error)
	SetTelegramToken(ctx context.Context, token string) error
	GetEnabledTelegramRecipients(ctx context.Context) ([]TelegramRecipient, error)
	UpdateRecipientDeliveryStatus(ctx context.Context, id int64, ok bool, errStr *string) error
}

func (db *DB) GetTelegramToken(ctx context.Context) (string, error) {
	var val string
	err := db.QueryRowContext(ctx, "SELECT value FROM notifier_config WHERE key = 'telegram_token'").Scan(&val)
	if err == sql.ErrNoRows {
		return "", nil // Not configured
	}
	return val, err
}

func (db *DB) SetTelegramToken(ctx context.Context, token string) error {
	_, err := db.ExecContext(ctx, "INSERT OR REPLACE INTO notifier_config (key, value) VALUES ('telegram_token', ?)", token)
	return err
}

func (db *DB) ClearTelegramToken(ctx context.Context) error {
	_, err := db.ExecContext(ctx, "DELETE FROM notifier_config WHERE key = 'telegram_token'")
	return err
}
