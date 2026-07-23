package telegrambot

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Brook-sys/senai-courses-track/internal/scraper"
	"github.com/Brook-sys/senai-courses-track/internal/storage"
	"github.com/Brook-sys/senai-courses-track/internal/telegramclient"
)

type Bot struct {
	db      *storage.DB
	scraper *scraper.Scraper
	client  telegramclient.Client

	mu           sync.Mutex
	token        string
	allowedChats map[string]bool

	lastUpdateID  int64
	lastToken     string
	polling       bool
	lastPollTime  time.Time
	lastPollError error
}

func New(db *storage.DB, s *scraper.Scraper, client telegramclient.Client) *Bot {
	return &Bot{
		db:           db,
		scraper:      s,
		client:       client,
		allowedChats: make(map[string]bool),
	}
}

func (b *Bot) Start(ctx context.Context) {
	go b.loop(ctx)
}

func (b *Bot) loop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		b.reloadConfig(ctx)

		b.mu.Lock()
		token := b.token
		b.mu.Unlock()

		if token == "" {
			b.mu.Lock()
			b.polling = false
			b.mu.Unlock()
			if !wait(ctx, 10*time.Second) {
				return
			}
			continue
		}

		b.mu.Lock()
		b.polling = true
		b.lastPollTime = time.Now()
		offset := b.lastUpdateID
		b.mu.Unlock()

		updates, err := b.client.GetUpdates(ctx, token, offset, 20)

		b.mu.Lock()
		b.polling = false
		b.lastPollError = err
		if err == nil {
			b.lastPollTime = time.Now()
		}
		b.mu.Unlock()

		if err != nil {
			log.Printf("telegram bot updates error: %v", err)
			if !wait(ctx, 5*time.Second) {
				return
			}
			continue
		}

		for _, upd := range updates {
			b.mu.Lock()
			if upd.UpdateID >= b.lastUpdateID {
				b.lastUpdateID = upd.UpdateID + 1
			}
			b.mu.Unlock()
			b.handleUpdate(ctx, upd, token)
		}
	}
}

func (b *Bot) reloadConfig(ctx context.Context) {
	token, _ := b.db.GetTelegramToken(ctx)

	recipients, err := b.db.GetEnabledTelegramRecipients(ctx)
	allowedChats := make(map[string]bool)
	if err == nil {
		for _, r := range recipients {
			allowedChats[r.ChatID] = true
		}
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	b.token = token
	b.allowedChats = allowedChats

	if b.lastToken != "" && token != b.lastToken {
		b.lastUpdateID = 0
	}
	b.lastToken = token
}

type BotStatus struct {
	Configured    bool      `json:"configured"`
	Polling       bool      `json:"polling"`
	LastPollTime  time.Time `json:"lastPollTime"`
	LastPollError string    `json:"lastPollError,omitempty"`
}

func (b *Bot) Status() BotStatus {
	b.mu.Lock()
	defer b.mu.Unlock()

	errStr := ""
	if b.lastPollError != nil {
		errStr = b.lastPollError.Error()
	}

	return BotStatus{
		Configured:    b.token != "",
		Polling:       b.polling,
		LastPollTime:  b.lastPollTime,
		LastPollError: errStr,
	}
}

func (b *Bot) handleUpdate(ctx context.Context, upd telegramclient.Update, token string) {
	if upd.Message != nil {
		if !b.allowedChat(upd.Message.Chat.ID) {
			return
		}
		switch strings.TrimSpace(upd.Message.Text) {
		case "/start", "/menu":
			b.sendMenu(ctx, upd.Message.Chat.ID, token)
		case "/filtros":
			b.sendFilters(ctx, upd.Message.Chat.ID, token)
		default:
			b.sendText(ctx, upd.Message.Chat.ID, "Use /menu para abrir o menu do SENAI Track.", token)
		}
		return
	}

	if upd.CallbackQuery != nil && upd.CallbackQuery.Message != nil {
		if !b.allowedChat(upd.CallbackQuery.Message.Chat.ID) {
			return
		}
		b.client.AnswerCallbackQuery(ctx, token, upd.CallbackQuery.ID)
		b.handleCallback(ctx, upd.CallbackQuery.Message.Chat.ID, upd.CallbackQuery.Data, token)
	}
}

func (b *Bot) allowedChat(chatID int64) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	idStr := strconv.FormatInt(chatID, 10)
	return b.allowedChats[idStr]
}

func (b *Bot) sendMenu(ctx context.Context, chatID int64, token string) {
	keyboard := inlineKeyboard([][]button{
		{{Text: "📋 Ver filtros", Data: "filters"}},
	})
	b.client.SendMessageWithMarkup(ctx, token, strconv.FormatInt(chatID, 10), "<b>SENAI Track</b>\nEscolha uma opção:", "HTML", keyboard)
}

