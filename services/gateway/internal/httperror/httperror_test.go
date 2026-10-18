package httperror_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/phuoctmse/settleguard/gateway/internal/httperror"
)

func TestWrite_SendsStatusContentTypeAndErrorBody(t *testing.T) {
	rec := httptest.NewRecorder()
	httperror.Write(rec, http.StatusTeapot, "short and stout")

	assert.Equal(t, http.StatusTeapot, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	assert.JSONEq(t, `{"error":"short and stout"}`, rec.Body.String())
}
