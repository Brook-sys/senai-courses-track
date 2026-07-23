package storage

import (
	"testing"
)

func TestTelegramRecipientMigration(t *testing.T) {
	path := t.TempDir() + "/courses.db"
	db, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SetConfig("telegram_chat_id", "987654321"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("DELETE FROM schema_migrations WHERE version = 2"); err != nil {
		t.Fatal(err)
	}
	db.Close()

	db, err = New(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	recipients, err := db.GetTelegramRecipients(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(recipients) != 1 || recipients[0].ChatID != "987654321" {
		t.Fatalf("recipients = %#v, want migrated chat ID", recipients)
	}
}
