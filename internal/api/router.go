package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/Brook-sys/senai-courses-track/internal/api/views"
	"github.com/Brook-sys/senai-courses-track/internal/scheduler"
	"github.com/Brook-sys/senai-courses-track/internal/scraper"
	"github.com/Brook-sys/senai-courses-track/internal/storage"
	"github.com/Brook-sys/senai-courses-track/internal/telegrambot"
	"github.com/Brook-sys/senai-courses-track/internal/telegramclient"
	"github.com/gorilla/mux"
)

type botStatusProvider interface {
	Status() telegrambot.BotStatus
}

func NewRouter(db *storage.DB, s *scraper.Scraper, sch *scheduler.Scheduler, providers ...botStatusProvider) *mux.Router {
	r := mux.NewRouter()
	var botStatus botStatusProvider
	if len(providers) > 0 {
		botStatus = providers[0]
	}
	r.HandleFunc("/healthz", healthHandler).Methods(http.MethodGet)
	r.HandleFunc("/readyz", func(w http.ResponseWriter, req *http.Request) {
		if err := db.Ping(); err != nil {
			http.Error(w, "Database unavailable: "+err.Error(), http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ready")
	}).Methods(http.MethodGet)
	r.HandleFunc("/api/status", func(w http.ResponseWriter, req *http.Request) {
		status := map[string]interface{}{"scheduler": sch.Status()}
		if botStatus != nil {
			status["telegram"] = botStatus.Status()
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(status)
	}).Methods(http.MethodGet)

	// API
	r.HandleFunc("/api/courses", func(w http.ResponseWriter, r *http.Request) {
		filters := parseFilters(r)
		courses, _ := s.FetchCourses(filters)
		json.NewEncoder(w).Encode(courses)
	}).Methods("GET")

	r.HandleFunc("/api/cities", func(w http.ResponseWriter, r *http.Request) {
		cities, _ := s.FetchCities()
		json.NewEncoder(w).Encode(cities)
	}).Methods("GET")

	r.HandleFunc("/api/subscriptions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			var req struct {
				Name    string            `json:"name"`
				Filters map[string]string `json:"filters"`
			}
			json.NewDecoder(r.Body).Decode(&req)
			id, _ := sch.AddSubscription(req.Name, req.Filters)
			json.NewEncoder(w).Encode(map[string]int64{"id": id})
			return
		}
		subs, _ := db.GetSubscriptions()
		json.NewEncoder(w).Encode(subs)
	}).Methods("GET", "POST")

	r.HandleFunc("/api/subscriptions/htmx", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		name := r.FormValue("name")
		filters := map[string]string{}
		if city := r.FormValue("cidadeint"); city != "" {
			filters["cidadeint"] = city
		}
		if r.FormValue("gratuito") == "1" {
			filters["gratuito"] = "1"
		}
		if r.FormValue("modalidade") == "1" {
			filters["modalidade"] = "1"
		}
		sch.AddSubscription(name, filters)
		subs, _ := db.GetSubscriptions()
		views.SubscriptionsList(subs).Render(r.Context(), w)
	}).Methods("POST")

	r.HandleFunc("/api/trigger", func(w http.ResponseWriter, r *http.Request) {
		sch.TriggerNow()
		// Returning a nice HTMX response instead of a raw text page
		fmt.Fprint(w, `<div class="bg-green-100 border border-green-400 text-green-700 px-4 py-3 rounded relative" role="alert"><strong class="font-bold">Sincronização Iniciada!</strong><span class="block sm:inline"> Os cursos estão sendo buscados em segundo plano.</span></div>`)
	}).Methods("POST")

	r.HandleFunc("/api/subscriptions/{id}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		db.DeleteSubscription(vars["id"])
		subs, _ := db.GetSubscriptions()
		views.SubscriptionsList(subs).Render(r.Context(), w)
	}).Methods("DELETE")

	r.HandleFunc("/api/subscriptions/{id}/activate", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		db.SetSubscriptionActive(vars["id"], true)
		subs, _ := db.GetSubscriptions()
		views.SubscriptionsList(subs).Render(r.Context(), w)
	}).Methods("POST")

	r.HandleFunc("/api/subscriptions/{id}/deactivate", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		db.SetSubscriptionActive(vars["id"], false)
		subs, _ := db.GetSubscriptions()
		views.SubscriptionsList(subs).Render(r.Context(), w)
	}).Methods("POST")

	r.HandleFunc("/api/courses/clear", func(w http.ResponseWriter, r *http.Request) {
		db.ClearAllCourses()
		courses, _ := db.GetAllCourses()
		views.CoursesList(courses).Render(r.Context(), w)
	}).Methods("DELETE")

	r.HandleFunc("/api/config", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			http.Error(w, "use the Telegram settings endpoints", http.StatusGone)
			return
		}
		token, err := db.GetTelegramToken(r.Context())
		if err != nil {
			http.Error(w, "failed to load configuration", http.StatusInternalServerError)
			return
		}
		recipients, err := db.GetTelegramRecipients(r.Context())
		if err != nil {
			http.Error(w, "failed to load recipients", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"telegram_token_configured": token != "",
			"telegram_recipient_count":  len(recipients),
		})
	}).Methods(http.MethodGet, http.MethodPost)

	r.HandleFunc("/api/telegram/panel", func(w http.ResponseWriter, req *http.Request) {
		renderTelegramPanel(db, sch, w, req)
	}).Methods(http.MethodGet)

	r.HandleFunc("/api/telegram/token", func(w http.ResponseWriter, req *http.Request) {
		if err := req.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}
		token := strings.TrimSpace(req.FormValue("telegram_token"))
		if token == "" {
			http.Error(w, "token is required", http.StatusBadRequest)
			return
		}
		if err := db.SetTelegramToken(req.Context(), token); err != nil {
			http.Error(w, "failed to save token", http.StatusInternalServerError)
			return
		}
		renderTelegramPanel(db, sch, w, req)
	}).Methods(http.MethodPost)

	r.HandleFunc("/api/telegram/interval", func(w http.ResponseWriter, req *http.Request) {
		if err := req.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}
		if err := sch.SetIntervalMinutes(req.FormValue("sync_interval_minutes")); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		renderTelegramPanel(db, sch, w, req)
	}).Methods(http.MethodPost)

	r.HandleFunc("/api/telegram/recipients", func(w http.ResponseWriter, req *http.Request) {
		if err := req.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}
		chatID := strings.TrimSpace(req.FormValue("chat_id"))
		label := strings.TrimSpace(req.FormValue("label"))
		if _, err := strconv.ParseInt(chatID, 10, 64); err != nil {
			http.Error(w, "invalid Chat ID", http.StatusBadRequest)
			return
		}
		if label == "" {
			http.Error(w, "label is required", http.StatusBadRequest)
			return
		}
		if _, err := db.AddTelegramRecipient(req.Context(), chatID, label); err != nil {
			http.Error(w, "failed to add recipient", http.StatusConflict)
			return
		}
		renderTelegramPanel(db, sch, w, req)
	}).Methods(http.MethodPost)

	r.HandleFunc("/api/telegram/recipients/{id}", func(w http.ResponseWriter, req *http.Request) {
		id, err := strconv.ParseInt(mux.Vars(req)["id"], 10, 64)
		if err != nil || id < 1 {
			http.Error(w, "invalid recipient ID", http.StatusBadRequest)
			return
		}
		if err := db.RemoveTelegramRecipient(req.Context(), id); err != nil {
			http.Error(w, "failed to remove recipient", http.StatusInternalServerError)
			return
		}
		renderTelegramPanel(db, sch, w, req)
	}).Methods(http.MethodDelete)

	r.HandleFunc("/api/telegram/recipients/{id}/toggle", func(w http.ResponseWriter, req *http.Request) {
		id, err := strconv.ParseInt(mux.Vars(req)["id"], 10, 64)
		if err != nil || id < 1 {
			http.Error(w, "invalid recipient ID", http.StatusBadRequest)
			return
		}
		enabled, err := strconv.ParseBool(req.URL.Query().Get("enabled"))
		if err != nil {
			http.Error(w, "invalid enabled value", http.StatusBadRequest)
			return
		}
		if err := db.SetTelegramRecipientEnabled(req.Context(), id, enabled); err != nil {
			http.Error(w, "failed to update recipient", http.StatusInternalServerError)
			return
		}
		renderTelegramPanel(db, sch, w, req)
	}).Methods(http.MethodPost)

	r.HandleFunc("/api/telegram/recipients/{id}/test", func(w http.ResponseWriter, req *http.Request) {
		token, err := db.GetTelegramToken(req.Context())
		if err != nil || token == "" {
			http.Error(w, "Token não configurado", http.StatusBadRequest)
			return
		}
		id, err := strconv.ParseInt(mux.Vars(req)["id"], 10, 64)
		if err != nil || id < 1 {
			http.Error(w, "invalid recipient ID", http.StatusBadRequest)
			return
		}
		recs, err := db.GetTelegramRecipients(req.Context())
		if err != nil {
			http.Error(w, "failed to load recipients", http.StatusInternalServerError)
			return
		}
		var target string
		for _, r := range recs {
			if r.ID == id {
				target = r.ChatID
				break
			}
		}
		if target == "" {
			http.Error(w, "Chat ID não encontrado", http.StatusNotFound)
			return
		}

		client := telegramclient.NewClient()
		err = client.SendMessage(req.Context(), token, target, "🤖 Teste do SENAI Tracker! Configuração OK.", "")
		if err != nil {
			errStr := err.Error()
			db.UpdateRecipientTestStatus(req.Context(), id, false, &errStr)
			views.AlertError("Falha: "+err.Error()).Render(req.Context(), w)
		} else {
			db.UpdateRecipientTestStatus(req.Context(), id, true, nil)
			views.AlertSuccess("Mensagem enviada com sucesso!").Render(req.Context(), w)
		}
	}).Methods(http.MethodPost)

	// Dashboard
	r.HandleFunc("/", dashboardHandler(db, s)).Methods("GET")
	r.HandleFunc("/courses/list", func(w http.ResponseWriter, r *http.Request) {
		courses, _ := db.GetAllCourses()
		views.CoursesList(courses).Render(r.Context(), w)
	}).Methods("GET")
	r.HandleFunc("/courses/live", func(w http.ResponseWriter, r *http.Request) {
		courses, _ := s.FetchCourses(map[string]string{"cidadeint": "piracicaba", "gratuito": "1", "modalidade": "1"})
		views.LiveCoursesList(courses).Render(r.Context(), w)
	}).Methods("GET")

	return r
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "ok")
}

