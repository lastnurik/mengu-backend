package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	DatabaseURL            string
	JWTSecret              string
	JWTAccessTTL           time.Duration
	JWTRefreshTTL          time.Duration
	LLMApiURL              string
	LLMApiKey              string
	LLMModel               string
	LLMTimeout             time.Duration
	GoogleClientID         string
	GoogleClientSecret     string
	MicrosoftClientID      string
	MicrosoftClientSecret  string
	GoogleCalendarCreds    string
	GmailTopicName         string
	GmailSubscriptionName  string
	GmailServiceAccount    string
	GmailWebhookAudience   string
	Port                   string
	TempDir                string
	WorkerPollInterval     time.Duration
	WorkerShutdownTimeout  time.Duration
	CORSAllowedOrigins     string
	ShutdownTimeout        time.Duration
	LogLevel               string
	LogFormat              string
	RateLimitRequests      int
	RateLimitWindow        time.Duration
	HealthBind             string
	OAuthRedirectURI       string
	FrontendURL            string
}

func Load() *Config {
	return &Config{
		DatabaseURL:           getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/mengu?sslmode=disable"),
		JWTSecret:             getEnv("JWT_SECRET", "dev-secret-change-in-production"),
		JWTAccessTTL:          getDuration("JWT_ACCESS_TTL", 1*time.Hour),
		JWTRefreshTTL:         getDuration("JWT_REFRESH_TTL", 7*24*time.Hour),
		LLMApiURL:             getEnv("LLM_API_URL", ""),
		LLMApiKey:             getEnv("LLM_API_KEY", ""),
		LLMModel:              getEnv("LLM_MODEL", "gpt-4"),
		LLMTimeout:            getDuration("LLM_TIMEOUT", 30*time.Second),
		GoogleClientID:        getEnv("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret:    getEnv("GOOGLE_CLIENT_SECRET", ""),
		MicrosoftClientID:     getEnv("MICROSOFT_CLIENT_ID", ""),
		MicrosoftClientSecret: getEnv("MICROSOFT_CLIENT_SECRET", ""),
		GoogleCalendarCreds:   getEnv("GOOGLE_CALENDAR_CREDENTIALS", ""),
		GmailTopicName:        getEnv("GMAIL_TOPIC_NAME", ""),
		GmailSubscriptionName: getEnv("GMAIL_SUBSCRIPTION_NAME", ""),
		GmailServiceAccount:   getEnv("GMAIL_SERVICE_ACCOUNT", ""),
		GmailWebhookAudience:  getEnv("GMAIL_WEBHOOK_AUDIENCE", ""),
		Port:                  getEnv("PORT", "8080"),
		TempDir:               getEnv("TEMP_DIR", "/tmp/mengu"),
		WorkerPollInterval:    getDuration("WORKER_POLL_INTERVAL", 5*time.Second),
		WorkerShutdownTimeout: getDuration("WORKER_SHUTDOWN_TIMEOUT", 30*time.Second),
		CORSAllowedOrigins:    getEnv("CORS_ALLOWED_ORIGINS", "*"),
		ShutdownTimeout:       getDuration("SHUTDOWN_TIMEOUT", 10*time.Second),
		LogLevel:              getEnv("LOG_LEVEL", "info"),
		LogFormat:             getEnv("LOG_FORMAT", "json"),
		RateLimitRequests:     getInt("RATE_LIMIT_REQUESTS", 100),
		RateLimitWindow:       getDuration("RATE_LIMIT_WINDOW", 60*time.Second),
		HealthBind:            getEnv("HEALTH_BIND", ""),
		OAuthRedirectURI:      getEnv("OAUTH_REDIRECT_URI", "http://localhost:8080/api/v1/auth/oauth/callback"),
		FrontendURL:           getEnv("FRONTEND_URL", "http://localhost:5173"),
	}
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func getDuration(key string, fallback time.Duration) time.Duration {
	if val := os.Getenv(key); val != "" {
		d, err := time.ParseDuration(val)
		if err == nil {
			return d
		}
	}
	return fallback
}

func getInt(key string, fallback int) int {
	if val := os.Getenv(key); val != "" {
		i, err := strconv.Atoi(val)
		if err == nil {
			return i
		}
	}
	return fallback
}