func (b *Bot) sendFilters(ctx context.Context, chatID int64, token string) {
	subs, err := b.db.GetSubscriptions()
	if err != nil || len(subs) == 0 {
		b.sendText(ctx, chatID, "Nenhuma assinatura cadastrada.", token)
		return
	}

	var rows [][]button
	for _, sub := range subs {
		status := "🟢"
		if !sub.Active {
			status = "⚪"
		}
		rows = append(rows, []button{{
			Text: fmt.Sprintf("%s %s", status, sub.Name),
			Data: fmt.Sprintf("sub:%d", sub.ID),
		}})
	}
	rows = append(rows, []button{{Text: "⬅️ Menu", Data: "menu"}})

	b.client.SendMessageWithMarkup(ctx, token, strconv.FormatInt(chatID, 10), "<b>Filtros cadastrados</b>\nSelecione um filtro para ver os cursos disponíveis:", "HTML", inlineKeyboard(rows))
}

func (b *Bot) handleCallback(ctx context.Context, chatID int64, data string, token string) {
	switch {
	case data == "menu":
		b.sendMenu(ctx, chatID, token)
	case data == "filters":
		b.sendFilters(ctx, chatID, token)
	case strings.HasPrefix(data, "sub:"):
		id := strings.TrimPrefix(data, "sub:")
		b.sendSubscriptionCourses(ctx, chatID, id, token)
	}
}

func (b *Bot) sendSubscriptionCourses(ctx context.Context, chatID int64, id string, token string) {
	sub, ok := b.findSubscription(id)
	if !ok {
		b.sendText(ctx, chatID, "Filtro não encontrado.", token)
		return
	}

	filters := map[string]string{}
	json.Unmarshal([]byte(sub.Filters), &filters)

	b.sendText(ctx, chatID, fmt.Sprintf("Buscando cursos para: %s...", sub.Name), token)
	courses, err := b.scraper.FetchCourses(filters)
	if err != nil {
		b.sendText(ctx, chatID, "Erro ao buscar cursos no SENAI.", token)
		return
	}

	available := make([]scraper.Course, 0)
	for _, c := range courses {
		if c.HasTurmas && len(c.Turmas) > 0 {
			available = append(available, c)
		}
	}

	if len(available) == 0 {
		b.client.SendMessage(ctx, token, strconv.FormatInt(chatID, 10), fmt.Sprintf("<b>%s</b>\nNenhum curso com turma disponível agora.", esc(sub.Name)), "HTML")
		return
	}

	b.client.SendMessage(ctx, token, strconv.FormatInt(chatID, 10), fmt.Sprintf("<b>%s</b>\n%d curso(s) com turma disponível:", esc(sub.Name), len(available)), "HTML")

	for _, c := range available {
		b.sendCourse(ctx, chatID, c, token)
	}
}

func (b *Bot) findSubscription(id string) (storage.Subscription, bool) {
	subs, err := b.db.GetSubscriptions()
	if err != nil {
		return storage.Subscription{}, false
	}
	for _, sub := range subs {
		if fmt.Sprint(sub.ID) == id {
			return sub, true
		}
	}
	return storage.Subscription{}, false
}

func (b *Bot) sendCourse(ctx context.Context, chatID int64, c scraper.Course, token string) {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📚 <b>%s</b>\n", esc(c.Title)))
	if c.Duration != "" {
		sb.WriteString(fmt.Sprintf("⏱ <b>Carga horária:</b> %s\n", esc(c.Duration)))
	}
	if c.URL != "" {
		sb.WriteString(fmt.Sprintf("🔗 <a href=\"%s\">Abrir curso</a>\n", esc(c.URL)))
	}
	for i, t := range c.Turmas {
		if i >= 4 {
			sb.WriteString("\n...mais turmas disponíveis no dashboard.\n")
			break
		}
		sb.WriteString("\n")
		sb.WriteString(fmt.Sprintf("🏫 <b>Unidade:</b> %s\n", esc(t.Unit)))
		if t.Address != "" {
			sb.WriteString(fmt.Sprintf("📍 <b>Endereço:</b> %s\n", esc(t.Address)))
		}
		if t.Phone != "" {
			sb.WriteString(fmt.Sprintf("☎️ <b>Telefone:</b> %s\n", esc(t.Phone)))
		}
		if t.Vacancies != "" {
			sb.WriteString(fmt.Sprintf("🎟 <b>Vagas:</b> %s\n", esc(t.Vacancies)))
		}
		if t.StartDate != "" || t.EndDate != "" {
			sb.WriteString(fmt.Sprintf("📅 <b>Início/Fim:</b> %s até %s\n", esc(t.StartDate), esc(t.EndDate)))
		}
		if t.Period != "" {
			sb.WriteString(fmt.Sprintf("🗓 <b>Dias:</b> %s\n", esc(t.Period)))
		}
		if t.Schedule != "" {
			sb.WriteString(fmt.Sprintf("🕒 <b>Horário:</b> %s\n", esc(t.Schedule)))
		}
	}
	b.client.SendMessage(ctx, token, strconv.FormatInt(chatID, 10), sb.String(), "HTML")
}

func (b *Bot) sendText(ctx context.Context, chatID int64, text string, token string) {
	b.client.SendMessage(ctx, token, strconv.FormatInt(chatID, 10), esc(text), "HTML")
}

type button struct {
	Text string `json:"text"`
	Data string `json:"callback_data"`
}

func inlineKeyboard(rows [][]button) string {
	payload := map[string][][]button{"inline_keyboard": rows}
	bt, _ := json.Marshal(payload)
	return string(bt)
}

func esc(s string) string {
	return html.EscapeString(s)
}

func wait(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
