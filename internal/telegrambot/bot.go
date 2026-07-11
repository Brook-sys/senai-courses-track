package telegrambot

import (
	"encoding/json"
	"fmt"
	"html"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Brook-sys/senai-courses-track/internal/scraper"
	"github.com/Brook-sys/senai-courses-track/internal/storage"
)

type Bot struct {
	db         *storage.DB
	scraper    *scraper.Scraper
	token      string
	chatID     string
	lastUpdate int64
	httpClient *http.Client
}

type updateResponse struct {
	OK     bool     `json:"ok"`
	Result []update `json:"result"`
}

type update struct {
	UpdateID      int64          `json:"update_id"`
	Message       *message       `json:"message"`
	CallbackQuery *callbackQuery `json:"callback_query"`
}

type message struct {
	Chat chat   `json:"chat"`
	Text string `json:"text"`
}

type chat struct {
	ID int64 `json:"id"`
}

type callbackQuery struct {
	ID      string  `json:"id"`
	Data    string  `json:"data"`
	Message message `json:"message"`
}

func New(db *storage.DB, s *scraper.Scraper) *Bot {
	return &Bot{
		db:         db,
		scraper:    s,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

func (b *Bot) Start() {
	go b.loop()
}

func (b *Bot) loop() {
	for {
		b.reloadConfig()
		if b.token == "" || b.chatID == "" {
			time.Sleep(10 * time.Second)
			continue
		}

		updates, err := b.getUpdates()
		if err != nil {
			log.Printf("telegram bot updates error: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}

		for _, upd := range updates {
			if upd.UpdateID >= b.lastUpdate {
				b.lastUpdate = upd.UpdateID + 1
			}
			b.handleUpdate(upd)
		}
	}
}

func (b *Bot) reloadConfig() {
	token, _ := b.db.GetConfig("telegram_token")
	chatID, _ := b.db.GetConfig("telegram_chat_id")
	b.token = token
	b.chatID = chatID
}

func (b *Bot) getUpdates() ([]update, error) {
	u := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates?timeout=20&offset=%d", b.token, b.lastUpdate)
	resp, err := b.httpClient.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var parsed updateResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}
	return parsed.Result, nil
}

func (b *Bot) handleUpdate(upd update) {
	if upd.Message != nil {
		if !b.allowedChat(upd.Message.Chat.ID) {
			return
		}
		switch strings.TrimSpace(upd.Message.Text) {
		case "/start", "/menu":
			b.sendMenu(upd.Message.Chat.ID)
		case "/filtros":
			b.sendFilters(upd.Message.Chat.ID)
		default:
			b.sendText(upd.Message.Chat.ID, "Use /menu para abrir o menu do SENAI Track.")
		}
		return
	}

	if upd.CallbackQuery != nil {
		if !b.allowedChat(upd.CallbackQuery.Message.Chat.ID) {
			return
		}
		b.answerCallback(upd.CallbackQuery.ID)
		b.handleCallback(upd.CallbackQuery.Message.Chat.ID, upd.CallbackQuery.Data)
	}
}

func (b *Bot) allowedChat(chatID int64) bool {
	allowed, err := strconv.ParseInt(b.chatID, 10, 64)
	return err == nil && allowed == chatID
}

func (b *Bot) sendMenu(chatID int64) {
	keyboard := inlineKeyboard([][]button{
		{{Text: "📋 Ver filtros", Data: "filters"}},
	})
	b.sendHTML(chatID, "<b>SENAI Track</b>\nEscolha uma opção:", keyboard)
}

func (b *Bot) sendFilters(chatID int64) {
	subs, err := b.db.GetSubscriptions()
	if err != nil || len(subs) == 0 {
		b.sendText(chatID, "Nenhuma assinatura cadastrada.")
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

	b.sendHTML(chatID, "<b>Filtros cadastrados</b>\nSelecione um filtro para ver os cursos disponíveis:", inlineKeyboard(rows))
}

func (b *Bot) handleCallback(chatID int64, data string) {
	switch {
	case data == "menu":
		b.sendMenu(chatID)
	case data == "filters":
		b.sendFilters(chatID)
	case strings.HasPrefix(data, "sub:"):
		id := strings.TrimPrefix(data, "sub:")
		b.sendSubscriptionCourses(chatID, id)
	}
}

func (b *Bot) sendSubscriptionCourses(chatID int64, id string) {
	sub, ok := b.findSubscription(id)
	if !ok {
		b.sendText(chatID, "Filtro não encontrado.")
		return
	}

	filters := map[string]string{}
	json.Unmarshal([]byte(sub.Filters), &filters)

	b.sendText(chatID, fmt.Sprintf("Buscando cursos para: %s...", sub.Name))
	courses, err := b.scraper.FetchCourses(filters)
	if err != nil {
		b.sendText(chatID, "Erro ao buscar cursos no SENAI.")
		return
	}

	available := make([]scraper.Course, 0)
	for _, c := range courses {
		if c.HasTurmas && len(c.Turmas) > 0 {
			available = append(available, c)
		}
	}

	if len(available) == 0 {
		b.sendHTML(chatID, fmt.Sprintf("<b>%s</b>\nNenhum curso com turma disponível agora.", esc(sub.Name)), "")
		return
	}

	b.sendHTML(chatID, fmt.Sprintf("<b>%s</b>\n%d curso(s) com turma disponível:", esc(sub.Name), len(available)), "")

	for _, c := range available {
		b.sendCourse(chatID, c)
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

func (b *Bot) sendCourse(chatID int64, c scraper.Course) {
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
	b.sendHTML(chatID, sb.String(), "")
}

func (b *Bot) sendText(chatID int64, text string) {
	b.sendHTML(chatID, esc(text), "")
}

func (b *Bot) sendHTML(chatID int64, text string, replyMarkup string) {
	form := url.Values{}
	form.Set("chat_id", fmt.Sprint(chatID))
	form.Set("text", text)
	form.Set("parse_mode", "HTML")
	form.Set("disable_web_page_preview", "true")
	if replyMarkup != "" {
		form.Set("reply_markup", replyMarkup)
	}

	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", b.token)
	resp, err := b.httpClient.PostForm(apiURL, form)
	if err != nil {
		log.Printf("telegram send error: %v", err)
		return
	}
	defer resp.Body.Close()
}

func (b *Bot) answerCallback(id string) {
	if id == "" {
		return
	}
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/answerCallbackQuery", b.token)
	_, _ = b.httpClient.PostForm(apiURL, url.Values{"callback_query_id": {id}})
}

type button struct {
	Text string `json:"text"`
	Data string `json:"callback_data"`
}

func inlineKeyboard(rows [][]button) string {
	payload := map[string][][]button{"inline_keyboard": rows}
	b, _ := json.Marshal(payload)
	return string(b)
}

func esc(s string) string {
	return html.EscapeString(s)
}
