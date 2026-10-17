// Package config membaca konfigurasi dari environment.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env          string
	HTTPAddr     string
	DBDSN        string
	RedisAddr    string
	RedisPass    string
	JWTSecret    string
	CookieSecure bool
	StorageDir   string
	FrontendURL  string
	CORSOrigins  []string
	// TrustedProxies: alamat/CIDR reverse proxy yang boleh menentukan IP klien lewat X-Forwarded-For (kosong = tidak ada).
	TrustedProxies []string

	AccessTTL  time.Duration
	RefreshTTL time.Duration

	LoginMaxFails int
	LoginLockTTL  time.Duration

	SMTPHost string
	SMTPPort int
	SMTPFrom string
	SMTPUser string
	SMTPPass string
}

func (c Config) IsProd() bool { return c.Env == "production" }

func Load() (Config, error) {
	c := Config{
		Env:           get("APP_ENV", "development"),
		HTTPAddr:      get("HTTP_ADDR", ":8080"),
		DBDSN:         get("DB_DSN", "sikerja:sikerja@tcp(127.0.0.1:3306)/sikerja?charset=utf8mb4&parseTime=true&loc=Local"),
		RedisAddr:     get("REDIS_ADDR", "127.0.0.1:6379"),
		RedisPass:     get("REDIS_PASSWORD", ""),
		JWTSecret:     get("JWT_SECRET", ""),
		StorageDir:    get("STORAGE_DIR", "./storage"),
		FrontendURL:   strings.TrimRight(get("FRONTEND_URL", "http://localhost:5173"), "/"),
		AccessTTL:     15 * time.Minute,
		RefreshTTL:    7 * 24 * time.Hour,
		LoginMaxFails: 5,
		LoginLockTTL:  time.Minute,
		SMTPHost:      get("SMTP_HOST", "127.0.0.1"),
		SMTPFrom:      get("SMTP_FROM", "SiKerja <no-reply@sikerja.test>"),
		SMTPUser:      get("SMTP_USER", ""),
		SMTPPass:      get("SMTP_PASSWORD", ""),
	}
	port, err := strconv.Atoi(get("SMTP_PORT", "1025"))
	if err != nil {
		return c, fmt.Errorf("SMTP_PORT tidak valid: %w", err)
	}
	c.SMTPPort = port
	c.CookieSecure = get("COOKIE_SECURE", strconv.FormatBool(c.IsProd())) == "true"
	for _, o := range strings.Split(get("CORS_ORIGINS", c.FrontendURL), ",") {
		if o = strings.TrimSpace(o); o != "" {
			c.CORSOrigins = append(c.CORSOrigins, o)
		}
	}
	for _, p := range strings.Split(get("TRUSTED_PROXIES", ""), ",") {
		if p = strings.TrimSpace(p); p != "" {
			c.TrustedProxies = append(c.TrustedProxies, p)
		}
	}
	if c.IsProd() && len(c.JWTSecret) < 32 {
		return c, fmt.Errorf("JWT_SECRET wajib diisi (min. 32 karakter) di production")
	}
	if c.JWTSecret == "" {
		c.JWTSecret = "dev-only-secret-change-me-dev-only-secret"
	}
	return c, nil
}

func get(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}
