package ledger_test

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/phuoctmse/settleguard/ledger-service/internal/ledger"
	"github.com/phuoctmse/settleguard/ledger-service/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func insertTestTransaction(t *testing.T, repo *ledger.Repository, entries []ledger.Entry) (uuid.UUID, []ledger.Entry) {
	t.Helper()
	txID := uuid.New()
	inserted, err := repo.InsertTransaction(context.Background(), txID, entries)
	require.NoError(t, err)

	return txID, inserted
}

func TestRepository_InsertTransaction(t *testing.T) {
	conn := testutil.NewTestDB(t)
	repo := ledger.NewRepository(conn)

	accountA := uuid.New()
	accountB := uuid.New()
	txID := uuid.New()

	entries := []ledger.Entry{
		{AccountID: accountA, Direction: ledger.Debit, Amount: 1000, Reason: "payout"},
		{AccountID: accountB, Direction: ledger.Credit, Amount: 1000, Reason: "payout"},
	}

	inserted, err := repo.InsertTransaction(context.Background(), txID, entries)
	require.NoError(t, err)
	require.Len(t, inserted, 2)
	for _, e := range inserted {
		assert.Equal(t, txID, e.TransactionID)
		assert.NotEqual(t, uuid.Nil, e.ID)
	}
}

func TestRepository_ListByAccount(t *testing.T) {
	conn := testutil.NewTestDB(t)
	repo := ledger.NewRepository(conn)

	accountA := uuid.New()
	accountB := uuid.New()

	entries := []ledger.Entry{
		{AccountID: accountA, Direction: ledger.Debit, Amount: 1000, Reason: "payout"},
		{AccountID: accountB, Direction: ledger.Credit, Amount: 1000, Reason: "payout"},
	}

	insertTestTransaction(t, repo, entries)

	byAccount, err := repo.ListByAccount(context.Background(), accountA)
	require.NoError(t, err)
	require.Len(t, byAccount, 1)
	assert.Equal(t, accountA, byAccount[0].AccountID)
}

func TestRepository_ListByTransaction(t *testing.T) {
	conn := testutil.NewTestDB(t)
	repo := ledger.NewRepository(conn)

	accountA := uuid.New()
	accountB := uuid.New()

	entries := []ledger.Entry{
		{AccountID: accountA, Direction: ledger.Debit, Amount: 1000, Reason: "payout"},
		{AccountID: accountB, Direction: ledger.Credit, Amount: 1000, Reason: "payout"},
	}

	txID, _ := insertTestTransaction(t, repo, entries)

	byTransaction, err := repo.ListByTransaction(context.Background(), txID)
	require.NoError(t, err)
	assert.Len(t, byTransaction, 2)
}

func TestRepository_InsertTransaction_RejectsUnbalanced(t *testing.T) {
	conn := testutil.NewTestDB(t)
	repo := ledger.NewRepository(conn)

	entries := []ledger.Entry{
		{AccountID: uuid.New(), Direction: ledger.Debit, Amount: 1000, Reason: "payout"},
		{AccountID: uuid.New(), Direction: ledger.Credit, Amount: 500, Reason: "payout"},
	}

	_, err := repo.InsertTransaction(context.Background(), uuid.New(), entries)
	assert.ErrorIs(t, err, ledger.ErrUnbalancedTransaction)
}

func TestRepository_InsertTransaction_ConcurrentSameAccount(t *testing.T) {
	conn := testutil.NewTestDB(t)
	repo := ledger.NewRepository(conn)

	const goroutines = 20
	target := uuid.New()

	var wg sync.WaitGroup
	errs := make(chan error, goroutines)

	for range goroutines {
		wg.Go(func() {
			entries := []ledger.Entry{
				{AccountID: target, Direction: ledger.Debit, Amount: 100, Reason: "concurrent"},
				{AccountID: uuid.New(), Direction: ledger.Credit, Amount: 100, Reason: "concurrent"},
			}
			_, err := repo.InsertTransaction(context.Background(), uuid.New(), entries)
			errs <- err
		})
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}

	byAccount, err := repo.ListByAccount(context.Background(), target)
	require.NoError(t, err)
	assert.Len(t, byAccount, goroutines)
}
