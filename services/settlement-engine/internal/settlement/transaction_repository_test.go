package settlement_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"sort"
	"testing"
	"time"

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
		amount           int64
		score            int
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

func TestGetTransaction(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := settlement.NewTransactionRepository(db)

	txID := uuid.New()
	acc := uuid.New()
	require.NoError(t, repo.RecordScore(context.Background(), []uuid.UUID{acc}, 1_000, passScore(txID)))

	got, err := repo.Get(context.Background(), txID)

	require.NoError(t, err)
	assert.Equal(t, txID, got.ID)
	assert.Equal(t, []uuid.UUID{acc}, got.AccountIDs)
	assert.Equal(t, settlement.StatusPendingSettlement, got.Status)
}

func TestGetTransaction_NotFound(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := settlement.NewTransactionRepository(db)

	_, err := repo.Get(context.Background(), uuid.New())

	assert.ErrorIs(t, err, settlement.ErrTransactionNotFound)
}

func TestListByStatus_ReturnsOnlyMatchingStatus(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := settlement.NewTransactionRepository(db)

	heldID, passID := uuid.New(), uuid.New()
	acc := uuid.New()
	require.NoError(t, repo.RecordScore(context.Background(), []uuid.UUID{acc}, 1_000, holdScore(heldID)))
	require.NoError(t, repo.RecordScore(context.Background(), []uuid.UUID{acc}, 1_000, passScore(passID)))

	held, _, err := repo.ListByStatusPage(context.Background(), settlement.StatusHeld, 50, nil)

	require.NoError(t, err)
	require.Len(t, held, 1)
	assert.Equal(t, heldID, held[0].ID)
	assert.Equal(t, []string{"velocity_limit"}, held[0].TriggeredRules)
}

func TestApprove_HeldToPendingSettlement(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := settlement.NewTransactionRepository(db)

	txID := uuid.New()
	acc := uuid.New()
	require.NoError(t, repo.RecordScore(context.Background(), []uuid.UUID{acc}, 1_000, holdScore(txID)))

	got, err := repo.Approve(context.Background(), txID)

	require.NoError(t, err)
	assert.Equal(t, settlement.StatusPendingSettlement, got.Status)

	var payloadRaw []byte
	require.NoError(t, db.QueryRow(`SELECT payload FROM outbox_events WHERE subject = $1 AND payload->>'transaction_id' = $2`,
		settlement.EventTransactionResolved, txID.String()).Scan(&payloadRaw))
	var payload settlement.ResolvedPayload
	require.NoError(t, json.Unmarshal(payloadRaw, &payload))
	assert.Equal(t, "approved", payload.Resolution)
}

func TestReject_HeldToRejected(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := settlement.NewTransactionRepository(db)

	txID := uuid.New()
	acc := uuid.New()
	require.NoError(t, repo.RecordScore(context.Background(), []uuid.UUID{acc}, 1_000, holdScore(txID)))

	got, err := repo.Reject(context.Background(), txID)

	require.NoError(t, err)
	assert.Equal(t, settlement.StatusRejected, got.Status)

	var payloadRaw []byte
	require.NoError(t, db.QueryRow(`SELECT payload FROM outbox_events WHERE subject = $1 AND payload->>'transaction_id' = $2`,
		settlement.EventTransactionResolved, txID.String()).Scan(&payloadRaw))
	var payload settlement.ResolvedPayload
	require.NoError(t, json.Unmarshal(payloadRaw, &payload))
	assert.Equal(t, "rejected", payload.Resolution)
}

func TestApprove_NotHeld_ReturnsError(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := settlement.NewTransactionRepository(db)

	txID := uuid.New()
	acc := uuid.New()
	require.NoError(t, repo.RecordScore(context.Background(), []uuid.UUID{acc}, 1_000, passScore(txID)))

	_, err := repo.Approve(context.Background(), txID)

	assert.ErrorIs(t, err, settlement.ErrTransactionNotHeld)
}

func TestReject_AlreadyRejected_ReturnsError(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := settlement.NewTransactionRepository(db)

	txID := uuid.New()
	acc := uuid.New()
	require.NoError(t, repo.RecordScore(context.Background(), []uuid.UUID{acc}, 1_000, holdScore(txID)))
	_, err := repo.Reject(context.Background(), txID)
	require.NoError(t, err)

	_, err = repo.Reject(context.Background(), txID)

	assert.ErrorIs(t, err, settlement.ErrTransactionNotHeld)
}

func TestApprove_NotFound_ReturnsError(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := settlement.NewTransactionRepository(db)

	_, err := repo.Approve(context.Background(), uuid.New())

	assert.ErrorIs(t, err, settlement.ErrTransactionNotFound)
}

func insertHeldAt(t *testing.T, db *sql.DB, id uuid.UUID, at time.Time) {
	t.Helper()
	_, err := db.Exec(`
		INSERT INTO transactions (id, amount, score, decision, status, triggered_rules, scored_at)
		VALUES ($1, 1000, 40, 'hold', 'held', '{velocity_limit}', $2)
	`, id, at)
	require.NoError(t, err)
}

func collectHeld(t *testing.T, repo *settlement.TransactionRepository, limit int) ([]uuid.UUID, int) {
	t.Helper()
	var ids []uuid.UUID
	var after *settlement.Cursor
	pages := 0
	for {
		page, next, err := repo.ListByStatusPage(context.Background(), settlement.StatusHeld, limit, after)
		require.NoError(t, err)
		require.LessOrEqual(t, len(page), limit)
		pages++
		for _, tx := range page {
			ids = append(ids, tx.ID)
		}
		if next == nil {
			return ids, pages
		}
		after = next
		require.Less(t, pages, 100, "pagination did not terminate")
	}
}

func TestListByStatusPage_WalksEveryRowOnceNewestFirst(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := settlement.NewTransactionRepository(db)
	base := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)

	var want []uuid.UUID // newest first
	for i := 4; i >= 0; i-- {
		id := uuid.New()
		insertHeldAt(t, db, id, base.Add(time.Duration(i)*time.Second))
		want = append(want, id)
	}
	require.NoError(t, repo.RecordScore(context.Background(), []uuid.UUID{uuid.New()}, 1_000, passScore(uuid.New())))

	got, pages := collectHeld(t, repo, 2)

	assert.Equal(t, want, got)
	assert.Equal(t, 3, pages)
}

func TestListByStatusPage_BreaksTimestampTiesByID(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := settlement.NewTransactionRepository(db)
	at := time.Date(2026, 10, 18, 12, 0, 0, 123456000, time.UTC)

	ids := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
	for _, id := range ids {
		insertHeldAt(t, db, id, at)
	}
	// Postgres orders uuid bytewise, which matches the canonical string order.
	sort.Slice(ids, func(i, j int) bool { return ids[i].String() > ids[j].String() })

	got, pages := collectHeld(t, repo, 1)

	assert.Equal(t, ids, got)
	assert.Equal(t, 3, pages)
}

func TestListByStatusPage_SinglePageHasNoCursor(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := settlement.NewTransactionRepository(db)
	insertHeldAt(t, db, uuid.New(), time.Now())

	page, next, err := repo.ListByStatusPage(context.Background(), settlement.StatusHeld, 50, nil)

	require.NoError(t, err)
	assert.Len(t, page, 1)
	assert.Nil(t, next)
}
