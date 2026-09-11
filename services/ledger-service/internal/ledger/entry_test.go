package ledger_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/phuoctmse/settleguard/ledger-service/internal/ledger"
)

func TestValidate(t *testing.T) {
	accountA := uuid.New()
	accountB := uuid.New()

	tests := []struct {
		name    string
		entries []ledger.Entry
		wantErr error
	}{
		{
			name: "balanced debit and credit",
			entries: []ledger.Entry{
				{AccountID: accountA, Direction: ledger.Debit, Amount: 500, Reason: "invoice"},
				{AccountID: accountB, Direction: ledger.Credit, Amount: 500, Reason: "invoice"},
			},
			wantErr: nil,
		},
		{
			name: "unbalanced amounts",
			entries: []ledger.Entry{
				{AccountID: accountA, Direction: ledger.Debit, Amount: 500, Reason: "invoice"},
				{AccountID: accountB, Direction: ledger.Credit, Amount: 400, Reason: "invoice"},
			},
			wantErr: ledger.ErrUnbalancedTransaction,
		},
		{
			name:    "no entries",
			entries: []ledger.Entry{},
			wantErr: ledger.ErrNoEntries,
		},
		{
			name: "zero amount",
			entries: []ledger.Entry{
				{AccountID: accountA, Direction: ledger.Debit, Amount: 0, Reason: "invoice"},
			},
			wantErr: ledger.ErrInvalidAmount,
		},
		{
			name: "invalid direction",
			entries: []ledger.Entry{
				{AccountID: accountA, Direction: "sideways", Amount: 100, Reason: "invoice"},
			},
			wantErr: ledger.ErrInvalidDirection,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ledger.ValidateBalanced(tt.entries)
			if tt.wantErr == nil {
				assert.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, tt.wantErr)
			}
		})
	}
}

// The int64 sums in ValidateBalanced wrap modulo 2^64. Without a ceiling,
// three credits of MaxInt64, MaxInt64 and 4 sum to exactly 2, so a lone
// debit of 2 "balances" 18,446,744,073,709,551,618 VND of credit and the
// transaction is accepted -- LEDGER-01 defeated by overflow, not by an
// unbalanced request. This is the exact shape a caller would send.
func TestValidateBalanced_RejectsOverflowingSums(t *testing.T) {
	accountA := uuid.New()
	accountB := uuid.New()
	const max = int64(9223372036854775807)

	entries := []ledger.Entry{
		{AccountID: accountA, Direction: ledger.Credit, Amount: max, Reason: "x"},
		{AccountID: accountA, Direction: ledger.Credit, Amount: max, Reason: "x"},
		{AccountID: accountA, Direction: ledger.Credit, Amount: 4, Reason: "x"},
		{AccountID: accountB, Direction: ledger.Debit, Amount: 2, Reason: "x"},
	}

	err := ledger.ValidateBalanced(entries)
	assert.ErrorIs(t, err, ledger.ErrAmountTooLarge)
}

func TestValidateBalanced_RejectsAmountAboveCeiling(t *testing.T) {
	accountA := uuid.New()
	accountB := uuid.New()
	over := ledger.MaxEntryAmount + 1

	entries := []ledger.Entry{
		{AccountID: accountA, Direction: ledger.Debit, Amount: over, Reason: "x"},
		{AccountID: accountB, Direction: ledger.Credit, Amount: over, Reason: "x"},
	}

	assert.ErrorIs(t, ledger.ValidateBalanced(entries), ledger.ErrAmountTooLarge)
}

func TestValidateBalanced_AcceptsAmountAtCeiling(t *testing.T) {
	accountA := uuid.New()
	accountB := uuid.New()

	entries := []ledger.Entry{
		{AccountID: accountA, Direction: ledger.Debit, Amount: ledger.MaxEntryAmount, Reason: "x"},
		{AccountID: accountB, Direction: ledger.Credit, Amount: ledger.MaxEntryAmount, Reason: "x"},
	}

	assert.NoError(t, ledger.ValidateBalanced(entries))
}

func TestValidateBalanced_RejectsTooManyEntries(t *testing.T) {
	accountA := uuid.New()
	accountB := uuid.New()

	// One over the cap, kept balanced so the only reason to reject is count.
	n := ledger.MaxEntriesPerTransaction + 1
	entries := make([]ledger.Entry, 0, n)
	for i := 0; i < n-1; i++ {
		entries = append(entries, ledger.Entry{AccountID: accountA, Direction: ledger.Debit, Amount: 1, Reason: "x"})
	}
	entries = append(entries, ledger.Entry{AccountID: accountB, Direction: ledger.Credit, Amount: int64(n - 1), Reason: "x"})

	assert.ErrorIs(t, ledger.ValidateBalanced(entries), ledger.ErrTooManyEntries)
}

// The two limits are what make the plain int64 accumulation in
// ValidateBalanced safe: the largest possible sum is cap * ceiling, and
// that product must fit in int64 with room to spare. Anyone raising either
// constant has to keep this true, or the overflow above comes back.
func TestValidateBalanced_LimitsCannotOverflowInt64(t *testing.T) {
	const maxInt64 = int64(9223372036854775807)
	// Division instead of multiplication so the assertion itself cannot wrap.
	assert.GreaterOrEqual(t, maxInt64/ledger.MaxEntryAmount, int64(ledger.MaxEntriesPerTransaction),
		"MaxEntriesPerTransaction * MaxEntryAmount must not exceed MaxInt64")
}
