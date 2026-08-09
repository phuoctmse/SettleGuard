package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createClientFixture(t *testing.T, server *httptest.Server, name string) map[string]any {
	t.Helper()
	body, err := json.Marshal(map[string]any{"name": name})
	require.NoError(t, err)
	resp, err := http.Post(server.URL+"/clients", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var created map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&created))
	return created
}

func TestCreateAccount(t *testing.T) {
	server := newTestServer(t)
	client := createClientFixture(t, server, "Acme Corp")

	body, err := json.Marshal(map[string]any{"client_id": client["id"], "external_ref": "ext-1"})
	require.NoError(t, err)
	resp, err := http.Post(server.URL+"/accounts", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var created map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&created))
	assert.Equal(t, client["id"], created["client_id"])
	assert.Equal(t, "active", created["status"])
}

func TestCreateAccount_RejectsSuspendedClient(t *testing.T) {
	server := newTestServer(t)
	client := createClientFixture(t, server, "Acme Corp")

	statusBody, err := json.Marshal(map[string]any{"status": "suspended"})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPatch, server.URL+"/clients/"+client["id"].(string)+"/status", bytes.NewReader(statusBody))
	require.NoError(t, err)
	statusResp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer statusResp.Body.Close()
	require.Equal(t, http.StatusOK, statusResp.StatusCode)

	body, err := json.Marshal(map[string]any{"client_id": client["id"]})
	require.NoError(t, err)
	resp, err := http.Post(server.URL+"/accounts", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
}

func TestCreateAccount_UnknownClient(t *testing.T) {
	server := newTestServer(t)

	body, err := json.Marshal(map[string]any{"client_id": "00000000-0000-0000-0000-000000000000"})
	require.NoError(t, err)
	resp, err := http.Post(server.URL+"/accounts", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestGetAccount(t *testing.T) {
	server := newTestServer(t)
	client := createClientFixture(t, server, "Acme Corp")

	body, err := json.Marshal(map[string]any{"client_id": client["id"], "external_ref": "ext-1"})
	require.NoError(t, err)
	createResp, err := http.Post(server.URL+"/accounts", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer createResp.Body.Close()
	var created map[string]any
	require.NoError(t, json.NewDecoder(createResp.Body).Decode(&created))

	resp, err := http.Get(server.URL + "/accounts/" + created["id"].(string))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var fetched map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&fetched))
	assert.Equal(t, created["id"], fetched["id"])
	assert.Equal(t, client["id"], fetched["client_id"])
	assert.Equal(t, "active", fetched["status"])
}

func TestGetAccount_NotFound(t *testing.T) {
	server := newTestServer(t)

	resp, err := http.Get(server.URL + "/accounts/00000000-0000-0000-0000-000000000000")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestListAccounts_ByClient(t *testing.T) {
	server := newTestServer(t)
	client := createClientFixture(t, server, "Acme Corp")

	body, err := json.Marshal(map[string]any{"client_id": client["id"]})
	require.NoError(t, err)
	createResp, err := http.Post(server.URL+"/accounts", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer createResp.Body.Close()
	require.Equal(t, http.StatusCreated, createResp.StatusCode)

	resp, err := http.Get(server.URL + "/accounts?client_id=" + client["id"].(string))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var listed []map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&listed))
	assert.Len(t, listed, 1)
}

func TestListAccounts_RequiresClientID(t *testing.T) {
	server := newTestServer(t)

	resp, err := http.Get(server.URL + "/accounts")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestUpdateAccountStatus(t *testing.T) {
	server := newTestServer(t)
	client := createClientFixture(t, server, "Acme Corp")

	body, err := json.Marshal(map[string]any{"client_id": client["id"]})
	require.NoError(t, err)
	createResp, err := http.Post(server.URL+"/accounts", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer createResp.Body.Close()
	var created map[string]any
	require.NoError(t, json.NewDecoder(createResp.Body).Decode(&created))

	statusBody, err := json.Marshal(map[string]any{"status": "closed"})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPatch, server.URL+"/accounts/"+created["id"].(string)+"/status", bytes.NewReader(statusBody))
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var updated map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&updated))
	assert.Equal(t, "closed", updated["status"])
}
