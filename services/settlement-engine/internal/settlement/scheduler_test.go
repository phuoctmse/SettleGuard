package settlement_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/phuoctmse/settleguard/settlement-engine/internal/settlement"
	"github.com/phuoctmse/settleguard/settlement-engine/internal/testutil"
)

func TestScheduler_RunsBatchOnEachTick(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := settlement.NewSettlementRepository(db)
	sched := settlement.NewScheduler(repo, 20*time.Millisecond)

	pending := uuid.New()
	insertTransactionWithStatus(t, db, pending, 1_000, settlement.StatusPendingSettlement)

	go func() { _ = sched.Run(t.Context()) }()

	require.Eventually(t, func() bool {
		var status string
		if err := db.QueryRow(`SELECT status FROM transactions WHERE id = $1`, pending).Scan(&status); err != nil {
			return false
		}
		return status == settlement.StatusSettled
	}, 2*time.Second, 20*time.Millisecond)
}
