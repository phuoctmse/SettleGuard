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

func TestGetSettlement(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := settlement.NewSettlementRepository(db)

	pending := uuid.New()
	insertTransactionWithStatus(t, db, pending, 1_000, settlement.StatusPendingSettlement)
	seeded, err := repo.RunBatch(context.Background())
	require.NoError(t, err)
	require.NotNil(t, seeded)

	got, err := repo.Get(context.Background(), seeded.ID)

	require.NoError(t, err)
	assert.Equal(t, seeded.TransactionCount, got.TransactionCount)
	assert.Equal(t, seeded.TotalAmount, got.TotalAmount)
	assert.ElementsMatch(t, seeded.TransactionIDs, got.TransactionIDs)
}

func TestGetSettlement_NotFound(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := settlement.NewSettlementRepository(db)

	_, err := repo.Get(context.Background(), uuid.New())

	assert.ErrorIs(t, err, settlement.ErrSettlementNotFound)
}

func TestListSettlements_MostRecentFirst(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := settlement.NewSettlementRepository(db)

	insertTransactionWithStatus(t, db, uuid.New(), 1_000, settlement.StatusPendingSettlement)
	first, err := repo.RunBatch(context.Background())
	require.NoError(t, err)
	require.NotNil(t, first)

	insertTransactionWithStatus(t, db, uuid.New(), 2_000, settlement.StatusPendingSettlement)
	second, err := repo.RunBatch(context.Background())
	require.NoError(t, err)
	require.NotNil(t, second)

	list, _, err := repo.ListPage(context.Background(), 50, nil)

	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, second.ID, list[0].ID)
	assert.Equal(t, first.ID, list[1].ID)
}

func TestListSettlements_IncludesTransactionIDs(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := settlement.NewSettlementRepository(db)

	tx1, tx2 := uuid.New(), uuid.New()
	insertTransactionWithStatus(t, db, tx1, 1_000, settlement.StatusPendingSettlement)
	insertTransactionWithStatus(t, db, tx2, 2_000, settlement.StatusPendingSettlement)
	batch, err := repo.RunBatch(context.Background())
	require.NoError(t, err)
	require.NotNil(t, batch)

	list, _, err := repo.ListPage(context.Background(), 50, nil)

	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.ElementsMatch(t, []uuid.UUID{tx1, tx2}, list[0].TransactionIDs)
}

// Three pending transactions of MaxInt64 sum, modulo 2^64, to 2^63-3: a
// positive number that passes the settlements.total_amount > 0 CHECK and
// would be persisted as the batch total. Two would wrap negative and be
// caught by the CHECK; three is the case only a code-level guard catches.
func TestRunBatch_RejectsOverflowingTotal(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := settlement.NewSettlementRepository(db)
	const max = int64(9223372036854775807)
	for i := 0; i < 3; i++ {
		insertTransactionWithStatus(t, db, uuid.New(), max, settlement.StatusPendingSettlement)
	}

	_, err := repo.RunBatch(context.Background())
	require.ErrorIs(t, err, settlement.ErrBatchTotalOverflow)
}

func insertSettlementAt(t *testing.T, db *sql.DB, id uuid.UUID, at time.Time) {
	t.Helper()
	_, err := db.Exec(`
		INSERT INTO settlements (id, transaction_count, total_amount, created_at) VALUES ($1, 1, 100, $2)
	`, id, at)
	require.NoError(t, err)
}

func TestListSettlementsPage_WalksEveryRowOnceNewestFirstWithTies(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := settlement.NewSettlementRepository(db)
	base := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)

	newest := uuid.New()
	insertSettlementAt(t, db, newest, base.Add(time.Minute))
	tied := []uuid.UUID{uuid.New(), uuid.New()}
	for _, id := range tied {
		insertSettlementAt(t, db, id, base)
	}
	sort.Slice(tied, func(i, j int) bool { return tied[i].String() > tied[j].String() })
	want := append([]uuid.UUID{newest}, tied...)

	var got []uuid.UUID
	var after *settlement.Cursor
	for pages := 0; ; pages++ {
		require.Less(t, pages, 10, "pagination did not terminate")
		page, next, err := repo.ListPage(context.Background(), 1, after)
		require.NoError(t, err)
		require.Len(t, page, 1)
		got = append(got, page[0].ID)
		if next == nil {
			break
		}
		after = next
	}

	assert.Equal(t, want, got)
}

func TestListSettlementsPage_IncludesTransactionIDs(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := settlement.NewSettlementRepository(db)
	tx1, tx2 := uuid.New(), uuid.New()
	insertTransactionWithStatus(t, db, tx1, 1_000, settlement.StatusPendingSettlement)
	insertTransactionWithStatus(t, db, tx2, 2_000, settlement.StatusPendingSettlement)
	_, err := repo.RunBatch(context.Background())
	require.NoError(t, err)

	page, next, err := repo.ListPage(context.Background(), 50, nil)

	require.NoError(t, err)
	require.Len(t, page, 1)
	assert.Nil(t, next)
	assert.ElementsMatch(t, []uuid.UUID{tx1, tx2}, page[0].TransactionIDs)
}
