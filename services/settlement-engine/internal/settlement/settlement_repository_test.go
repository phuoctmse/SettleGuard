package settlement_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/phuoctmse/settleguard/settlement-engine/internal/settlement"
	"github.com/phuoctmse/settleguard/settlement-engine/internal/testutil"
)

func insertTransactionWithStatus(t *testing.T, db *sql.DB, id uuid.UUID, amount int64, status string) {
	t.Helper()
	decision := "pass"
	if status == settlement.StatusHeld {
		decision = "hold"
	}
	_, err := db.Exec(`
		INSERT INTO transactions (id, amount, score, decision, status, triggered_rules, scored_at)
		VALUES ($1, $2, 0, $3, $4, '{}', now())
	`, id, amount, decision, status)
	require.NoError(t, err)
}

func TestRunBatch_BatchesPendingExcludesHeldAndSettled(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := settlement.NewSettlementRepository(db)

	pending1, pending2 := uuid.New(), uuid.New()
	held := uuid.New()
	alreadySettled := uuid.New()

	insertTransactionWithStatus(t, db, pending1, 1_000, settlement.StatusPendingSettlement)
	insertTransactionWithStatus(t, db, pending2, 2_000, settlement.StatusPendingSettlement)
	insertTransactionWithStatus(t, db, held, 5_000, settlement.StatusHeld)
	insertTransactionWithStatus(t, db, alreadySettled, 9_000, settlement.StatusSettled)

	result, err := repo.RunBatch(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, 2, result.TransactionCount)
	assert.Equal(t, int64(3_000), result.TotalAmount)
	assert.ElementsMatch(t, []uuid.UUID{pending1, pending2}, result.TransactionIDs)

	var status1, status2 string
	require.NoError(t, db.QueryRow(`SELECT status FROM transactions WHERE id = $1`, pending1).Scan(&status1))
	require.NoError(t, db.QueryRow(`SELECT status FROM transactions WHERE id = $1`, pending2).Scan(&status2))
	assert.Equal(t, settlement.StatusSettled, status1)
	assert.Equal(t, settlement.StatusSettled, status2)

	var heldStatus, alreadySettledStatus string
	require.NoError(t, db.QueryRow(`SELECT status FROM transactions WHERE id = $1`, held).Scan(&heldStatus))
	require.NoError(t, db.QueryRow(`SELECT status FROM transactions WHERE id = $1`, alreadySettled).Scan(&alreadySettledStatus))
	assert.Equal(t, settlement.StatusHeld, heldStatus)
	assert.Equal(t, settlement.StatusSettled, alreadySettledStatus)

	var linkCount int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM settlement_transactions WHERE settlement_id = $1`, result.ID).Scan(&linkCount))
	assert.Equal(t, 2, linkCount)

	var payloadRaw []byte
	require.NoError(t, db.QueryRow(`SELECT payload FROM outbox_events WHERE subject = $1 AND payload->>'settlement_id' = $2`,
		settlement.EventSettlementFinalized, result.ID.String()).Scan(&payloadRaw))
	var payload settlement.SettlementFinalizedPayload
	require.NoError(t, json.Unmarshal(payloadRaw, &payload))
	assert.Equal(t, 2, payload.TransactionCount)
	assert.Equal(t, int64(3_000), payload.TotalAmount)
	assert.ElementsMatch(t, []uuid.UUID{pending1, pending2}, payload.TransactionIDs)
}

func TestRunBatch_NoopWhenNothingPending(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := settlement.NewSettlementRepository(db)

	held := uuid.New()
	insertTransactionWithStatus(t, db, held, 1_000, settlement.StatusHeld)

	result, err := repo.RunBatch(context.Background())
	require.NoError(t, err)
	assert.Nil(t, result)
}
