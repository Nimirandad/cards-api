package config

import (
	"fmt"
	"os"
)

type Config struct {
	HTTPAddr    string
	DBHost      string
	DBPort      string
	DBName      string
	DBUser      string
	DBPassword  string
	DBSSLMode   string
	PANHashSalt string
}


// LoadFromEnv loads configuration from environment variables, returning an error if any required variable is missing or invalid.
func LoadFromEnv() (Config, error) {
	dbHost, err := requiredEnv("DB_HOST")
	if err != nil {
		return Config{}, err
	}
	dbName, err := requiredEnv("DB_NAME")
	if err != nil {
		return Config{}, err
	}
	dbUser, err := requiredEnv("DB_USER")
	if err != nil {
		return Config{}, err
	}
	dbPassword, err := requiredEnv("DB_PASSWORD")
	if err != nil {
		return Config{}, err
	}
	panHashSalt, err := requiredEnv("PAN_HASH_SALT")
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		HTTPAddr:    envOrDefault("HTTP_ADDR", ":8080"),
		DBHost:      dbHost,
		DBPort:      envOrDefault("DB_PORT", "5432"),
		DBName:      dbName,
		DBUser:      dbUser,
		DBPassword:  dbPassword,
		DBSSLMode:   envOrDefault("DB_SSLMODE", "disable"),
		PANHashSalt: panHashSalt,
	}
	if len(cfg.PANHashSalt) < 32 {
		return Config{}, fmt.Errorf("PAN_HASH_SALT debe tener al menos 32 caracteres")
	}
	return cfg, nil
}

func requiredEnv(name string) (string, error) {
	value := os.Getenv(name)
	if value == "" {
		return "", fmt.Errorf("falta configurar la variable de entorno %s", name)
	}
	return value, nil
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
