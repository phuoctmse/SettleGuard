package settlement_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/phuoctmse/settleguard/settlement-engine/internal/settlement"
	"github.com/phuoctmse/settleguard/settlement-engine/internal/testutil"
)

func insertTestTransaction(t *testing.T, db *sql.DB, id uuid.UUID) {
	t.Helper()
	_, err := db.Exec(`
		INSERT INTO transactions (id, amount, score, decision, status, triggered_rules, scored_at)
		VALUES ($1, 1000, 0, 'pass', 'pending_settlement', '{}', now())
	`, id)
	require.NoError(t, err)
}

func insertTransactionAccount(t *testing.T, db *sql.DB, transactionID, accountID uuid.UUID, createdAt time.Time) {
	t.Helper()
	_, err := db.Exec(`
		INSERT INTO transaction_accounts (transaction_id, account_id, created_at)
		VALUES ($1, $2, $3)
	`, transactionID, accountID, createdAt)
	require.NoError(t, err)
}

func insertBlocklistEntry(t *testing.T, db *sql.DB, accountID uuid.UUID) {
	t.Helper()
	_, err := db.Exec(`
		INSERT INTO blocklist (id, entity_type, entity_id, reason) VALUES ($1, 'account', $2, 'test')
	`, uuid.New(), accountID)
	require.NoError(t, err)
}

func TestCountRecentTransactions(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := settlement.NewTransactionRepository(db)
	ctx := context.Background()

	acc := uuid.New()
	other := uuid.New()
	now := time.Now().UTC()
	since := now.Add(-5 * time.Minute)

	// Two within the window for acc.
	tx1, tx2 := uuid.New(), uuid.New()
	insertTestTransaction(t, db, tx1)
	insertTestTransaction(t, db, tx2)
	insertTransactionAccount(t, db, tx1, acc, now.Add(-1*time.Minute))
	insertTransactionAccount(t, db, tx2, acc, now.Add(-2*time.Minute))

	// One outside the window for acc -- must be excluded.
	tx3 := uuid.New()
	insertTestTransaction(t, db, tx3)
	insertTransactionAccount(t, db, tx3, acc, now.Add(-10*time.Minute))

	// One within the window but for a different account -- must be excluded.
	tx4 := uuid.New()
	insertTestTransaction(t, db, tx4)
	insertTransactionAccount(t, db, tx4, other, now.Add(-1*time.Minute))

	count, err := repo.CountRecentTransactions(ctx, acc, since)
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestIsBlocked_FalseWhenNoneBlocked(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := settlement.NewTransactionRepository(db)

	acc1, acc2 := uuid.New(), uuid.New()

	blocked, err := repo.IsBlocked(context.Background(), []uuid.UUID{acc1, acc2})
	require.NoError(t, err)
	assert.False(t, blocked)
}

func TestIsBlocked_TrueWhenSingleAccountBlocked(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := settlement.NewTransactionRepository(db)

	acc := uuid.New()
	insertBlocklistEntry(t, db, acc)

	blocked, err := repo.IsBlocked(context.Background(), []uuid.UUID{acc})
	require.NoError(t, err)
	assert.True(t, blocked)
}

func TestIsBlocked_TrueWhenAnyOfMultipleBlocked(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := settlement.NewTransactionRepository(db)

	clean1, blockedAcc, clean2 := uuid.New(), uuid.New(), uuid.New()
	insertBlocklistEntry(t, db, blockedAcc)

	blocked, err := repo.IsBlocked(context.Background(), []uuid.UUID{clean1, blockedAcc, clean2})
	require.NoError(t, err)
	assert.True(t, blocked)
}