func parseFilters(r *http.Request) map[string]string {
	f := make(map[string]string)
	q := r.URL.Query()
	if city := q.Get("cidadeint"); city != "" {
		f["cidadeint"] = city
	}
	if q.Get("gratuito") == "1" {
		f["gratuito"] = "1"
	}
	if q.Get("modalidade") == "1" {
		f["modalidade"] = "1"
	}
	return f
}

func renderTelegramPanel(db *storage.DB, sch *scheduler.Scheduler, w http.ResponseWriter, r *http.Request) {
	token, err := db.GetTelegramToken(r.Context())
	if err != nil {
		http.Error(w, "failed to load token configuration", http.StatusInternalServerError)
		return
	}

	interval := "1440"
	if val, err := db.GetConfig("sync_interval_minutes"); err == nil && val != "" {
		interval = val
	}

	recipients, err := db.GetTelegramRecipients(r.Context())
	if err != nil {
		http.Error(w, "failed to load recipients", http.StatusInternalServerError)
		return
	}
	if err := views.TelegramPanel(maskToken(token), recipients, interval, sch.Status()).Render(r.Context(), w); err != nil {
		http.Error(w, "failed to render Telegram panel", http.StatusInternalServerError)
	}
}

func maskToken(token string) string {
	if token == "" {
		return ""
	}
	if len(token) <= 8 {
		return token[:2] + "••••" + token[len(token)-2:]
	}
	return token[:6] + "••••••" + token[len(token)-4:]
}

func dashboardHandler(db *storage.DB, s *scraper.Scraper) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		subs, _ := db.GetSubscriptions()
		courses, _ := db.GetAllCourses()
		cities, _ := s.FetchCities()
		views.Dashboard(subs, courses, nil, cities).Render(r.Context(), w)
	}
}
