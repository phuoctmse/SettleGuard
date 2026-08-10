package ledgerevent_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/phuoctmse/settleguard/accounts-service/internal/ledgerevent"
)

func TestBalanceDeltas_SingleCredit(t *testing.T) {
	accountID := uuid.New()
	entries := []ledgerevent.OutboxPayloadEntry{
		{AccountID: accountID, Direction: "credit", Amount: 500},
	}

	deltas, err := ledgerevent.BalanceDeltas(entries)
	require.NoError(t, err)
	assert.Equal(t, int64(500), deltas[accountID])
}

func TestBalanceDeltas_SingleDebit(t *testing.T) {
	accountID := uuid.New()
	entries := []ledgerevent.OutboxPayloadEntry{
		{AccountID: accountID, Direction: "debit", Amount: 500},
	}

	deltas, err := ledgerevent.BalanceDeltas(entries)
	require.NoError(t, err)
	assert.Equal(t, int64(-500), deltas[accountID])
}

func TestBalanceDeltas_MultipleEntriesSameAccountNet(t *testing.T) {
	accountID := uuid.New()
	entries := []ledgerevent.OutboxPayloadEntry{
		{AccountID: accountID, Direction: "credit", Amount: 500},
		{AccountID: accountID, Direction: "debit", Amount: 200},
	}

	deltas, err := ledgerevent.BalanceDeltas(entries)
	require.NoError(t, err)
	assert.Equal(t, int64(300), deltas[accountID])
}

func TestBalanceDeltas_MultipleAccounts(t *testing.T) {
	accountA := uuid.New()
	accountB := uuid.New()
	entries := []ledgerevent.OutboxPayloadEntry{
		{AccountID: accountA, Direction: "debit", Amount: 500},
		{AccountID: accountB, Direction: "credit", Amount: 500},
	}

	deltas, err := ledgerevent.BalanceDeltas(entries)
	require.NoError(t, err)
	assert.Equal(t, int64(-500), deltas[accountA])
	assert.Equal(t, int64(500), deltas[accountB])
}

func TestBalanceDeltas_UnknownDirection(t *testing.T) {
	entries := []ledgerevent.OutboxPayloadEntry{
		{AccountID: uuid.New(), Direction: "bogus", Amount: 500},
	}

	_, err := ledgerevent.BalanceDeltas(entries)
	assert.Error(t, err)
}
