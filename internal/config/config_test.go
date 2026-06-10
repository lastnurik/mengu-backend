package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	os.Unsetenv("DATABASE_URL")
	os.Unsetenv("PORT")
	os.Unsetenv("LOG_LEVEL")

	cfg := Load()

	if cfg.DatabaseURL != "postgres://postgres:postgres@localhost:5432/mengu?sslmode=disable" {
		t.Errorf("expected default DATABASE_URL, got %s", cfg.DatabaseURL)
	}
	if cfg.Port != "8080" {
		t.Errorf("expected default PORT 8080, got %s", cfg.Port)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("expected default LOG_LEVEL info, got %s", cfg.LogLevel)
	}
	if cfg.JWTAccessTTL != 1*time.Hour {
		t.Errorf("expected default JWT_ACCESS_TTL 1h, got %v", cfg.JWTAccessTTL)
	}
}

func TestLoadFromEnv(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgres://user:pass@host:5432/db")
	os.Setenv("PORT", "3000")
	os.Setenv("LOG_LEVEL", "debug")
	os.Setenv("JWT_ACCESS_TTL", "30m")
	defer func() {
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("PORT")
		os.Unsetenv("LOG_LEVEL")
		os.Unsetenv("JWT_ACCESS_TTL")
	}()

	cfg := Load()

	if cfg.DatabaseURL != "postgres://user:pass@host:5432/db" {
		t.Errorf("expected DATABASE_URL from env, got %s", cfg.DatabaseURL)
	}
	if cfg.Port != "3000" {
		t.Errorf("expected PORT from env, got %s", cfg.Port)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("expected LOG_LEVEL from env, got %s", cfg.LogLevel)
	}
	if cfg.JWTAccessTTL != 30*time.Minute {
		t.Errorf("expected JWT_ACCESS_TTL 30m, got %v", cfg.JWTAccessTTL)
	}
}
