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

// Per-account deltas are plain int64 and can wrap. A wrapped delta would be
// applied to Account.balance as if it were real, so BalanceDeltas must
// refuse rather than return a nonsense number.
func TestBalanceDeltas_RejectsOverflowingCredit(t *testing.T) {
	accountID := uuid.New()
	const max = int64(9223372036854775807)
	entries := []ledgerevent.OutboxPayloadEntry{
		{AccountID: accountID, Direction: "credit", Amount: max},
		{AccountID: accountID, Direction: "credit", Amount: max},
	}

	_, err := ledgerevent.BalanceDeltas(entries)
	assert.Error(t, err)
}

func TestBalanceDeltas_RejectsOverflowingDebit(t *testing.T) {
	accountID := uuid.New()
	const max = int64(9223372036854775807)
	entries := []ledgerevent.OutboxPayloadEntry{
		{AccountID: accountID, Direction: "debit", Amount: max},
		{AccountID: accountID, Direction: "debit", Amount: max},
	}

	_, err := ledgerevent.BalanceDeltas(entries)
	assert.Error(t, err)
}

func TestBalanceDeltas_RejectsNonPositiveAmount(t *testing.T) {
	accountID := uuid.New()
	for _, amount := range []int64{0, -5} {
		entries := []ledgerevent.OutboxPayloadEntry{
			{AccountID: accountID, Direction: "credit", Amount: amount},
		}
		_, err := ledgerevent.BalanceDeltas(entries)
		assert.Error(t, err, "amount %d", amount)
	}
}
