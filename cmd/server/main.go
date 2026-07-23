package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Brook-sys/senai-courses-track/internal/api"
	"github.com/Brook-sys/senai-courses-track/internal/appconfig"
	"github.com/Brook-sys/senai-courses-track/internal/scheduler"
	"github.com/Brook-sys/senai-courses-track/internal/scraper"
	"github.com/Brook-sys/senai-courses-track/internal/storage"
	"github.com/Brook-sys/senai-courses-track/internal/telegrambot"
	"github.com/Brook-sys/senai-courses-track/internal/telegramclient"
)

func main() {
	cfg := appconfig.FromEnv()
	db, err := storage.New(cfg.DBPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := scraper.New()
	tgClient := telegramclient.NewClient()

	sched := scheduler.New(db, s)
	sched.Start(ctx, tgClient)

	bot := telegrambot.New(db, s, tgClient)
	bot.Start(ctx)

	r := api.NewRouter(db, s, sched, bot)
	srv := &http.Server{
		Addr:    cfg.Addr,
		Handler: r,
	}

	go func() {
		log.Printf("Server starting on %s", cfg.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig

	log.Println("Shutting down...")
	cancel() // Cancel context for bot and scheduler

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}
}
