package ledgerevent_test

import (
	"testing"
	"time"

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

func TestOccurredAt(t *testing.T) {
	earlier := time.Date(2026, 8, 15, 10, 0, 0, 0, time.UTC)
	later := earlier.Add(5 * time.Minute)

	tests := []struct {
		name    string
		entries []ledgerevent.OutboxPayloadEntry
		want    time.Time
	}{
		{
			name:    "no entries",
			entries: nil,
			want:    time.Time{},
		},
		{
			name: "single entry",
			entries: []ledgerevent.OutboxPayloadEntry{
				{CreatedAt: earlier},
			},
			want: earlier,
		},
		{
			name: "returns the earliest entry regardless of order",
			entries: []ledgerevent.OutboxPayloadEntry{
				{CreatedAt: later},
				{CreatedAt: earlier},
			},
			want: earlier,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.True(t, tt.want.Equal(ledgerevent.OccurredAt(tt.entries)))
		})
	}
}
