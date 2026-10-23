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
}

func Load() (Config, error) {
	cfg := Config{
		Environment:            env("APP_ENV", "development"),
		Address:                env("APP_ADDR", ":8080"),
		DatabaseURL:            strings.TrimSpace(os.Getenv("DATABASE_URL")),
		JWTSecret:              []byte(strings.TrimSpace(os.Getenv("JWT_SECRET"))),
		BootstrapAdminEmail:    strings.ToLower(strings.TrimSpace(os.Getenv("BOOTSTRAP_ADMIN_EMAIL"))),
		BootstrapAdminPassword: os.Getenv("BOOTSTRAP_ADMIN_PASSWORD"),
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
	return cfg, nil
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
