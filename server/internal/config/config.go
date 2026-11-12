package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	DatabaseURL    string
	ListenAddress  string
	MetricsAddress string
	PublicURL      string
	CookieSecure   bool
	TrustProxy     bool // read X-Forwarded-For; only set behind a proxy that overwrites it
	MaxSeats       int
	SMTPHost       string
	SMTPPort       string
	SMTPUser       string
	SMTPPassword   string
	SMTPFrom       string
	BackupAlertURL string
	FilesDir       string
	MaxUploadMB    int
}

func Load() (Config, error) {
	maxSeats, err := strconv.Atoi(value("MAX_SEATS", "25"))
	if err != nil || maxSeats < 1 {
		return Config{}, fmt.Errorf("MAX_SEATS must be a positive integer")
	}
	cfg := Config{
		DatabaseURL: os.Getenv("DATABASE_URL"), ListenAddress: value("LISTEN_ADDRESS", ":8080"), MetricsAddress: value("METRICS_ADDRESS", ":9090"),
		PublicURL:    strings.TrimRight(value("PUBLIC_URL", "http://localhost:8080"), "/"),
		CookieSecure: value("COOKIE_SECURE", "false") == "true", TrustProxy: value("TRUST_PROXY", "false") == "true", MaxSeats: maxSeats,
		SMTPHost: os.Getenv("SMTP_HOST"), SMTPPort: value("SMTP_PORT", "587"), SMTPUser: os.Getenv("SMTP_USER"),
		SMTPPassword: os.Getenv("SMTP_PASSWORD"), SMTPFrom: value("SMTP_FROM", "PMS <noreply@localhost>"),
		BackupAlertURL: os.Getenv("BACKUP_ALERT_URL"),
		FilesDir:       value("PMS_FILES_DIR", "/data/files"),
	}
	cfg.MaxUploadMB, err = strconv.Atoi(value("PMS_MAX_UPLOAD_MB", "100"))
	if err != nil || cfg.MaxUploadMB < 1 || cfg.MaxUploadMB > 1024 {
		return Config{}, fmt.Errorf("PMS_MAX_UPLOAD_MB must be between 1 and 1024")
	}
	if cfg.DatabaseURL == "" {
		// Parts instead of a URL: url.UserPassword escapes the password, so any characters work.
		host, password := os.Getenv("DB_HOST"), os.Getenv("DB_PASSWORD")
		if host == "" || password == "" {
			return Config{}, fmt.Errorf("set DATABASE_URL, or DB_HOST and DB_PASSWORD (optional DB_USER, DB_NAME, DB_PORT)")
		}
		u := url.URL{Scheme: "postgres", User: url.UserPassword(value("DB_USER", "pms"), password), Host: net.JoinHostPort(host, value("DB_PORT", "5432")), Path: "/" + value("DB_NAME", "pms"), RawQuery: "sslmode=disable"}
		cfg.DatabaseURL = u.String()
	}
	return cfg, nil
}

func value(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

const IdleSessionTimeout = 14 * 24 * time.Hour
const AbsoluteSessionTimeout = 30 * 24 * time.Hour
