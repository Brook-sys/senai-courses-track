package notifier

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
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
	msg := fmt.Sprintf("🆕 Novo curso em %s:\n%v", subName, course)
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.Token)
	resp, err := http.PostForm(apiURL, url.Values{
		"chat_id": {t.ChatID},
		"text":    {msg},
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

func (m *NotifierManager) NotifyAll(course interface{}, subName string) {
	for _, n := range m.notifiers {
		if err := n.NotifyNewCourse(course, subName); err != nil {
			log.Printf("notifier %s error: %v", n.Name(), err)
		}
	}
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
