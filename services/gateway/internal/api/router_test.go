package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/phuoctmse/settleguard/gateway/internal/api"
	"github.com/phuoctmse/settleguard/gateway/internal/auth"
	"github.com/phuoctmse/settleguard/gateway/internal/testutil"
)

// pathEcho answers every request with what it received: its own name, so a
// test can prove which upstream the prefix actually reached; the path, so a
// test can prove the prefix was stripped; and the three headers the gateway
// controls at the trust boundary.
func pathEcho(t *testing.T, name string) *httptest.Server {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"upstream":      name,
			"path":          r.URL.Path,
			"client":        r.Header.Get(api.HeaderClientID),
			"authorization": r.Header.Get("Authorization"),
			"request_id":    r.Header.Get(api.HeaderRequestID),
		})
	}))
	t.Cleanup(s.Close)
	return s
}

type routerFixture struct {
	handler http.Handler
	rawKey  string
	client  uuid.UUID
	ledger  *httptest.Server
}

func newRouterFixture(t *testing.T, origins []string) routerFixture {
	t.Helper()
	repo := auth.NewRepository(testutil.NewTestDB(t))
	client := uuid.New()
	_, raw, err := repo.Create(context.Background(), client, "t")
	require.NoError(t, err)

	ledger := pathEcho(t, "ledger")
	cfg := api.Config{
		ListenAddr:               ":0",
		LedgerUpstream:           ledger.URL,
		AccountsUpstream:         pathEcho(t, "accounts").URL,
		SettlementUpstream:       pathEcho(t, "settlement").URL,
		NotificationsUpstream:    pathEcho(t, "notifications").URL,
		CORSAllowedOrigins:       origins,
		RateLimitIPPerMinute:     1000,
		RateLimitClientPerMinute: 1000,
		UpstreamTimeout:          time.Second,
	}
	h, err := api.NewRouter(cfg, repo)
	require.NoError(t, err)
	return routerFixture{handler: h, rawKey: raw, client: client, ledger: ledger}
}

func (f routerFixture) do(method, path string, mutate func(*http.Request)) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if mutate != nil {
		mutate(req)
	}
	rec := httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)
	return rec
}

func withKey(raw string) func(*http.Request) {
	return func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+raw) }
}

func TestRouter_HealthNeedsNoKey(t *testing.T) {
	f := newRouterFixture(t, nil)
	rec := f.do(http.MethodGet, "/health", nil)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"status":"ok"}`, rec.Body.String())
}

// Every prefix is checked against its own upstream, not just one: with a
// single route verified, a transposed prefix-to-upstream pair would send a
// tenant's requests to the wrong service and still pass.
func TestRouter_RoutesByPrefixAndStripsIt(t *testing.T) {
	f := newRouterFixture(t, nil)
	cases := []struct {
		request  string
		wantPath string
		upstream string
	}{
		{"/ledger/transactions", "/transactions", "ledger"},
		{"/accounts/clients", "/clients", "accounts"},
		{"/settlement/settlements", "/settlements", "settlement"},
		{"/notifications/", "/", "notifications"},
	}
	for _, tc := range cases {
		t.Run(tc.request, func(t *testing.T) {
			rec := f.do(http.MethodGet, tc.request, withKey(f.rawKey))
			require.Equal(t, http.StatusOK, rec.Code)

			var got map[string]string
			require.NoError(t, json.NewDecoder(rec.Body).Decode(&got))
			assert.Equal(t, tc.upstream, got["upstream"], "prefix must reach its own upstream")
			assert.Equal(t, tc.wantPath, got["path"], "prefix must be stripped before forwarding")
			assert.Equal(t, f.client.String(), got["client"], "X-Client-Id must reach the upstream")
		})
	}
}

// The gateway is the trust boundary: a caller cannot erase the gateway's
// X-Client-Id via Connection (ReverseProxy strips headers named there
// after the director runs), and the bearer key must not travel upstream.
func TestRouter_TrustBoundaryHeadersReachUpstreamCorrectly(t *testing.T) {
	f := newRouterFixture(t, nil)
	rec := f.do(http.MethodGet, "/ledger/transactions", func(r *http.Request) {
		r.Header.Set("Authorization", "Bearer "+f.rawKey)
		r.Header.Set(api.HeaderClientID, uuid.New().String())                   // spoof attempt
		r.Header.Set("Connection", api.HeaderClientID+", "+api.HeaderRequestID) // erasure attempt
		r.Header.Set(api.HeaderRequestID, "trace-42")
	})
	require.Equal(t, http.StatusOK, rec.Code)
	var got map[string]string
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&got))
	assert.Equal(t, f.client.String(), got["client"], "X-Client-Id must be the authenticated client, not erased or spoofed")
	assert.Equal(t, "trace-42", got["request_id"], "X-Request-Id must survive a Connection-header erasure attempt")
	assert.Empty(t, got["authorization"], "the raw API key must never reach an upstream")
}

func TestRouter_ProtectedPrefixWithoutKeyIs401(t *testing.T) {
	f := newRouterFixture(t, nil)
	for _, p := range []string{"/ledger/transactions", "/accounts/clients", "/settlement/transactions", "/notifications/"} {
		rec := f.do(http.MethodGet, p, nil)
		assert.Equal(t, http.StatusUnauthorized, rec.Code, p)
	}
}

func TestRouter_UnknownPrefixIs404JSON(t *testing.T) {
	f := newRouterFixture(t, nil)
	rec := f.do(http.MethodGet, "/nope/x", withKey(f.rawKey))
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.JSONEq(t, `{"error":"not found"}`, rec.Body.String())
}

func TestRouter_PreflightPassesWithoutKeyForAllowedOrigin(t *testing.T) {
	f := newRouterFixture(t, []string{"http://localhost:8090"})
	rec := f.do(http.MethodOptions, "/ledger/transactions", func(r *http.Request) {
		r.Header.Set("Origin", "http://localhost:8090")
		r.Header.Set("Access-Control-Request-Method", "POST")
		r.Header.Set("Access-Control-Request-Headers", "Authorization, Content-Type")
	})
	// go-chi/cors always answers a handled (non-passthrough) preflight with
	// 200, never 204 -- verified against every published version (v1.0.0
	// through the @latest-resolved v1.2.2): cors.go's Handler
	// unconditionally calls w.WriteHeader(http.StatusOK) when
	// OptionsPassthrough is false. The brief's troubleshooting note assumed
	// 204 was the library's default and attributed a 200 result to
	// accidentally setting OptionsPassthrough: true; router.go never sets
	// that field, so this is the library's real behaviour, not a wiring bug.
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "http://localhost:8090", rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "true", rec.Header().Get("Access-Control-Allow-Credentials"))
}

func TestRouter_DisallowedOriginGetsNoCORSHeaders(t *testing.T) {
	f := newRouterFixture(t, []string{"http://localhost:8090"})
	rec := f.do(http.MethodOptions, "/ledger/transactions", func(r *http.Request) {
		r.Header.Set("Origin", "https://evil.example")
		r.Header.Set("Access-Control-Request-Method", "POST")
	})
	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestRouter_EmptyOriginListBlocksEveryOrigin(t *testing.T) {
	f := newRouterFixture(t, nil)
	rec := f.do(http.MethodOptions, "/ledger/transactions", func(r *http.Request) {
		r.Header.Set("Origin", "http://localhost:8090")
		r.Header.Set("Access-Control-Request-Method", "GET")
	})
	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"), "no allowlist means fail-closed")
}
