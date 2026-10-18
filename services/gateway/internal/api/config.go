package api

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Config is everything the gateway reads from its environment. Defaults
// match the spec's §4.1 table exactly.
type Config struct {
	ListenAddr  string
	DatabaseURL string

	LedgerUpstream        string
	AccountsUpstream      string
	SettlementUpstream    string
	NotificationsUpstream string

	// CORSAllowedOrigins empty means no origin is allowed. Missing
	// configuration fails closed; it never opens the gateway to every site.
	CORSAllowedOrigins []string

	RateLimitIPPerMinute     int
	RateLimitClientPerMinute int
	UpstreamTimeout          time.Duration
}

// LoadConfig builds a Config from getenv (normally os.Getenv; tests pass a
// map lookup). DATABASE_URL is required; every other variable has the
// default the spec lists.
func LoadConfig(getenv func(string) string) (Config, error) {
	cfg := Config{
		ListenAddr:            valueOr(getenv("LISTEN_ADDR"), ":8000"),
		DatabaseURL:           getenv("DATABASE_URL"),
		LedgerUpstream:        valueOr(getenv("LEDGER_UPSTREAM"), "http://localhost:8080"),
		AccountsUpstream:      valueOr(getenv("ACCOUNTS_UPSTREAM"), "http://localhost:8081"),
		SettlementUpstream:    valueOr(getenv("SETTLEMENT_UPSTREAM"), "http://localhost:8082"),
		NotificationsUpstream: valueOr(getenv("NOTIFICATIONS_UPSTREAM"), "http://localhost:8083"),
	}
	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("config: DATABASE_URL is required")
	}

	for _, o := range strings.Split(getenv("CORS_ALLOWED_ORIGINS"), ",") {
		o = strings.TrimSpace(o)
		if o == "" {
			continue
		}
		if o == "*" {
			return Config{}, errors.New("config: CORS_ALLOWED_ORIGINS must list origins explicitly; \"*\" is not allowed with credentials")
		}
		cfg.CORSAllowedOrigins = append(cfg.CORSAllowedOrigins, o)
	}

	var err error
	if cfg.RateLimitIPPerMinute, err = intOr(getenv("RATE_LIMIT_IP_PER_MINUTE"), 60); err != nil {
		return Config{}, err
	}
	if cfg.RateLimitClientPerMinute, err = intOr(getenv("RATE_LIMIT_CLIENT_PER_MINUTE"), 600); err != nil {
		return Config{}, err
	}
	seconds, err := intOr(getenv("UPSTREAM_TIMEOUT_SECONDS"), 30)
	if err != nil {
		return Config{}, err
	}
	cfg.UpstreamTimeout = time.Duration(seconds) * time.Second

	return cfg, nil
}

func valueOr(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func intOr(v string, def int) (int, error) {
	if v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("config: %q is not a positive integer", v)
	}
	return n, nil
}
