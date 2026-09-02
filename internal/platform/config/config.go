// Package config loads configuration from the environment only. Nothing
// secret is in git; the documented surface is .env.example and the real file
// is git-ignored.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env             string
	HTTPAddr        string
	DatabaseURL     string
	TestDatabaseURL string

	JWTSecret       string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	TOTPIssuer      string

	SMTPHost      string
	SMTPPort      int
	SMTPFrom      string
	NotifyEnabled bool

	S3Endpoint  string
	S3AccessKey string
	S3SecretKey string
	S3Bucket    string
	S3UseSSL    bool

	ImportDropPath string
	LogLevel       string
	MetricsEnabled bool
	TrustedProxies []string
}

// Redacted is what may be printed at boot. A startup line that prints the
// database URL leaks the password into the journal, where it stays.
func (c Config) Redacted() map[string]string {
	return map[string]string{
		"env":        c.Env,
		"http_addr":  c.HTTPAddr,
		"database":   redactURL(c.DatabaseURL),
		"smtp":       fmt.Sprintf("%s:%d", c.SMTPHost, c.SMTPPort),
		"s3":         c.S3Endpoint,
		"jwt_secret": mask(c.JWTSecret),
		"log_level":  c.LogLevel,
	}
}

func mask(s string) string {
	if s == "" {
		return "(unset)"
	}
	return "••••••"
}

func redactURL(u string) string {
	// postgres://user:pass@host/db -> postgres://user:••••••@host/db
	at := strings.LastIndex(u, "@")
	if at < 0 {
		return u
	}
	scheme := strings.Index(u, "://")
	if scheme < 0 {
		return u
	}
	creds := u[scheme+3 : at]
	if colon := strings.Index(creds, ":"); colon >= 0 {
		creds = creds[:colon] + ":••••••"
	}
	return u[:scheme+3] + creds + u[at:]
}

// Load reads the environment and refuses to start on a missing required
// value. A service that boots with an empty JWT secret is worse than one that
// does not boot.
func Load() (Config, error) {
	c := Config{
		Env:             env("MC_ENV", "development"),
		HTTPAddr:        env("MC_HTTP_ADDR", "127.0.0.1:8081"),
		DatabaseURL:     os.Getenv("MC_DATABASE_URL"),
		TestDatabaseURL: os.Getenv("MC_TEST_DATABASE_URL"),
		JWTSecret:       os.Getenv("MC_JWT_SECRET"),
		TOTPIssuer:      env("MC_TOTP_ISSUER", "Marketing Calendar"),
		SMTPHost:        env("MC_SMTP_HOST", "127.0.0.1"),
		SMTPFrom:        env("MC_SMTP_FROM", "marketing-calendar@localhost"),
		S3Endpoint:      os.Getenv("MC_S3_ENDPOINT"),
		S3AccessKey:     os.Getenv("MC_S3_ACCESS_KEY"),
		S3SecretKey:     os.Getenv("MC_S3_SECRET_KEY"),
		S3Bucket:        env("MC_S3_BUCKET", "marketing-calendar"),
		ImportDropPath:  env("MC_IMPORT_DROP_PATH", "/srv/marketing_calendar/import"),
		LogLevel:        env("MC_LOG_LEVEL", "info"),
	}
	c.SMTPPort = envInt("MC_SMTP_PORT", 1025)
	c.AccessTokenTTL = envDuration("MC_ACCESS_TOKEN_TTL", 15*time.Minute)
	c.RefreshTokenTTL = envDuration("MC_REFRESH_TOKEN_TTL", 720*time.Hour)
	c.NotifyEnabled = envBool("MC_NOTIFY_ENABLED", true)
	c.S3UseSSL = envBool("MC_S3_USE_SSL", false)
	c.MetricsEnabled = envBool("MC_METRICS_ENABLED", true)
	if p := os.Getenv("MC_TRUSTED_PROXIES"); p != "" {
		c.TrustedProxies = strings.Split(p, ",")
	}

	var missing []string
	if c.DatabaseURL == "" {
		missing = append(missing, "MC_DATABASE_URL")
	}
	if c.JWTSecret == "" {
		missing = append(missing, "MC_JWT_SECRET")
	}
	if len(missing) > 0 {
		return c, fmt.Errorf("konfigurasi wajib tidak ada: %s", strings.Join(missing, ", "))
	}
	// A short secret is a weak secret; refusing at boot is the only moment
	// anyone will notice.
	if len(c.JWTSecret) < 32 {
		return c, fmt.Errorf("MC_JWT_SECRET harus minimal 32 karakter, saat ini %d", len(c.JWTSecret))
	}
	return c, nil
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func envInt(k string, def int) int {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envBool(k string, def bool) bool {
	if v := os.Getenv(k); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}

func envDuration(k string, def time.Duration) time.Duration {
	if v := os.Getenv(k); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}
