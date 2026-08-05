package ledger_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/phuoctmse/settleguard/ledger-service/internal/ledger"
	"github.com/phuoctmse/settleguard/ledger-service/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepository_InsertAndListTransaction(t *testing.T) {
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
	byAccount, err := repo.ListByAccount(context.Background(), accountA)
	require.NoError(t, err)
	require.Len(t, byAccount, 1)
	assert.Equal(t, accountA, byAccount[0].AccountID)

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
