package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Brook-sys/senai-courses-track/internal/api/views"
	"github.com/Brook-sys/senai-courses-track/internal/notifier"
	"github.com/Brook-sys/senai-courses-track/internal/scheduler"
	"github.com/Brook-sys/senai-courses-track/internal/scraper"
	"github.com/Brook-sys/senai-courses-track/internal/storage"
	"github.com/gorilla/mux"
)

func NewRouter(db *storage.DB, s *scraper.Scraper, sch *scheduler.Scheduler) *mux.Router {
	r := mux.NewRouter()

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
		if r.Method == "POST" {
			var cfg map[string]string
			json.NewDecoder(r.Body).Decode(&cfg)
			for k, v := range cfg {
				db.SetConfig(k, v)
			}
			fmt.Fprint(w, "saved")
			return
		}
		token, _ := db.GetConfig("telegram_token")
		chat, _ := db.GetConfig("telegram_chat_id")
		json.NewEncoder(w).Encode(map[string]string{"telegram_token": token, "telegram_chat_id": chat})
	}).Methods("GET", "POST")

	r.HandleFunc("/api/config/htmx", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		if token := r.FormValue("telegram_token"); token != "" {
			db.SetConfig("telegram_token", token)
		}
		db.SetConfig("telegram_chat_id", r.FormValue("telegram_chat_id"))
		if interval := r.FormValue("sync_interval_minutes"); interval != "" {
			sch.SetIntervalMinutes(interval)
		}
		fmt.Fprint(w, `<div class="text-green-600 text-sm mt-2">Configuração salva. Novo intervalo aplicado automaticamente.</div>`)
	}).Methods("POST")

	r.HandleFunc("/api/test-telegram", func(w http.ResponseWriter, r *http.Request) {
		var req struct{ Token, ChatID, Message string }
		json.NewDecoder(r.Body).Decode(&req)
		err := notifier.SendTestMessage(req.Token, req.ChatID, req.Message)
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		fmt.Fprint(w, "sent")
	}).Methods("POST")

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

func dashboardHandler(db *storage.DB, s *scraper.Scraper) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, _ := db.GetConfig("telegram_token")
		chat, _ := db.GetConfig("telegram_chat_id")
		interval, _ := db.GetConfig("sync_interval_minutes")
		if interval == "" {
			interval = "1440"
		}
		subs, _ := db.GetSubscriptions()
		courses, _ := db.GetAllCourses()
		cities, _ := s.FetchCities()
		tokenConfigured := token != ""
		views.Dashboard(tokenConfigured, chat, interval, subs, courses, nil, cities).Render(r.Context(), w)
	}
}
