package notifier

import (
	"fmt"
	"log"
	"net/http"
	"net/url"

	"github.com/Brook-sys/senai-courses-track/internal/scraper"
)

type Notifier interface {
	NotifyNewCourse(course interface{}, subName string) error
	Name() string
}

type TelegramNotifier struct {
	GetConfig func(string) (string, error)
}

func (t *TelegramNotifier) Name() string { return "telegram" }

func (t *TelegramNotifier) NotifyNewCourse(course interface{}, subName string) error {
	token, _ := t.GetConfig("telegram_token")
	chatID, _ := t.GetConfig("telegram_chat_id")

	if token == "" || chatID == "" {
		return nil
	}

	c, ok := course.(scraper.Course)
	if !ok {
		// Fallback for raw interface
		msg := fmt.Sprintf("🆕 Novo curso em %s:\n%v", subName, course)
		return t.send(token, chatID, msg)
	}

	msg := formatCourseMessage(c, subName)
	return t.send(token, chatID, msg)
}

func (t *TelegramNotifier) send(token, chatID, msg string) error {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token)
	resp, err := http.PostForm(apiURL, url.Values{
		"chat_id":    {chatID},
		"text":       {msg},
		"parse_mode": {"HTML"},
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("telegram error: %d", resp.StatusCode)
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

func (m *NotifierManager) NotifyAll(course interface{}, subName string) error {
	var firstErr error
	for _, n := range m.notifiers {
		if err := n.NotifyNewCourse(course, subName); err != nil {
			log.Printf("notifier %s error: %v", n.Name(), err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

func RegisterFromConfig(m *NotifierManager, getConfig func(string) (string, error)) {
	m.Register(&TelegramNotifier{GetConfig: getConfig})
	log.Println("Telegram notifier registered (dynamic)")
}

func SendTestMessage(token, chatID, msg string) error {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token)
	resp, err := http.PostForm(apiURL, url.Values{
		"chat_id": {chatID},
		"text":    {msg},
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}
