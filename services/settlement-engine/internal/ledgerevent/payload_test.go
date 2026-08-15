package ledgerevent_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/phuoctmse/settleguard/settlement-engine/internal/ledgerevent"
)

func TestTotalAmount(t *testing.T) {
	acc1, acc2 := uuid.New(), uuid.New()

	tests := []struct {
		name    string
		entries []ledgerevent.OutboxPayloadEntry
		want    int64
	}{
		{
			name:    "no entries",
			entries: nil,
			want:    0,
		},
		{
			name: "single balanced pair sums debit side",
			entries: []ledgerevent.OutboxPayloadEntry{
				{AccountID: acc1, Direction: "debit", Amount: 500},
				{AccountID: acc2, Direction: "credit", Amount: 500},
			},
			want: 500,
		},
		{
			name: "multiple debit entries sum together",
			entries: []ledgerevent.OutboxPayloadEntry{
				{AccountID: acc1, Direction: "debit", Amount: 300},
				{AccountID: acc1, Direction: "debit", Amount: 200},
				{AccountID: acc2, Direction: "credit", Amount: 500},
			},
			want: 500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ledgerevent.TotalAmount(tt.entries))
		})
	}
}

func TestAccountIDs(t *testing.T) {
	acc1, acc2, acc3 := uuid.New(), uuid.New(), uuid.New()

	tests := []struct {
		name    string
		entries []ledgerevent.OutboxPayloadEntry
		want    []uuid.UUID
	}{
		{
			name:    "no entries",
			entries: nil,
			want:    nil,
		},
		{
			name: "distinct accounts in first-seen order",
			entries: []ledgerevent.OutboxPayloadEntry{
				{AccountID: acc1, Direction: "debit", Amount: 100},
				{AccountID: acc2, Direction: "credit", Amount: 100},
			},
			want: []uuid.UUID{acc1, acc2},
		},
		{
			name: "repeated account deduplicated",
			entries: []ledgerevent.OutboxPayloadEntry{
				{AccountID: acc1, Direction: "debit", Amount: 100},
				{AccountID: acc1, Direction: "debit", Amount: 50},
				{AccountID: acc2, Direction: "credit", Amount: 150},
				{AccountID: acc3, Direction: "credit", Amount: 0},
			},
			want: []uuid.UUID{acc1, acc2, acc3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ledgerevent.AccountIDs(tt.entries))
		})
	}
}
