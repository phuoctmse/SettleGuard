package settlement_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/phuoctmse/settleguard/settlement-engine/internal/risk"
	"github.com/phuoctmse/settleguard/settlement-engine/internal/settlement"
	"github.com/phuoctmse/settleguard/settlement-engine/internal/testutil"
)

func passScore(txID uuid.UUID) risk.RiskScore {
	return risk.RiskScore{
		TransactionID: txID,
		Score:         0,
		Decision:      risk.DecisionPass,
		Outcomes: []risk.RuleOutcome{
			{Rule: "velocity_limit", Triggered: false},
			{Rule: "mismatch_threshold", Triggered: false},
			{Rule: "blocklist", Triggered: false},
		},
	}
}

func holdScore(txID uuid.UUID) risk.RiskScore {
	return risk.RiskScore{
		TransactionID: txID,
		Score:         40,
		Decision:      risk.DecisionHold,
		Outcomes: []risk.RuleOutcome{
			{Rule: "velocity_limit", Triggered: true, Detail: "too many transactions"},
			{Rule: "mismatch_threshold", Triggered: false},
			{Rule: "blocklist", Triggered: false},
		},
	}
}

func TestRecordScore_PersistsPassingTransaction(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := settlement.NewTransactionRepository(db)

	txID := uuid.New()
	acc1, acc2 := uuid.New(), uuid.New()

	err := repo.RecordScore(context.Background(), []uuid.UUID{acc1, acc2}, 5_000, passScore(txID))
	require.NoError(t, err)

	var (
		amount int64
		score  int
		decision, status string
	)
	err = db.QueryRow(`SELECT amount, score, decision, status FROM transactions WHERE id = $1`, txID).
		Scan(&amount, &score, &decision, &status)
	require.NoError(t, err)
	assert.Equal(t, int64(5_000), amount)
	assert.Equal(t, 0, score)
	assert.Equal(t, "pass", decision)
	assert.Equal(t, settlement.StatusPendingSettlement, status)

	var accountCount int
	err = db.QueryRow(`SELECT count(*) FROM transaction_accounts WHERE transaction_id = $1`, txID).Scan(&accountCount)
	require.NoError(t, err)
	assert.Equal(t, 2, accountCount)

	var payloadRaw []byte
	err = db.QueryRow(`SELECT payload FROM outbox_events WHERE subject = $1 AND payload->>'transaction_id' = $2`,
		settlement.EventTransactionRiskScored, txID.String()).Scan(&payloadRaw)
	require.NoError(t, err)

	var payload settlement.RiskScoredPayload
	require.NoError(t, json.Unmarshal(payloadRaw, &payload))
	assert.Equal(t, txID, payload.TransactionID)
	assert.Equal(t, int64(5_000), payload.Amount)
	assert.Equal(t, "pass", payload.Decision)
	assert.Empty(t, payload.TriggeredRules)
	assert.ElementsMatch(t, []uuid.UUID{acc1, acc2}, payload.AccountIDs)
}

func TestRecordScore_PersistsHeldTransaction(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := settlement.NewTransactionRepository(db)

	txID := uuid.New()
	acc := uuid.New()

	err := repo.RecordScore(context.Background(), []uuid.UUID{acc}, 12_000_000, holdScore(txID))
	require.NoError(t, err)

	var status, decision string
	err = db.QueryRow(`SELECT status, decision FROM transactions WHERE id = $1`, txID).Scan(&status, &decision)
	require.NoError(t, err)
	assert.Equal(t, settlement.StatusHeld, status)
	assert.Equal(t, "hold", decision)

	var payloadRaw []byte
	err = db.QueryRow(`SELECT payload FROM outbox_events WHERE subject = $1 AND payload->>'transaction_id' = $2`,
		settlement.EventTransactionRiskScored, txID.String()).Scan(&payloadRaw)
	require.NoError(t, err)

	var payload settlement.RiskScoredPayload
	require.NoError(t, json.Unmarshal(payloadRaw, &payload))
	assert.Equal(t, []string{"velocity_limit"}, payload.TriggeredRules)
}

func TestRecordScore_IdempotentOnSameTransactionID(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := settlement.NewTransactionRepository(db)

	txID := uuid.New()
	acc := uuid.New()
	score := passScore(txID)

	require.NoError(t, repo.RecordScore(context.Background(), []uuid.UUID{acc}, 1_000, score))
	require.NoError(t, repo.RecordScore(context.Background(), []uuid.UUID{acc}, 1_000, score))

	var txCount, outboxCount int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM transactions WHERE id = $1`, txID).Scan(&txCount))
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM outbox_events WHERE subject = $1 AND payload->>'transaction_id' = $2`,
		settlement.EventTransactionRiskScored, txID.String()).Scan(&outboxCount))

	assert.Equal(t, 1, txCount)
	assert.Equal(t, 1, outboxCount)
}
