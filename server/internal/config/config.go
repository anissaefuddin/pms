package config

import (
	"encoding/base64"
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
	// DisableIPAllowlist is the way back in after an allowlist locked everyone out: the operator sets it and restarts (D-95).
	DisableIPAllowlist bool
	TrustProxy         bool // read X-Forwarded-For; only set behind a proxy that overwrites it
	// PublicLinks (PMS_PUBLIC_LINKS) is off by default. Public dashboards, forms and Gantt snapshots are served only when it is true (B3-17).
	PublicLinks bool
	// PydioListing (PMS_PYDIO_LISTING) is off by default. The Pydio folder listing stays disabled until identity and ACL are verified (B4-07).
	PydioListing   bool
	MaxSeats       int
	SMTPHost       string
	SMTPPort       string
	SMTPUser       string
	SMTPPassword   string
	SMTPFrom       string
	SMTPFromAuth   string // sender for account links (activation, reset); defaults to SMTPFrom
	SMTPReplyTo    string
	BackupAlertURL string
	FilesDir       string
	MaxUploadMB    int
	// Storage is local (PMS_FILES_DIR), s3 (MinIO or AWS; S3URL is endpoint/bucket[/prefix]) or pydio (Cells gateway folder URL).
	Storage, S3URL, S3Region, S3AccessKey, S3SecretKey, PydioURL, PydioToken string
	// TwoFactorKey and TwoFactorKeyPrevious are 32-byte keys for TOTP secrets; without TwoFactorKey 2FA cannot be turned on.
	TwoFactorKey, TwoFactorKeyPrevious []byte
	// OTPKey authenticates short login-code hashes so a database copy cannot be used to guess codes offline.
	OTPKey []byte
	// SSOKey and SSOKeyPrevious encrypt the client secret of single sign-on providers (same format and rotation as the 2FA key).
	SSOKey, SSOKeyPrevious []byte
	// ConnectorKey and ConnectorKeyPrevious encrypt connector secrets (same format and rotation as the 2FA key).
	ConnectorKey, ConnectorKeyPrevious []byte
	// Seeds is PMS_SEED: comma-separated starter configuration applied once at start (see package seed).
	Seeds string
	// PydioLink* (MVP-13) let an item panel list the files of a Pydio Cells folder linked in a text field, read-only:
	// LinkURL is the Cells address users open (https://dms.example.com), LinkToken a read-only account's personal access
	// token, LinkRoots the top folders (workspace slug[/path]) a link may point into. Empty URL = feature off.
	PydioLinkURL, PydioLinkToken string
	PydioLinkRoots               []string
}

