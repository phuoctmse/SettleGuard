package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/phuoctmse/settleguard/accounts-service/internal/account"
	"github.com/phuoctmse/settleguard/accounts-service/internal/api"
	"github.com/phuoctmse/settleguard/accounts-service/internal/testutil"
)

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	conn := testutil.NewTestDB(t)
	clients := account.NewClientRepository(conn)
	accounts := account.NewAccountRepository(conn)
	handlers := api.NewHandlers(clients, accounts)
	server := httptest.NewServer(api.NewRouter(handlers))
	t.Cleanup(server.Close)
	return server
}

func TestCreateClient(t *testing.T) {
	server := newTestServer(t)

	body, err := json.Marshal(map[string]any{"name": "Acme Corp"})
	require.NoError(t, err)

	resp, err := http.Post(server.URL+"/clients", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var created map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&created))
	assert.Equal(t, "Acme Corp", created["name"])
	assert.Equal(t, "active", created["status"])
}

func TestCreateClient_RejectsEmptyName(t *testing.T) {
	server := newTestServer(t)

	body, err := json.Marshal(map[string]any{"name": ""})
	require.NoError(t, err)

	resp, err := http.Post(server.URL+"/clients", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestGetClient(t *testing.T) {
	server := newTestServer(t)
	client := createClientFixture(t, server, "Acme Corp")

	resp, err := http.Get(server.URL + "/clients/" + client["id"].(string))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var fetched map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&fetched))
	assert.Equal(t, client["id"], fetched["id"])
	assert.Equal(t, "Acme Corp", fetched["name"])
	assert.Equal(t, "active", fetched["status"])
}

func TestGetClient_NotFound(t *testing.T) {
	server := newTestServer(t)

	resp, err := http.Get(server.URL + "/clients/00000000-0000-0000-0000-000000000000")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestUpdateClientStatus(t *testing.T) {
	server := newTestServer(t)

	createBody, err := json.Marshal(map[string]any{"name": "Acme Corp"})
	require.NoError(t, err)
	createResp, err := http.Post(server.URL+"/clients", "application/json", bytes.NewReader(createBody))
	require.NoError(t, err)
	defer createResp.Body.Close()
	var created map[string]any
	require.NoError(t, json.NewDecoder(createResp.Body).Decode(&created))

	statusBody, err := json.Marshal(map[string]any{"status": "suspended"})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPatch, server.URL+"/clients/"+created["id"].(string)+"/status", bytes.NewReader(statusBody))
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var updated map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&updated))
	assert.Equal(t, "suspended", updated["status"])
}

func TestUpdateClientStatus_RejectsInvalid(t *testing.T) {
	server := newTestServer(t)

	createBody, err := json.Marshal(map[string]any{"name": "Acme Corp"})
	require.NoError(t, err)
	createResp, err := http.Post(server.URL+"/clients", "application/json", bytes.NewReader(createBody))
	require.NoError(t, err)
	defer createResp.Body.Close()
	var created map[string]any
	require.NoError(t, json.NewDecoder(createResp.Body).Decode(&created))

	statusBody, err := json.Marshal(map[string]any{"status": "bogus"})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPatch, server.URL+"/clients/"+created["id"].(string)+"/status", bytes.NewReader(statusBody))
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}
