package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Environment            string
	Address                string
	DatabaseURL            string
	JWTSecret              []byte
	JWTTTL                 time.Duration
	BootstrapAdminEmail    string
	BootstrapAdminPassword string
	DevNodeEnrollmentToken string
	TrustedProxies         []string
	StagingDir             string
	DeliveryDir            string
}

func Load() (Config, error) {
	var trustedProxies []string
	if raw := strings.TrimSpace(os.Getenv("TRUSTED_PROXIES")); raw != "" {
		for _, item := range strings.Split(raw, ",") {
			if proxy := strings.TrimSpace(item); proxy != "" {
				trustedProxies = append(trustedProxies, proxy)
			}
		}
	}
	cfg := Config{
		TrustedProxies:         trustedProxies,
		Environment:            env("APP_ENV", "production"),
		Address:                env("APP_ADDR", ":8080"),
		DatabaseURL:            strings.TrimSpace(os.Getenv("DATABASE_URL")),
		JWTSecret:              []byte(strings.TrimSpace(os.Getenv("JWT_SECRET"))),
		BootstrapAdminEmail:    strings.ToLower(strings.TrimSpace(os.Getenv("BOOTSTRAP_ADMIN_EMAIL"))),
		BootstrapAdminPassword: os.Getenv("BOOTSTRAP_ADMIN_PASSWORD"),
		DevNodeEnrollmentToken: strings.TrimSpace(os.Getenv("DEV_NODE_ENROLLMENT_TOKEN")),
		StagingDir:             env("STAGING_DIR", "./data/staging"),
		DeliveryDir:            env("DELIVERY_DIR", "./data/delivery"),
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if len(cfg.JWTSecret) < 32 {
		return Config{}, fmt.Errorf("JWT_SECRET must contain at least 32 bytes")
	}
	hours, err := strconv.Atoi(env("JWT_TTL_HOURS", "12"))
	if err != nil || hours < 1 || hours > 168 {
		return Config{}, fmt.Errorf("JWT_TTL_HOURS must be between 1 and 168")
	}
	cfg.JWTTTL = time.Duration(hours) * time.Hour
	if (cfg.BootstrapAdminEmail == "") != (cfg.BootstrapAdminPassword == "") {
		return Config{}, fmt.Errorf("BOOTSTRAP_ADMIN_EMAIL and BOOTSTRAP_ADMIN_PASSWORD must both be set or both be empty")
	}
	if cfg.BootstrapAdminPassword != "" && len(cfg.BootstrapAdminPassword) < 12 {
		return Config{}, fmt.Errorf("BOOTSTRAP_ADMIN_PASSWORD must contain at least 12 characters")
	}
	if cfg.DevNodeEnrollmentToken != "" {
		if cfg.Environment != "development" {
			return Config{}, fmt.Errorf("DEV_NODE_ENROLLMENT_TOKEN is only allowed when APP_ENV=development")
		}
		if len(cfg.DevNodeEnrollmentToken) < 32 {
			return Config{}, fmt.Errorf("DEV_NODE_ENROLLMENT_TOKEN must contain at least 32 characters when enabled")
		}
	}
	if strings.TrimSpace(cfg.DeliveryDir) == "" {
		return Config{}, fmt.Errorf("DELIVERY_DIR must not be empty")
	}
	if strings.TrimSpace(cfg.StagingDir) == "" {
		return Config{}, fmt.Errorf("STAGING_DIR must not be empty")
	}
	return cfg, nil
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
