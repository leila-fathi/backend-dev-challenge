package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port               string
	JWTSigningKey      string
	JWTClaimsNamespace string
	AccessTokenTTL     time.Duration
	HasuraGraphQLURL   string
	HasuraAdminSecret  string
	CORSAllowOrigins   []string
	UploadDir          string
	ThumbnailMaxSize   int

	//DB and Session
	DatabaseURL string
	SessionTTL  time.Duration

	// Zitadel
	ZitadelDomain                string
	ZitadelBrowserClientID       string
	ZitadelServiceAccountKeyPath string
}

func Load() (Config, error) {
	cfg := Config{
		Port:               getEnv("PORT", "8081"),
		JWTSigningKey:      os.Getenv("JWT_SIGNING_KEY"),
		JWTClaimsNamespace: getEnv("JWT_CLAIMS_NAMESPACE", "https://challenge.example/jwt/claims"),
		HasuraGraphQLURL:   getEnv("HASURA_GRAPHQL_URL", "http://localhost:8080/v1/graphql"),
		HasuraAdminSecret:  getEnv("HASURA_ADMIN_SECRET", "hasura-admin-secret"),
		CORSAllowOrigins:   splitCSV(getEnv("CORS_ALLOW_ORIGINS", "http://localhost:5173")),
		UploadDir:          getEnv("UPLOAD_DIR", "./tmp/uploads"),
		ThumbnailMaxSize:   getIntEnv("THUMBNAIL_MAX_SIZE", 256),

		//Database
		DatabaseURL: getEnv("DATABASE_URL", "postgres://challenge:challenge@localhost:5433/challenge?sslmode=disable"),

		// Zitadel
		ZitadelDomain:                getEnv("ZITADEL_DOMAIN", ""),
		ZitadelBrowserClientID:       getEnv("ZITADEL_BROWSER_CLIENT_ID", ""),
		ZitadelServiceAccountKeyPath: getEnv("ZITADEL_SERVICE_ACCOUNT_KEY_PATH", ""),
	}

	if cfg.JWTSigningKey == "" {
		return Config{}, errors.New("JWT_SIGNING_KEY is required")
	}

	accessTTL, err := time.ParseDuration(getEnv("ACCESS_TOKEN_TTL", "15m"))
	if err != nil {
		return Config{}, fmt.Errorf("invalid ACCESS_TOKEN_TTL: %w", err)
	}
	if accessTTL <= 0 {
		return Config{}, errors.New("ACCESS_TOKEN_TTL must be >0")
	}
	cfg.AccessTokenTTL = accessTTL

	// Validate SessionTTL
	sessionTTL, err := time.ParseDuration(getEnv("API_SESSION_TTL", "24h"))
	if err != nil {
		return Config{}, fmt.Errorf("invalid API_SESSION_TTL: %w", err)
	}
	if sessionTTL <= 0 {
		return Config{}, errors.New("API_SESSION_TTL must be >0")
	}
	cfg.SessionTTL = sessionTTL

	// Validate Zitadel config (required for new public API login).
	if cfg.ZitadelDomain == "" {
		return Config{}, errors.New("ZITADEL_DOMAIN is required")
	}
	if cfg.ZitadelBrowserClientID == "" {
		return Config{}, errors.New("ZITADEL_BROWSER_CLIENT_ID is required")
	}
	if cfg.ZitadelServiceAccountKeyPath == "" {
		return Config{}, errors.New("ZITADEL_SERVICE_ACCOUNT_KEY_PATH is required")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getIntEnv(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func splitCSV(value string) []string {
	if value == "" {
		return nil
	}
	raw := strings.Split(value, ",")
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		trimmed := strings.TrimSpace(item)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
