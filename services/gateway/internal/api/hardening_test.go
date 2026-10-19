package api_test

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/phuoctmse/settleguard/gateway/internal/api"
)

// SG-PT-09: the headers ZAP flagged as missing on every spec ([10021],
// [90004]) are set on every gateway response, including its own errors.
func TestSecurityHeaders_SetOnEveryResponse(t *testing.T) {
	h := api.SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ledger/transactions", nil))

	assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "same-origin", rec.Header().Get("Cross-Origin-Resource-Policy"))
}

// SG-PT-08: a declared Content-Length over the limit is refused up front,
// before anything reads the body or reaches an upstream.
func TestLimitRequestBody_DeclaredOversizedBodyIs413(t *testing.T) {
	called := false
	h := api.LimitRequestBody(10)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/accounts/clients", strings.NewReader(strings.Repeat("x", 11))))

	assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
	assert.JSONEq(t, `{"error":"request body too large"}`, rec.Body.String())
	assert.False(t, called, "an oversized request must not reach the next handler")
}

// A chunked body declares no length, so the cap has to hold while reading.
func TestLimitRequestBody_UndeclaredOversizedBodyStopsAtTheLimit(t *testing.T) {
	var readErr error
	h := api.LimitRequestBody(10)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, readErr = io.ReadAll(r.Body)
	}))

	req := httptest.NewRequest(http.MethodPost, "/accounts/clients", strings.NewReader(strings.Repeat("x", 11)))
	req.ContentLength = -1
	h.ServeHTTP(httptest.NewRecorder(), req)

	var tooLarge *http.MaxBytesError
	require.True(t, errors.As(readErr, &tooLarge), "want *http.MaxBytesError, got %v", readErr)
}

func TestLimitRequestBody_BodyAtTheLimitPasses(t *testing.T) {
	var got []byte
	h := api.LimitRequestBody(10)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/accounts/clients", strings.NewReader(strings.Repeat("x", 10))))

	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.Len(t, got, 10)
}
