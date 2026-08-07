package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/phuoctmse/settleguard/ledger-service/internal/api"
	"github.com/phuoctmse/settleguard/ledger-service/internal/ledger"
	"github.com/phuoctmse/settleguard/ledger-service/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateTransactionAndListEntries(t *testing.T) {
	conn := testutil.NewTestDB(t)
	repo := ledger.NewRepository(conn)
	handlers := api.NewHandlers(repo)
	server := httptest.NewServer(api.NewRouter(handlers))

	accountA := uuid.New().String()
	accountB := uuid.New().String()

	body := map[string]any{
		"entries": []map[string]any{
			{"account_id": accountA, "direction": "debit", "amount": 1500, "reason": "invoice #1"},
			{"account_id": accountB, "direction": "credit", "amount": 1500, "reason": "invoice #1"},
		},
	}
	payload, err := json.Marshal(body)
	require.NoError(t, err)

	resp, err := http.Post(server.URL+"/transactions", "application/json", bytes.NewReader(payload))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var created []map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&created))
	require.Len(t, created, 2)

	listResp, err := http.Get(server.URL + "/entries?account_id=" + accountA)
	require.NoError(t, err)
	defer listResp.Body.Close()
	assert.Equal(t, http.StatusOK, listResp.StatusCode)

	var listed []map[string]any
	require.NoError(t, json.NewDecoder(listResp.Body).Decode(&listed))
	assert.Len(t, listed, 1)
}

func TestCreateTransaction_RejectsUnbalanced(t *testing.T) {
	conn := testutil.NewTestDB(t)
	repo := ledger.NewRepository(conn)
	handlers := api.NewHandlers(repo)
	server := httptest.NewServer(api.NewRouter(handlers))
	defer server.Close()

	body := map[string]any{
		"entries": []map[string]any{
			{"account_id": uuid.New().String(), "direction": "debit", "amount": 1000, "reason": "bad"},
			{"account_id": uuid.New().String(), "direction": "credit", "amount": 900, "reason": "bad"},
		},
	}
	payload, err := json.Marshal(body)
	require.NoError(t, err)

	resp, err := http.Post(server.URL+"/transactions", "application/json", bytes.NewReader(payload))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
}

func TestListEntries_RequiresQueryParam(t *testing.T) {
	conn := testutil.NewTestDB(t)
	repo := ledger.NewRepository(conn)
	handlers := api.NewHandlers(repo)
	server := httptest.NewServer(api.NewRouter(handlers))
	defer server.Close()

	resp, err := http.Get(server.URL + "/entries")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}
