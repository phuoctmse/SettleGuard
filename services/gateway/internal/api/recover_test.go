package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/phuoctmse/settleguard/gateway/internal/api"
)

// A panic must answer with the gateway's standard JSON error body (spec
// §8), not chi's bare empty 500, so one client-side parser covers every
// gateway-originated error.
func TestRecoverJSON_TurnsAPanicIntoTheStandardErrorBody(t *testing.T) {
	h := api.RecoverJSON(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ledger/transactions", nil))

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.JSONEq(t, `{"error":"internal error"}`, rec.Body.String())
}
