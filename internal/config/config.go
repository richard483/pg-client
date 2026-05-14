package config

import (
	"bufio"
	"errors"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port                string
	KarasuIntrospectURL string
	RequiredAudience    string
	RequiredScope       string
	PostgresHost        string
	PostgresPort        string
	PostgresUser        string
	PostgresPassword    string
	PostgresSSLMode     string
	QueryTimeout        time.Duration
	MaxRows             int
	MaxResponseBytes    int
}

func Load() (Config, error) {
	loadDotEnv(".env")

	cfg := Config{
		Port:                env("PORT", "8090"),
		KarasuIntrospectURL: env("KARASU_INTROSPECT_URL", "http://127.0.0.1:8080/oauth/introspect"),
		RequiredAudience:    env("REQUIRED_AUDIENCE", "pg-client-api"),
		RequiredScope:       env("REQUIRED_SCOPE", "pg-client:execute"),
		PostgresHost:        env("POSTGRES_HOST", "127.0.0.1"),
		PostgresPort:        env("POSTGRES_PORT", "5432"),
		PostgresUser:        env("POSTGRES_USER", "postgres"),
		PostgresPassword:    env("POSTGRES_PASSWORD", ""),
		PostgresSSLMode:     env("POSTGRES_SSLMODE", "disable"),
		QueryTimeout:        time.Duration(envInt("QUERY_TIMEOUT_SECONDS", 30)) * time.Second,
		MaxRows:             envInt("MAX_ROWS", 1000),
		MaxResponseBytes:    envInt("MAX_RESPONSE_BYTES", 10*1024*1024),
	}

	if cfg.PostgresUser == "" {
		return Config{}, errors.New("POSTGRES_USER is required")
	}
	if cfg.KarasuIntrospectURL == "" {
		return Config{}, errors.New("KARASU_INTROSPECT_URL is required")
	}
	if cfg.MaxRows <= 0 {
		return Config{}, errors.New("MAX_ROWS must be greater than zero")
	}
	if cfg.MaxResponseBytes <= 0 {
		return Config{}, errors.New("MAX_RESPONSE_BYTES must be greater than zero")
	}
	if cfg.QueryTimeout <= 0 {
		return Config{}, errors.New("QUERY_TIMEOUT_SECONDS must be greater than zero")
	}

	return cfg, nil
}

func loadDotEnv(path string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		_ = os.Setenv(key, value)
	}
}

func (c Config) PostgresDSN(database string) string {
	u := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(c.PostgresUser, c.PostgresPassword),
		Host:   c.PostgresHost + ":" + c.PostgresPort,
		Path:   database,
	}
	q := u.Query()
	q.Set("sslmode", c.PostgresSSLMode)
	u.RawQuery = q.Encode()
	return u.String()
}

func env(key, fallback string) string {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	return value
}

func envInt(key string, fallback int) int {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
