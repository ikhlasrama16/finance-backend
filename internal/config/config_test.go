package config

import "testing"

func TestConfigValidate(t *testing.T) {
	base := Config{DatabaseURL: "postgres://example", IngestAPIKey: "secret", AppEnv: "production"}
	if err := base.Validate(); err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]Config{
		"missing database": {IngestAPIKey: "secret", AppEnv: "production"},
		"missing key":      {DatabaseURL: "postgres://example", AppEnv: "production"},
		"invalid env":      {DatabaseURL: "postgres://example", IngestAPIKey: "secret", AppEnv: "staging"},
	} {
		t.Run(name, func(t *testing.T) {
			if err := value.Validate(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}
