package main

import (
	"log"
	"net/http"

	"github.com/Brook-sys/senai-courses-track/internal/api"
	"github.com/Brook-sys/senai-courses-track/internal/appconfig"
	"github.com/Brook-sys/senai-courses-track/internal/scheduler"
	"github.com/Brook-sys/senai-courses-track/internal/scraper"
	"github.com/Brook-sys/senai-courses-track/internal/storage"
	"github.com/Brook-sys/senai-courses-track/internal/telegrambot"
)

func main() {
	cfg := appconfig.FromEnv()
	db, err := storage.New(cfg.DBPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	s := scraper.New()
	sched := scheduler.New(db, s)

	// Schedule daily update at 08:00
	sched.Start("0 8 * * *")

	bot := telegrambot.New(db, s)
	bot.Start()

	r := api.NewRouter(db, s, sched)
	log.Printf("Server starting on %s", cfg.Addr)
	log.Fatal(http.ListenAndServe(cfg.Addr, r))
}
