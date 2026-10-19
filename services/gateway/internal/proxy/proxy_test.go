package proxy_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/phuoctmse/settleguard/gateway/internal/proxy"
)

type echo struct {
	Method string `json:"method"`
	Path   string `json:"path"`
	Query  string `json:"query"`
	Body   string `json:"body"`
	ReqID  string `json:"req_id"`
}

func echoUpstream(t *testing.T, status int) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(echo{
			Method: r.Method, Path: r.URL.Path, Query: r.URL.RawQuery, Body: string(b),
			ReqID: r.Header.Get("X-Request-Id"),
		})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func mustURL(t *testing.T, s string) *url.URL {
	t.Helper()
	u, err := url.Parse(s)
	require.NoError(t, err)
	return u
}

func TestProxy_PreservesMethodPathQueryBodyAndStatus(t *testing.T) {
	up := echoUpstream(t, http.StatusCreated)
	h := proxy.New(mustURL(t, up.URL), time.Second)

	req := httptest.NewRequest(http.MethodPost, "/transactions?limit=5", strings.NewReader(`{"a":1}`))
	req.Header.Set("X-Request-Id", "rid-1")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
	var got echo
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&got))
	assert.Equal(t, echo{Method: "POST", Path: "/transactions", Query: "limit=5", Body: `{"a":1}`, ReqID: "rid-1"}, got)
}

func TestProxy_UnreachableUpstreamIs502(t *testing.T) {
	// A closed port: nothing is listening.
	dead := httptest.NewServer(http.NotFoundHandler())
	deadURL := dead.URL
	dead.Close()
	h := proxy.New(mustURL(t, deadURL), time.Second)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))

	assert.Equal(t, http.StatusBadGateway, rec.Code)
	var body map[string]string
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Equal(t, "upstream unavailable", body["error"])
}

// SG-PT-09: an upstream's Server header (notification-service sends its
// Python version) must not reach the caller.
func TestProxy_StripsUpstreamServerHeader(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Server", "BaseHTTP/0.6 Python/3.12.0")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(up.Close)
	h := proxy.New(mustURL(t, up.URL), time.Second)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/notifications", nil))

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, rec.Header().Get("Server"))
}

// SG-PT-08: a body that passes the up-front Content-Length check but then
// overruns http.MaxBytesReader mid-stream is a 413, not an upstream 502.
func TestProxy_BodyOverMaxBytesIs413(t *testing.T) {
	up := echoUpstream(t, http.StatusCreated)
	h := proxy.New(mustURL(t, up.URL), time.Second)

	req := httptest.NewRequest(http.MethodPost, "/clients", strings.NewReader(strings.Repeat("x", 64)))
	req.ContentLength = -1
	rec := httptest.NewRecorder()
	req.Body = http.MaxBytesReader(rec, req.Body, 10)
	h.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
	var body map[string]string
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Equal(t, "request body too large", body["error"])
}

func TestProxy_SlowUpstreamIs504(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(400 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(slow.Close)
	h := proxy.New(mustURL(t, slow.URL), 100*time.Millisecond)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))

	assert.Equal(t, http.StatusGatewayTimeout, rec.Code)
	var body map[string]string
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Equal(t, "upstream timeout", body["error"])
}
