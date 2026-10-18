package api_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/phuoctmse/settleguard/gateway/internal/api"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLoadConfig_AppliesSpecDefaults(t *testing.T) {
	cfg, err := api.LoadConfig(env(map[string]string{"DATABASE_URL": "postgres://x"}))
	require.NoError(t, err)

	assert.Equal(t, ":8000", cfg.ListenAddr)
	assert.Equal(t, "http://localhost:8080", cfg.LedgerUpstream)
	assert.Equal(t, "http://localhost:8081", cfg.AccountsUpstream)
	assert.Equal(t, "http://localhost:8082", cfg.SettlementUpstream)
	assert.Equal(t, "http://localhost:8083", cfg.NotificationsUpstream)
	assert.Empty(t, cfg.CORSAllowedOrigins, "empty means block every origin, not allow all")
	assert.Equal(t, 60, cfg.RateLimitIPPerMinute)
	assert.Equal(t, 600, cfg.RateLimitClientPerMinute)
	assert.Equal(t, 30*time.Second, cfg.UpstreamTimeout)
}

func TestLoadConfig_RequiresDatabaseURL(t *testing.T) {
	_, err := api.LoadConfig(env(map[string]string{}))
	assert.Error(t, err)
}

func TestLoadConfig_ParsesOriginsAndNumbers(t *testing.T) {
	cfg, err := api.LoadConfig(env(map[string]string{
		"DATABASE_URL":                 "postgres://x",
		"CORS_ALLOWED_ORIGINS":         "https://app.example.com, http://localhost:8090",
		"RATE_LIMIT_IP_PER_MINUTE":     "5",
		"RATE_LIMIT_CLIENT_PER_MINUTE": "50",
		"UPSTREAM_TIMEOUT_SECONDS":     "3",
	}))
	require.NoError(t, err)
	assert.Equal(t, []string{"https://app.example.com", "http://localhost:8090"}, cfg.CORSAllowedOrigins)
	assert.Equal(t, 5, cfg.RateLimitIPPerMinute)
	assert.Equal(t, 50, cfg.RateLimitClientPerMinute)
	assert.Equal(t, 3*time.Second, cfg.UpstreamTimeout)
}

func TestLoadConfig_RejectsUnparseableNumber(t *testing.T) {
	_, err := api.LoadConfig(env(map[string]string{"DATABASE_URL": "postgres://x", "RATE_LIMIT_IP_PER_MINUTE": "many"}))
	assert.Error(t, err)
}

func TestLoadConfig_RejectsWildcardOrigin(t *testing.T) {
	// Requests carry credentials; "*" is never acceptable (spec §4).
	_, err := api.LoadConfig(env(map[string]string{"DATABASE_URL": "postgres://x", "CORS_ALLOWED_ORIGINS": "*"}))
	assert.Error(t, err)
}
