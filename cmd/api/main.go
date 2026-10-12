package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/Nimirandad/cards-api/internal/config"
	"github.com/Nimirandad/cards-api/internal/database"
	"github.com/Nimirandad/cards-api/internal/handler"
	"github.com/Nimirandad/cards-api/internal/repository/postgres"
	"github.com/Nimirandad/cards-api/internal/router"
	"github.com/Nimirandad/cards-api/internal/service"
)

func main() {
	cfg, err := config.LoadFromEnv()
	if err != nil {
		log.Fatal(err)
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
		log.Fatal(err)
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
	log.Printf("Listening on %s", cfg.HTTPAddr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