func Load() (Config, error) {
	maxSeats, err := strconv.Atoi(value("MAX_SEATS", "25"))
	if err != nil || maxSeats < 1 {
		return Config{}, fmt.Errorf("MAX_SEATS must be a positive integer")
	}
	cfg := Config{
		DatabaseURL: os.Getenv("DATABASE_URL"), ListenAddress: value("LISTEN_ADDRESS", ":8080"), MetricsAddress: value("METRICS_ADDRESS", ":9090"),
		PublicURL:    strings.TrimRight(value("PUBLIC_URL", "http://localhost:8080"), "/"),
		CookieSecure: value("COOKIE_SECURE", "false") == "true", TrustProxy: value("TRUST_PROXY", "false") == "true", PublicLinks: value("PMS_PUBLIC_LINKS", "false") == "true", PydioListing: value("PMS_PYDIO_LISTING", "false") == "true", DisableIPAllowlist: value("PMS_DISABLE_IP_ALLOWLIST", "false") == "true", MaxSeats: maxSeats,
		SMTPHost: os.Getenv("SMTP_HOST"), SMTPPort: value("SMTP_PORT", "587"), SMTPUser: os.Getenv("SMTP_USER"),
		SMTPPassword: os.Getenv("SMTP_PASSWORD"), SMTPFrom: value("SMTP_FROM", "PMS <noreply@localhost>"),
		BackupAlertURL: os.Getenv("BACKUP_ALERT_URL"),
		FilesDir:       value("PMS_FILES_DIR", "/data/files"),
		Seeds:          os.Getenv("PMS_SEED"),
	}
	cfg.SMTPFromAuth, cfg.SMTPReplyTo = value("SMTP_FROM_AUTH", cfg.SMTPFrom), os.Getenv("SMTP_REPLY_TO")
	cfg.MaxUploadMB, err = strconv.Atoi(value("PMS_MAX_UPLOAD_MB", "100"))
	if err != nil || cfg.MaxUploadMB < 1 || cfg.MaxUploadMB > 1024 {
		return Config{}, fmt.Errorf("PMS_MAX_UPLOAD_MB must be between 1 and 1024")
	}
	cfg.Storage, cfg.S3URL, cfg.S3Region = value("PMS_STORAGE", "local"), strings.TrimRight(os.Getenv("PMS_S3_URL"), "/"), value("PMS_S3_REGION", "us-east-1")
	cfg.S3AccessKey, cfg.S3SecretKey = os.Getenv("PMS_S3_ACCESS_KEY"), os.Getenv("PMS_S3_SECRET_KEY")
	cfg.PydioURL, cfg.PydioToken = strings.TrimRight(os.Getenv("PMS_PYDIO_URL"), "/"), os.Getenv("PMS_PYDIO_TOKEN")
	switch {
	case cfg.Storage == "pydio":
		// B4-08: the Pydio storage backend signs requests with a fixed gateway secret and is not verified against a live gateway, so it
		// cannot be enabled. Attachments stay on local or S3 storage. Pydio links are configured separately (PMS_PYDIO_LINK_URL).
		return Config{}, fmt.Errorf("PMS_STORAGE=pydio is not supported in this release (B4-08); use local or s3")
	case cfg.Storage == "local":
	case cfg.Storage == "s3" && cfg.S3URL != "" && cfg.S3AccessKey != "" && cfg.S3SecretKey != "":
	case cfg.Storage == "pydio" && cfg.PydioURL != "" && cfg.PydioToken != "":
	default:
		return Config{}, fmt.Errorf("PMS_STORAGE must be local, s3 (needs PMS_S3_URL, PMS_S3_ACCESS_KEY, PMS_S3_SECRET_KEY) or pydio (needs PMS_PYDIO_URL, PMS_PYDIO_TOKEN)")
	}
	cfg.PydioLinkURL, cfg.PydioLinkToken = strings.TrimRight(os.Getenv("PMS_PYDIO_LINK_URL"), "/"), os.Getenv("PMS_PYDIO_LINK_TOKEN")
	// B4-02: production folder references are https only. The origin comes from the operator and is checked once, here.
	if cfg.PydioLinkURL != "" && !strings.HasPrefix(strings.ToLower(cfg.PydioLinkURL), "https://") {
		return Config{}, fmt.Errorf("PMS_PYDIO_LINK_URL must use https (B4-02)")
	}
	for _, root := range strings.Split(os.Getenv("PMS_PYDIO_LINK_ROOTS"), ",") {
		if root = strings.Trim(strings.TrimSpace(root), "/"); root != "" {
			cfg.PydioLinkRoots = append(cfg.PydioLinkRoots, root)
		}
	}
	if cfg.PydioLinkURL != "" {
		u, err := url.Parse(cfg.PydioLinkURL)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || u.Path != "" || u.RawQuery != "" {
			return Config{}, fmt.Errorf("PMS_PYDIO_LINK_URL must be the Cells address without a path, for example https://dms.example.com")
		}
		// Deny by default: without roots a link could point anywhere the token can read.
		if cfg.PydioLinkToken == "" || len(cfg.PydioLinkRoots) == 0 {
			return Config{}, fmt.Errorf("PMS_PYDIO_LINK_URL needs PMS_PYDIO_LINK_TOKEN and PMS_PYDIO_LINK_ROOTS")
		}
	}
	for env, dst := range map[string]*[]byte{"PMS_2FA_KEY": &cfg.TwoFactorKey, "PMS_2FA_KEY_PREVIOUS": &cfg.TwoFactorKeyPrevious, "PMS_OTP_KEY": &cfg.OTPKey, "PMS_SSO_KEY": &cfg.SSOKey, "PMS_SSO_KEY_PREVIOUS": &cfg.SSOKeyPrevious, "PMS_CONNECTOR_KEY": &cfg.ConnectorKey, "PMS_CONNECTOR_KEY_PREVIOUS": &cfg.ConnectorKeyPrevious} {
		if raw := os.Getenv(env); raw != "" {
			key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(raw))
			if err != nil || len(key) != 32 {
				return Config{}, fmt.Errorf("%s must be 32 random bytes encoded as base64 (for example `openssl rand -base64 32`)", env)
			}
			*dst = key
		}
	}
	if cfg.TwoFactorKeyPrevious != nil && cfg.TwoFactorKey == nil {
		return Config{}, fmt.Errorf("PMS_2FA_KEY_PREVIOUS needs PMS_2FA_KEY")
	}
	if cfg.SSOKeyPrevious != nil && cfg.SSOKey == nil {
		return Config{}, fmt.Errorf("PMS_SSO_KEY_PREVIOUS needs PMS_SSO_KEY")
	}
	if cfg.ConnectorKeyPrevious != nil && cfg.ConnectorKey == nil {
		return Config{}, fmt.Errorf("PMS_CONNECTOR_KEY_PREVIOUS needs PMS_CONNECTOR_KEY")
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
