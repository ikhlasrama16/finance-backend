package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"finance-monitor/backend/internal/config"
	"finance-monitor/backend/internal/database"
	"finance-monitor/backend/internal/server"
)

func main() {
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := database.NewPostgresPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	log.Println("PostgreSQL connected")

	srv := server.NewWithOptions(cfg.Port, db, server.Options{
		IngestAPIKey: cfg.IngestAPIKey, AppEnv: cfg.AppEnv, CORSAllowedOrigins: cfg.CORSAllowedOrigins,
		OpenRouterAPIKey: cfg.OpenRouterAPIKey, OpenRouterModel: cfg.OpenRouterModel,
	})

	log.Printf(
		"Finance API running on http://localhost:%s",
		cfg.Port,
	)

	if err := srv.RunContext(ctx); err != nil &&
		!errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
