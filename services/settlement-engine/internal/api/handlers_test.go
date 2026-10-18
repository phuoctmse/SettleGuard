package api_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
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

type pageBody struct {
	Items      []map[string]any `json:"items"`
	NextCursor *string          `json:"next_cursor"`
}

func getPage(t *testing.T, url string) pageBody {
	t.Helper()
	resp, err := http.Get(url)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var body pageBody
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.NotNil(t, body.Items, "items must be an array, never null")
	return body
}

func TestListTransactions_ReturnsHeldOnly(t *testing.T) {
	server, transactions, _ := newTestServer(t)
	heldID := seedHeldTransaction(t, transactions)

	body := getPage(t, server.URL+"/transactions?status=held")

	require.Len(t, body.Items, 1)
	assert.Equal(t, heldID.String(), body.Items[0]["id"])
	assert.Equal(t, "held", body.Items[0]["status"])
	assert.Nil(t, body.NextCursor)
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

	body := getPage(t, server.URL+"/settlements")

	require.Len(t, body.Items, 1)
	assert.Equal(t, seeded.ID.String(), body.Items[0]["id"])
	assert.Nil(t, body.NextCursor)
}

func TestListTransactions_PagesWithCursor(t *testing.T) {
	server, transactions, _ := newTestServer(t)
	seeded := map[string]bool{}
	for range 3 {
		seeded[seedHeldTransaction(t, transactions).String()] = true
	}

	first := getPage(t, server.URL+"/transactions?status=held&limit=2")
	require.Len(t, first.Items, 2)
	require.NotNil(t, first.NextCursor)

	second := getPage(t, server.URL+"/transactions?status=held&limit=2&cursor="+*first.NextCursor)
	require.Len(t, second.Items, 1)
	assert.Nil(t, second.NextCursor)

	got := map[string]bool{}
	for _, item := range append(first.Items, second.Items...) {
		got[item["id"].(string)] = true
	}
	assert.Equal(t, seeded, got)
}

func TestListTransactions_DefaultAndMaximumLimit(t *testing.T) {
	server, transactions, _ := newTestServer(t)
	for range 201 {
		seedHeldTransaction(t, transactions)
	}

	assert.Len(t, getPage(t, server.URL+"/transactions?status=held").Items, 50)
	assert.Len(t, getPage(t, server.URL+"/transactions?status=held&limit=500").Items, 200)
}

func TestListSettlements_PagesWithCursor(t *testing.T) {
	server, transactions, settlements := newTestServer(t)
	var batches []uuid.UUID
	for range 2 {
		require.NoError(t, transactions.RecordScore(context.Background(), []uuid.UUID{uuid.New()}, 5_000, risk.RiskScore{
			TransactionID: uuid.New(), Score: 0, Decision: risk.DecisionPass,
		}))
		s, err := settlements.RunBatch(context.Background())
		require.NoError(t, err)
		require.NotNil(t, s)
		batches = append(batches, s.ID)
	}

	first := getPage(t, server.URL+"/settlements?limit=1")
	require.Len(t, first.Items, 1)
	require.NotNil(t, first.NextCursor)
	second := getPage(t, server.URL+"/settlements?limit=1&cursor="+*first.NextCursor)
	require.Len(t, second.Items, 1)
	assert.Nil(t, second.NextCursor)

	assert.Equal(t, batches[1].String(), first.Items[0]["id"]) // newest first
	assert.Equal(t, batches[0].String(), second.Items[0]["id"])
}

func TestListEndpoints_RejectBadPaging(t *testing.T) {
	server, _, _ := newTestServer(t)
	notJSON := base64.RawURLEncoding.EncodeToString([]byte("not json"))
	badID := base64.RawURLEncoding.EncodeToString([]byte(`{"t":"2026-10-18T12:00:00Z","id":"nope"}`))
	badTime := base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf(`{"t":"yesterday","id":"%s"}`, uuid.New())))

	cases := []struct{ query, message string }{
		{"limit=0", "invalid limit"},
		{"limit=-1", "invalid limit"},
		{"limit=abc", "invalid limit"},
		{"cursor=%25%25%25", "invalid cursor"},
		{"cursor=" + notJSON, "invalid cursor"},
		{"cursor=" + badID, "invalid cursor"},
		{"cursor=" + badTime, "invalid cursor"},
	}
	for _, path := range []string{"/transactions?status=held&", "/settlements?"} {
		for _, c := range cases {
			t.Run(path+c.query, func(t *testing.T) {
				resp, err := http.Get(server.URL + path + c.query)
				require.NoError(t, err)
				defer resp.Body.Close()
				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err)
				assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
				assert.JSONEq(t, fmt.Sprintf(`{"error":%q}`, c.message), string(body))
			})
		}
	}
}

func TestGetSettlement_NotFound(t *testing.T) {
	server, _, _ := newTestServer(t)

	resp, err := http.Get(server.URL + "/settlements/" + uuid.New().String())
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}
