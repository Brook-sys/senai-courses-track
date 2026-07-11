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
	Token  string
	ChatID string
}

func (t *TelegramNotifier) Name() string { return "telegram" }

func (t *TelegramNotifier) NotifyNewCourse(course interface{}, subName string) error {
	if t.Token == "" || t.ChatID == "" {
		return nil
	}
	c, ok := course.(scraper.Course)
	if !ok {
		// Fallback for raw interface
		msg := fmt.Sprintf("🆕 Novo curso em %s:\n%v", subName, course)
		return t.send(msg)
	}

	msg := formatCourseMessage(c, subName)
	return t.send(msg)
}

func (t *TelegramNotifier) send(msg string) error {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.Token)
	resp, err := http.PostForm(apiURL, url.Values{
		"chat_id":    {t.ChatID},
		"text":       {msg},
		"parse_mode": {"Markdown"},
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

func LoadTelegramFromDB(getConfig func(string) (string, error)) *TelegramNotifier {
	token, _ := getConfig("telegram_token")
	chat, _ := getConfig("telegram_chat_id")
	if token != "" && chat != "" {
		return &TelegramNotifier{Token: token, ChatID: chat}
	}
	return nil
}

func RegisterFromConfig(m *NotifierManager, getConfig func(string) (string, error)) {
	if tg := LoadTelegramFromDB(getConfig); tg != nil {
		m.Register(tg)
		log.Println("Telegram notifier registered")
	}
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
