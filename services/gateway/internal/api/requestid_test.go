package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/phuoctmse/settleguard/gateway/internal/api"
)

func TestRequestID_GeneratesWhenAbsent(t *testing.T) {
	var seenByNext string
	h := api.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenByNext = r.Header.Get(api.HeaderRequestID)
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	_, err := uuid.Parse(seenByNext)
	require.NoError(t, err, "next handler must see a generated UUID")
	assert.Equal(t, seenByNext, rec.Header().Get(api.HeaderRequestID), "the same id must be echoed on the response")
}

func TestRequestID_PreservesClientSuppliedID(t *testing.T) {
	var seenByNext string
	h := api.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenByNext = r.Header.Get(api.HeaderRequestID)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(api.HeaderRequestID, "client-trace-123")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	assert.Equal(t, "client-trace-123", seenByNext)
	assert.Equal(t, "client-trace-123", rec.Header().Get(api.HeaderRequestID))
}
