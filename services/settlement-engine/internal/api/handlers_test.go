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

	"github.com/phuoctmse/settleguard/settlement-engine/internal/api"
	"github.com/phuoctmse/settleguard/settlement-engine/internal/risk"
	"github.com/phuoctmse/settleguard/settlement-engine/internal/settlement"
	"github.com/phuoctmse/settleguard/settlement-engine/internal/testutil"
)

func newTestServer(t *testing.T) (*httptest.Server, *settlement.TransactionRepository, *settlement.SettlementRepository) {
	t.Helper()
	db := testutil.NewTestDB(t)
	transactions := settlement.NewTransactionRepository(db)
	settlements := settlement.NewSettlementRepository(db)
	handlers := api.NewHandlers(transactions, settlements)
	server := httptest.NewServer(api.NewRouter(handlers))
	t.Cleanup(server.Close)
	return server, transactions, settlements
}

func seedHeldTransaction(t *testing.T, transactions *settlement.TransactionRepository) uuid.UUID {
	t.Helper()
	txID := uuid.New()
	acc := uuid.New()
	require.NoError(t, transactions.RecordScore(context.Background(), []uuid.UUID{acc}, 1_000, risk.RiskScore{
		TransactionID: txID,
		Score:         40,
		Decision:      risk.DecisionHold,
		Outcomes:      []risk.RuleOutcome{{Rule: "velocity_limit", Triggered: true}},
	}))
	return txID
}

func TestHealth(t *testing.T) {
	server, _, _ := newTestServer(t)

	resp, err := http.Get(server.URL + "/health")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestListTransactions_RequiresStatus(t *testing.T) {
	server, _, _ := newTestServer(t)

	resp, err := http.Get(server.URL + "/transactions")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestListTransactions_ReturnsHeldOnly(t *testing.T) {
	server, transactions, _ := newTestServer(t)
	heldID := seedHeldTransaction(t, transactions)

	resp, err := http.Get(server.URL + "/transactions?status=held")
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	var body []map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.Len(t, body, 1)
	assert.Equal(t, heldID.String(), body[0]["id"])
	assert.Equal(t, "held", body[0]["status"])
}

func TestGetTransaction_NotFound(t *testing.T) {
	server, _, _ := newTestServer(t)

	resp, err := http.Get(server.URL + "/transactions/" + uuid.New().String())
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestApproveTransaction_HeldToPendingSettlement(t *testing.T) {
	server, transactions, _ := newTestServer(t)
	txID := seedHeldTransaction(t, transactions)

	resp, err := http.Post(server.URL+"/transactions/"+txID.String()+"/approve", "application/json", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "pending_settlement", body["status"])
}

func TestApproveTransaction_AlreadyResolved_Returns409(t *testing.T) {
	server, transactions, _ := newTestServer(t)
	txID := seedHeldTransaction(t, transactions)
	_, err := transactions.Approve(context.Background(), txID)
	require.NoError(t, err)

	resp, err := http.Post(server.URL+"/transactions/"+txID.String()+"/approve", "application/json", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusConflict, resp.StatusCode)
}

func TestApproveTransaction_NotFound_Returns404(t *testing.T) {
	server, _, _ := newTestServer(t)

	resp, err := http.Post(server.URL+"/transactions/"+uuid.New().String()+"/approve", "application/json", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestRejectTransaction_HeldToRejected(t *testing.T) {
	server, transactions, _ := newTestServer(t)
	txID := seedHeldTransaction(t, transactions)

	resp, err := http.Post(server.URL+"/transactions/"+txID.String()+"/reject", "application/json", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "rejected", body["status"])
}

func TestListSettlements_ReturnsFinalizedBatches(t *testing.T) {
	server, transactions, settlements := newTestServer(t)
	txID := uuid.New()
	acc := uuid.New()
	require.NoError(t, transactions.RecordScore(context.Background(), []uuid.UUID{acc}, 5_000, risk.RiskScore{
		TransactionID: txID, Score: 0, Decision: risk.DecisionPass,
	}))
	seeded, err := settlements.RunBatch(context.Background())
	require.NoError(t, err)
	require.NotNil(t, seeded)

	resp, err := http.Get(server.URL + "/settlements")
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	var body []map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.Len(t, body, 1)
	assert.Equal(t, seeded.ID.String(), body[0]["id"])
}

func TestGetSettlement_NotFound(t *testing.T) {
	server, _, _ := newTestServer(t)

	resp, err := http.Get(server.URL + "/settlements/" + uuid.New().String())
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}
