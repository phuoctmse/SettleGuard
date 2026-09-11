package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/phuoctmse/settleguard/gateway/internal/api"
	"github.com/phuoctmse/settleguard/gateway/internal/auth"
	"github.com/phuoctmse/settleguard/gateway/internal/testutil"
)

// nextCapture records what the protected handler saw. values keeps every
// X-Client-Id value, not just the first, so the spoof test can prove the
// caller's copies were replaced rather than merely preceded.
type nextCapture struct {
	called       bool
	headerClient string
	values       []string
	ctxClient    uuid.UUID
	ctxOK        bool
}

func (n *nextCapture) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n.called = true
		n.headerClient = r.Header.Get(api.HeaderClientID)
		n.values = r.Header.Values(api.HeaderClientID)
		n.ctxClient, n.ctxOK = api.ClientIDFromContext(r.Context())
		w.WriteHeader(http.StatusNoContent)
	})
}

func errorBody(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body map[string]string
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	return body["error"]
}

func TestAPIKeyAuth_ValidKeyPassesAndSetsClientID(t *testing.T) {
	repo := auth.NewRepository(testutil.NewTestDB(t))
	clientID := uuid.New()
	_, raw, err := repo.Create(context.Background(), clientID, "t")
	require.NoError(t, err)

	next := &nextCapture{}
	h := api.APIKeyAuth(repo)(next.handler())
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer "+raw)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.True(t, next.called)
	assert.Equal(t, clientID.String(), next.headerClient)
	assert.True(t, next.ctxOK)
	assert.Equal(t, clientID, next.ctxClient)
}

// §5 of the spec: the header must be overwritten unconditionally. If the
// middleware only added it when absent, a caller could set X-Client-Id
// themselves and impersonate another client the moment a service trusts it.
func TestAPIKeyAuth_OverwritesClientSuppliedClientID(t *testing.T) {
	repo := auth.NewRepository(testutil.NewTestDB(t))
	clientID := uuid.New()
	_, raw, err := repo.Create(context.Background(), clientID, "t")
	require.NoError(t, err)

	next := &nextCapture{}
	h := api.APIKeyAuth(repo)(next.handler())
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer "+raw)
	req.Header.Set(api.HeaderClientID, uuid.New().String()) // spoof attempt
	req.Header.Add(api.HeaderClientID, uuid.New().String()) // and a second value
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	assert.Equal(t, []string{clientID.String()}, next.values, "exactly one value, and it is the key's client")
}

func TestAPIKeyAuth_MissingWrongAndRevokedAllGetTheSame401(t *testing.T) {
	repo := auth.NewRepository(testutil.NewTestDB(t))
	ctx := context.Background()
	revokedKey, revokedRaw, err := repo.Create(ctx, uuid.New(), "r")
	require.NoError(t, err)
	require.NoError(t, repo.Revoke(ctx, revokedKey.ID))

	cases := map[string]func(*http.Request){
		"missing header": func(r *http.Request) {},
		"not bearer":     func(r *http.Request) { r.Header.Set("Authorization", "Basic abc") },
		"unknown key":    func(r *http.Request) { r.Header.Set("Authorization", "Bearer sg_live_unknown") },
		"revoked key":    func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+revokedRaw) },
	}

	var bodies []string
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			next := &nextCapture{}
			h := api.APIKeyAuth(repo)(next.handler())
			req := httptest.NewRequest(http.MethodGet, "/x", nil)
			mutate(req)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusUnauthorized, rec.Code)
			assert.False(t, next.called, "protected handler must not run")
			bodies = append(bodies, errorBody(t, rec))
		})
	}
	for _, b := range bodies {
		assert.Equal(t, "unauthorized", b, "every failure mode must return the identical message")
	}
}
