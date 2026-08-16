package config

import (
	"errors"
	"os"
	"strings"
)

type Config struct {
	Port               string
	DatabaseURL        string
	IngestAPIKey       string
	AppEnv             string
	CORSAllowedOrigins []string
	OpenRouterAPIKey   string
	OpenRouterModel    string
}

func Load() Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "development"
	}
	openRouterModel := os.Getenv("OPENROUTER_MODEL")
	if openRouterModel == "" {
		openRouterModel = "openrouter/free"
	}
	var origins []string
	for _, origin := range strings.Split(os.Getenv("CORS_ALLOWED_ORIGINS"), ",") {
		if value := strings.TrimSpace(origin); value != "" {
			origins = append(origins, value)
		}
	}

	return Config{
		Port:               port,
		DatabaseURL:        os.Getenv("DATABASE_URL"),
		IngestAPIKey:       os.Getenv("INGEST_API_KEY"),
		AppEnv:             env,
		CORSAllowedOrigins: origins,
		OpenRouterAPIKey:   os.Getenv("OPENROUTER_API_KEY"),
		OpenRouterModel:    openRouterModel,
	}
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.DatabaseURL) == "" {
		return errors.New("DATABASE_URL is required")
	}
	if strings.TrimSpace(c.IngestAPIKey) == "" {
		return errors.New("INGEST_API_KEY is required")
	}
	if c.AppEnv != "development" && c.AppEnv != "production" {
		return errors.New("APP_ENV must be development or production")
	}
	return nil
}
