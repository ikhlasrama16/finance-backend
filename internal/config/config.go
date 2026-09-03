package config

import (
	"bufio"
	"errors"
	"os"
	"strings"
)

type Config struct {
	Port                      string
	DatabaseURL               string
	IngestAPIKey              string
	AppEnv                    string
	CORSAllowedOrigins        []string
	OpenRouterAPIKey          string
	OpenRouterModel           string
	OpenRouterClassifierModel string
}

func loadDotEnv() {
	paths := []string{".env", "../.env"}
	for _, p := range paths {
		f, err := os.Open(p)
		if err != nil {
			continue
		}
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			if strings.HasPrefix(line, "$env:") {
				line = strings.TrimPrefix(line, "$env:")
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				continue
			}
			k := strings.TrimSpace(parts[0])
			v := strings.TrimSpace(parts[1])
			v = strings.Trim(v, `"'`)
			if os.Getenv(k) == "" && k != "" {
				_ = os.Setenv(k, v)
			}
		}
		break
	}
}

func Load() Config {
	loadDotEnv()
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
	openRouterClassifierModel := os.Getenv("OPENROUTER_CLASSIFIER_MODEL")
	if openRouterClassifierModel == "" {
		openRouterClassifierModel = openRouterModel
	}
	var origins []string
	for _, origin := range strings.Split(os.Getenv("CORS_ALLOWED_ORIGINS"), ",") {
		if value := strings.TrimSpace(origin); value != "" {
			origins = append(origins, value)
		}
	}

	return Config{
		Port:                      port,
		DatabaseURL:               os.Getenv("DATABASE_URL"),
		IngestAPIKey:              os.Getenv("INGEST_API_KEY"),
		AppEnv:                    env,
		CORSAllowedOrigins:        origins,
		OpenRouterAPIKey:          os.Getenv("OPENROUTER_API_KEY"),
		OpenRouterModel:           openRouterModel,
		OpenRouterClassifierModel: openRouterClassifierModel,
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
