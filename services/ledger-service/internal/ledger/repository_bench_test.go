package ledger_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/phuoctmse/settleguard/ledger-service/internal/ledger"
	"github.com/phuoctmse/settleguard/ledger-service/internal/testutil"
)

func BenchmarkRepository_InsertTransaction(b *testing.B) {
	conn := testutil.NewTestDB(b)
	repo := ledger.NewRepository(conn)
	ctx := context.Background()

	for b.Loop() {
		entries := []ledger.Entry{
			{AccountID: uuid.New(), Direction: ledger.Debit, Amount: 100, Reason: "bench"},
			{AccountID: uuid.New(), Direction: ledger.Credit, Amount: 100, Reason: "bench"},
		}
		if _, err := repo.InsertTransaction(ctx, uuid.New(), entries); err != nil {
			b.Fatalf("insert transaction: %v", err)
		}
	}
}
