package database

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/url"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type Config struct {
	Host     string
	Port     string
	Name     string
	User     string
	Password string
	SSLMode  string
}

// Open establishes a connection to the PostgreSQL database using the provided configuration.
func Open(ctx context.Context, cfg Config) (*sql.DB, error) {
	dbURL := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(cfg.User, cfg.Password),
		Host:   net.JoinHostPort(cfg.Host, cfg.Port),
		Path:   cfg.Name,
	}
	query := dbURL.Query()
	query.Set("sslmode", cfg.SSLMode)
	dbURL.RawQuery = query.Encode()

	db, err := sql.Open("pgx", dbURL.String())
	if err != nil {
		return nil, fmt.Errorf("abrir pool PostgreSQL: %w", err)
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("comprobar conexión PostgreSQL: %w", err)
	}
	return db, nil
}
