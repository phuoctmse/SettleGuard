package ledger_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/phuoctmse/settleguard/ledger-service/internal/ledger"
	"github.com/stretchr/testify/assert"
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
