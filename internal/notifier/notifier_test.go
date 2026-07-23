package notifier

import (
	"context"
	"errors"
	"testing"

	"github.com/Brook-sys/senai-courses-track/internal/storage"
	"github.com/Brook-sys/senai-courses-track/internal/telegramclient"
)

type notifierTestStore struct {
	recipients []storage.TelegramRecipient
	statuses   map[int64]bool
}

func (s *notifierTestStore) GetTelegramToken(context.Context) (string, error) {
	return "token", nil
}

func (s *notifierTestStore) SetTelegramToken(context.Context, string) error {
	return nil
}

func (s *notifierTestStore) GetEnabledTelegramRecipients(context.Context) ([]storage.TelegramRecipient, error) {
	return s.recipients, nil
}

func (s *notifierTestStore) UpdateRecipientDeliveryStatus(_ context.Context, id int64, ok bool, _ *string) error {
	s.statuses[id] = ok
	return nil
}

type notifierTestClient struct {
	fail map[string]bool
}

func (c *notifierTestClient) SendMessage(_ context.Context, _, chatID, _, _ string) error {
	if c.fail[chatID] {
		return errors.New("send failed")
	}
	return nil
}

func (c *notifierTestClient) SendMessageWithMarkup(context.Context, string, string, string, string, string) error {
	return nil
}

func (c *notifierTestClient) GetUpdates(context.Context, string, int64, int) ([]telegramclient.Update, error) {
	return nil, nil
}

func (c *notifierTestClient) AnswerCallbackQuery(context.Context, string, string) error {
	return nil
}

func TestTelegramNotifierReturnsErrorWhenAllRecipientsFail(t *testing.T) {
	store := &notifierTestStore{
		recipients: []storage.TelegramRecipient{{ID: 1, ChatID: "1"}, {ID: 2, ChatID: "2"}},
		statuses:   map[int64]bool{},
	}
	n := &TelegramNotifier{Store: store, Client: &notifierTestClient{fail: map[string]bool{"1": true, "2": true}}}

	if err := n.NotifyNewCourse(t.Context(), "course", "subscription"); err == nil {
		t.Fatal("expected error when every recipient fails")
	}
	if store.statuses[1] || store.statuses[2] {
		t.Fatal("failed recipients were marked successful")
	}
}

func TestTelegramNotifierRecordsPartialDelivery(t *testing.T) {
	store := &notifierTestStore{
		recipients: []storage.TelegramRecipient{{ID: 1, ChatID: "1"}, {ID: 2, ChatID: "2"}},
		statuses:   map[int64]bool{},
	}
	n := &TelegramNotifier{Store: store, Client: &notifierTestClient{fail: map[string]bool{"2": true}}}

	if err := n.NotifyNewCourse(t.Context(), "course", "subscription"); err != nil {
		t.Fatalf("unexpected partial delivery error: %v", err)
	}
	if !store.statuses[1] {
		t.Fatal("successful recipient was not marked successful")
	}
	if store.statuses[2] {
		t.Fatal("failed recipient was marked successful")
	}
}
