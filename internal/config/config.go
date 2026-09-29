// Package config loads process settings from the environment.
package config

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"
)

// Config is everything the process needs to start. Secrets stay here and
// are not written to logs by the rest of the program.
type Config struct {
	HTTPAddr       string
	DatabaseURL    string
	JWTSecret      string
	AccessTTL      time.Duration
	RefreshTTL     time.Duration
	CORSOrigins    []string
	LogLevel       slog.Level
	DataDir        string
	TrustProxy     bool
	AdminUsername  string
	AdminPassword  string
	AdminEmail     string
	AdminFullName  string
	EnvFile        string
	WarnWeakSecret bool
	WarnWeakAdmin  bool
}

// Load reads QC_ENV_FILE (default .env) and then QC_* variables.
func Load() (Config, error) {
	envFile := os.Getenv("QC_ENV_FILE")
	if envFile == "" {
		envFile = ".env"
	}
	if err := loadDotEnv(envFile); err != nil {
		return Config{}, err
	}

	cfg := Config{
		HTTPAddr:      getenv("QC_HTTP_ADDR", "127.0.0.1:8080"),
		DatabaseURL:   strings.TrimSpace(os.Getenv("QC_DATABASE_URL")),
		JWTSecret:     os.Getenv("QC_JWT_SECRET"),
		CORSOrigins:   splitCSV(os.Getenv("QC_CORS_ORIGINS")),
		DataDir:       getenv("QC_DATA_DIR", "./data"),
		TrustProxy:    strings.EqualFold(strings.TrimSpace(os.Getenv("QC_TRUST_PROXY")), "true"),
		AdminUsername: getenv("QC_ADMIN_USERNAME", "admin"),
		AdminPassword: os.Getenv("QC_ADMIN_PASSWORD"),
		AdminEmail:    getenv("QC_ADMIN_EMAIL", "admin@localhost"),
		AdminFullName: getenv("QC_ADMIN_FULL_NAME", "Administrator"),
		EnvFile:       envFile,
	}

	var err error
	cfg.AccessTTL, err = time.ParseDuration(getenv("QC_ACCESS_TTL", "15m"))
	if err != nil {
		return Config{}, fmt.Errorf("QC_ACCESS_TTL: %w", err)
	}
	cfg.RefreshTTL, err = time.ParseDuration(getenv("QC_REFRESH_TTL", "168h"))
	if err != nil {
		return Config{}, fmt.Errorf("QC_REFRESH_TTL: %w", err)
	}
	if cfg.AccessTTL < time.Minute || cfg.RefreshTTL < time.Hour || cfg.RefreshTTL <= cfg.AccessTTL {
		return Config{}, fmt.Errorf("token lifetimes must be at least 1m and 1h, and refresh must outlive access")
	}

	switch strings.ToLower(getenv("QC_LOG_LEVEL", "info")) {
	case "debug":
		cfg.LogLevel = slog.LevelDebug
	case "info":
		cfg.LogLevel = slog.LevelInfo
	case "warn", "warning":
		cfg.LogLevel = slog.LevelWarn
	case "error":
		cfg.LogLevel = slog.LevelError
	default:
		return Config{}, fmt.Errorf("QC_LOG_LEVEL must be debug, info, warn, or error")
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("QC_DATABASE_URL is required")
	}
	if len(cfg.JWTSecret) < 32 {
		return Config{}, fmt.Errorf("QC_JWT_SECRET must be at least 32 characters")
	}
	if strings.Contains(cfg.JWTSecret, "replace-with") {
		cfg.WarnWeakSecret = true
	}
	if cfg.AdminPassword == "change-me-now" || strings.Contains(cfg.AdminPassword, "change-me") {
		cfg.WarnWeakAdmin = true
	}
	return cfg, nil
}

func getenv(key, fallback string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	return v
}

func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
