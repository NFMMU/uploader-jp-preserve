package config

import (
	"encoding/hex"
	"fmt"
	"net/mail"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Environment                      string
	Address                          string
	DatabaseURL                      string
	JWTSecret                        []byte
	JWTTTL                           time.Duration
	BootstrapAdminEmail              string
	BootstrapAdminPassword           string
	DevNodeEnrollmentToken           string
	TrustedProxies                   []string
	StagingDir                       string
	DeliveryDir                      string
	SourcePasswordEncryptionKey      []byte
	LibreDeskBaseURL                 string
	LibreDeskAPIKey                  string
	LibreDeskAPISecret               string
	LibreDeskInboxID                 int64
	LibreDeskTeamID                  int64
	LibreDeskAgentID                 int64
	LibreDeskFallbackContactEmail    string
	LibreDeskConversationURLTemplate string
	TurnstileSiteKey                 string
	TurnstileSecretKey               string
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
		TrustedProxies:                   trustedProxies,
		Environment:                      env("APP_ENV", "production"),
		Address:                          env("APP_ADDR", ":8080"),
		DatabaseURL:                      strings.TrimSpace(os.Getenv("DATABASE_URL")),
		JWTSecret:                        []byte(strings.TrimSpace(os.Getenv("JWT_SECRET"))),
		BootstrapAdminEmail:              strings.ToLower(strings.TrimSpace(os.Getenv("BOOTSTRAP_ADMIN_EMAIL"))),
		BootstrapAdminPassword:           os.Getenv("BOOTSTRAP_ADMIN_PASSWORD"),
		DevNodeEnrollmentToken:           strings.TrimSpace(os.Getenv("DEV_NODE_ENROLLMENT_TOKEN")),
		StagingDir:                       env("STAGING_DIR", "./data/staging"),
		DeliveryDir:                      env("DELIVERY_DIR", "./data/delivery"),
		LibreDeskBaseURL:                 strings.TrimRight(strings.TrimSpace(os.Getenv("LIBREDESK_BASE_URL")), "/"),
		LibreDeskAPIKey:                  strings.TrimSpace(os.Getenv("LIBREDESK_API_KEY")),
		LibreDeskAPISecret:               strings.TrimSpace(os.Getenv("LIBREDESK_API_SECRET")),
		LibreDeskFallbackContactEmail:    strings.TrimSpace(env("LIBREDESK_FALLBACK_CONTACT_EMAIL", "uploaderjp-archive@udonya.moe")),
		LibreDeskConversationURLTemplate: env("LIBREDESK_CONVERSATION_URL_TEMPLATE", "{{base_url}}/app/inbox/conversations/{{uuid}}"),
		TurnstileSiteKey:                 strings.TrimSpace(os.Getenv("TURNSTILE_SITE_KEY")),
		TurnstileSecretKey:               strings.TrimSpace(os.Getenv("TURNSTILE_SECRET_KEY")),
	}
	if rawKey := strings.TrimSpace(os.Getenv("SOURCE_PASSWORD_ENCRYPTION_KEY")); rawKey != "" {
		key, err := hex.DecodeString(rawKey)
		if err != nil || len(key) != 32 {
			return Config{}, fmt.Errorf("SOURCE_PASSWORD_ENCRYPTION_KEY must be 64 hex characters (32 bytes)")
		}
		cfg.SourcePasswordEncryptionKey = key
	}
	libreDeskConfigured := cfg.LibreDeskBaseURL != "" || cfg.LibreDeskAPIKey != "" || cfg.LibreDeskAPISecret != "" || strings.TrimSpace(os.Getenv("LIBREDESK_INBOX_ID")) != ""
	if libreDeskConfigured {
		if cfg.LibreDeskBaseURL == "" || cfg.LibreDeskAPIKey == "" || cfg.LibreDeskAPISecret == "" {
			return Config{}, fmt.Errorf("LIBREDESK_BASE_URL, LIBREDESK_API_KEY, and LIBREDESK_API_SECRET must all be set to enable LibreDesk")
		}
		u, err := url.Parse(cfg.LibreDeskBaseURL)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Hostname() == "" || u.User != nil {
			return Config{}, fmt.Errorf("LIBREDESK_BASE_URL must be an http(s) URL without user information")
		}
		inboxID, err := strconv.ParseInt(strings.TrimSpace(os.Getenv("LIBREDESK_INBOX_ID")), 10, 64)
		if err != nil || inboxID <= 0 {
			return Config{}, fmt.Errorf("LIBREDESK_INBOX_ID must be a positive integer when LibreDesk is enabled")
		}
		cfg.LibreDeskInboxID = inboxID
		if raw := strings.TrimSpace(os.Getenv("LIBREDESK_TEAM_ID")); raw != "" {
			cfg.LibreDeskTeamID, err = strconv.ParseInt(raw, 10, 64)
			if err != nil || cfg.LibreDeskTeamID <= 0 {
				return Config{}, fmt.Errorf("LIBREDESK_TEAM_ID must be a positive integer")
			}
		}
		if raw := strings.TrimSpace(os.Getenv("LIBREDESK_AGENT_ID")); raw != "" {
			cfg.LibreDeskAgentID, err = strconv.ParseInt(raw, 10, 64)
			if err != nil || cfg.LibreDeskAgentID <= 0 {
				return Config{}, fmt.Errorf("LIBREDESK_AGENT_ID must be a positive integer")
			}
		}
		fallbackAddress, err := mail.ParseAddress(cfg.LibreDeskFallbackContactEmail)
		if err != nil {
			return Config{}, fmt.Errorf("LIBREDESK_FALLBACK_CONTACT_EMAIL must be a valid email address")
		}
		cfg.LibreDeskFallbackContactEmail = fallbackAddress.Address
		testConversationURL := strings.ReplaceAll(cfg.LibreDeskConversationURLTemplate, "{{base_url}}", cfg.LibreDeskBaseURL)
		testConversationURL = strings.ReplaceAll(testConversationURL, "{{uuid}}", "00000000-0000-4000-8000-000000000000")
		conversationURL, err := url.Parse(testConversationURL)
		if err != nil || (conversationURL.Scheme != "https" && conversationURL.Scheme != "http") || conversationURL.Hostname() == "" || conversationURL.User != nil {
			return Config{}, fmt.Errorf("LIBREDESK_CONVERSATION_URL_TEMPLATE must resolve to a valid http(s) URL")
		}
	}
	if (cfg.TurnstileSiteKey == "") != (cfg.TurnstileSecretKey == "") {
		return Config{}, fmt.Errorf("TURNSTILE_SITE_KEY and TURNSTILE_SECRET_KEY must both be set or both be empty")
	}
	if cfg.Environment == "production" && (cfg.TurnstileSiteKey == "" || cfg.TurnstileSecretKey == "") {
		return Config{}, fmt.Errorf("TURNSTILE_SITE_KEY and TURNSTILE_SECRET_KEY are required when APP_ENV=production")
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
