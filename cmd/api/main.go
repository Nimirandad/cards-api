package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/Nimirandad/cards-api/internal/config"
	"github.com/Nimirandad/cards-api/internal/database"
	"github.com/Nimirandad/cards-api/internal/handler"
	"github.com/Nimirandad/cards-api/internal/repository/postgres"
	"github.com/Nimirandad/cards-api/internal/router"
	"github.com/Nimirandad/cards-api/internal/service"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg, err := config.LoadFromEnv()
	if err != nil {
		slog.Error("configuration_load_failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	dbConfig := database.Config{
		Host:     cfg.DBHost,
		Port:     cfg.DBPort,
		Name:     cfg.DBName,
		User:     cfg.DBUser,
		Password: cfg.DBPassword,
		SSLMode:  cfg.DBSSLMode,
	}

	dbCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	db, err := database.Open(dbCtx, dbConfig)
	cancel()
	if err != nil {
		slog.Error("database_connection_failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer db.Close()

	cardRepository := postgres.NewCardRepository(db, 24*time.Hour)
	cardService := service.NewCardService(cardRepository, cfg.PANHashSalt)
	cardHandler := handler.NewCardHandler(cardService)
	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router.New(cardHandler),
		ReadHeaderTimeout: 5 * time.Second,
	}
	slog.Info("server_started", slog.String("address", cfg.HTTPAddr))
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server_failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
