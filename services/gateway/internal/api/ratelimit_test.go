package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/phuoctmse/settleguard/gateway/internal/api"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
}

func TestRateLimiter_AllowsBurstThenRefuses(t *testing.T) {
	l := api.NewRateLimiter(3)
	for i := 0; i < 3; i++ {
		assert.True(t, l.Allow("k"), "request %d within the limit", i+1)
	}
	assert.False(t, l.Allow("k"), "the 4th request in the same minute is refused")
	assert.True(t, l.Allow("other"), "a different key has its own budget")
}

func TestRateLimitByIP_Returns429PastTheLimit(t *testing.T) {
	h := api.RateLimitByIP(api.NewRateLimiter(2))(okHandler())
	do := func(remote string) int {
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.RemoteAddr = remote
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}

	assert.Equal(t, http.StatusNoContent, do("10.0.0.1:1111"))
	assert.Equal(t, http.StatusNoContent, do("10.0.0.1:2222"), "same IP, different source port, same bucket")
	assert.Equal(t, http.StatusTooManyRequests, do("10.0.0.1:3333"))
	assert.Equal(t, http.StatusNoContent, do("10.0.0.2:1111"), "another IP is unaffected")
}

func TestRateLimitByClient_KeysOnAuthenticatedClient(t *testing.T) {
	h := api.RateLimitByClient(api.NewRateLimiter(1))(okHandler())
	a, b := uuid.New(), uuid.New()
	do := func(client uuid.UUID) int {
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req = req.WithContext(api.ContextWithClientID(context.Background(), client))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}

	assert.Equal(t, http.StatusNoContent, do(a))
	assert.Equal(t, http.StatusTooManyRequests, do(a))
	assert.Equal(t, http.StatusNoContent, do(b))
}

func TestRateLimitByClient_WithoutClientIsAnInternalError(t *testing.T) {
	// This middleware is only ever mounted after APIKeyAuth. Reaching it
	// without a client id means the chain was assembled wrong; fail loudly
	// rather than silently sharing one bucket for everyone.
	h := api.RateLimitByClient(api.NewRateLimiter(10))(okHandler())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	require.Equal(t, http.StatusInternalServerError, rec.Code)
}
