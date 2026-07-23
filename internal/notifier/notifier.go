package notifier

import (
	"context"
	"fmt"
	"log"

	"github.com/Brook-sys/senai-courses-track/internal/scraper"
	"github.com/Brook-sys/senai-courses-track/internal/storage"
	"github.com/Brook-sys/senai-courses-track/internal/telegramclient"
)

type Notifier interface {
	NotifyNewCourse(ctx context.Context, course interface{}, subName string) error
	Name() string
}

type TelegramNotifier struct {
	Store  storage.TelegramConfigStore
	Client telegramclient.Client
}

func (t *TelegramNotifier) Name() string { return "telegram" }

func (t *TelegramNotifier) NotifyNewCourse(ctx context.Context, course interface{}, subName string) error {
	token, err := t.Store.GetTelegramToken(ctx)
	if err != nil || token == "" {
		return nil // Not configured
	}

	recipients, err := t.Store.GetEnabledTelegramRecipients(ctx)
	if err != nil || len(recipients) == 0 {
		return nil // No recipients
	}

	var msg string
	c, ok := course.(scraper.Course)
	if !ok {
		// Fallback for raw interface
		msg = fmt.Sprintf("🆕 Novo curso em %s:\n%v", subName, course)
	} else {
		msg = formatCourseMessage(c, subName)
	}

	var firstErr error
	successCount := 0
	for _, recipient := range recipients {
		err := t.Client.SendMessage(ctx, token, recipient.ChatID, msg, "HTML")
		if err != nil {
			errStr := err.Error()
			if statusErr := t.Store.UpdateRecipientDeliveryStatus(ctx, recipient.ID, false, &errStr); statusErr != nil {
				log.Printf("telegram delivery status error for recipient %d: %v", recipient.ID, statusErr)
			}
			log.Printf("telegram delivery error for recipient %d: %v", recipient.ID, err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}

		successCount++
		if statusErr := t.Store.UpdateRecipientDeliveryStatus(ctx, recipient.ID, true, nil); statusErr != nil {
			log.Printf("telegram delivery status error for recipient %d: %v", recipient.ID, statusErr)
		}
	}

	if successCount == 0 {
		return firstErr
	}
	return nil
}

type NotifierManager struct {
	notifiers []Notifier
}

func NewManager() *NotifierManager {
	return &NotifierManager{}
}

func (m *NotifierManager) Register(n Notifier) {
	m.notifiers = append(m.notifiers, n)
}

func (m *NotifierManager) NotifyAll(ctx context.Context, course interface{}, subName string) error {
	var firstErr error
	for _, n := range m.notifiers {
		if err := n.NotifyNewCourse(ctx, course, subName); err != nil {
			log.Printf("notifier %s error: %v", n.Name(), err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

func RegisterFromConfig(m *NotifierManager, store storage.TelegramConfigStore, client telegramclient.Client) {
	m.Register(&TelegramNotifier{Store: store, Client: client})
	log.Println("Telegram notifier registered (multi-recipient)")
}
