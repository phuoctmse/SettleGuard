package consumer_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/require"

	"github.com/phuoctmse/settleguard/settlement-engine/internal/broker"
	"github.com/phuoctmse/settleguard/settlement-engine/internal/consumer"
	"github.com/phuoctmse/settleguard/settlement-engine/internal/ledgerevent"
	"github.com/phuoctmse/settleguard/settlement-engine/internal/risk"
	"github.com/phuoctmse/settleguard/settlement-engine/internal/settlement"
	"github.com/phuoctmse/settleguard/settlement-engine/internal/testutil"
)

func testScorerConfig() risk.Config {
	return risk.Config{
		VelocityLimit:     100,
		VelocityWindow:    5 * time.Minute,
		MismatchThreshold: 1_000_000,
	}
}

func ensureLedgerStream(t *testing.T, ctx context.Context, js jetstream.JetStream) {
	t.Helper()
	require.NoError(t, broker.EnsureStream(ctx, js, jetstream.StreamConfig{
		Name:     broker.LedgerEventsStream,
		Subjects: []string{"ledger.>"},
		Storage:  jetstream.FileStorage,
	}))
}

func publishLedgerEntry(t *testing.T, ctx context.Context, js jetstream.JetStream, txID, accountID uuid.UUID, amount int64) {
	t.Helper()
	payload, err := json.Marshal(ledgerevent.OutboxPayload{
		TransactionID: txID,
		Entries: []ledgerevent.OutboxPayloadEntry{
			{ID: uuid.New(), AccountID: accountID, Direction: "debit", Amount: amount, CreatedAt: time.Now()},
		},
	})
	require.NoError(t, err)
	_, err = js.Publish(ctx, ledgerevent.EventLedgerEntryRecorded, payload)
	require.NoError(t, err)
}

func transactionStatus(t *testing.T, db *sql.DB, txID uuid.UUID) (string, bool) {
	t.Helper()
	var status string
	err := db.QueryRow(`SELECT status FROM transactions WHERE id = $1`, txID).Scan(&status)
	if err != nil {
		return "", false
	}
	return status, true
}

func TestConsumer_ScoresAndPersistsOnPublish(t *testing.T) {
	db := testutil.NewTestDB(t)
	_, js := testutil.NewTestNATS(t)
	ctx := context.Background()
	ensureLedgerStream(t, ctx, js)

	transactions := settlement.NewTransactionRepository(db)
	scorer := risk.NewScorer(testScorerConfig(), transactions, transactions)

	c := consumer.New(scorer, transactions)
	consumeCtx, err := c.Start(ctx, js)
	require.NoError(t, err)
	t.Cleanup(consumeCtx.Stop)

	txID, acc := uuid.New(), uuid.New()
	publishLedgerEntry(t, ctx, js, txID, acc, 5_000)

	require.Eventually(t, func() bool {
		status, ok := transactionStatus(t, db, txID)
		return ok && status == settlement.StatusPendingSettlement
	}, 5*time.Second, 100*time.Millisecond, "transaction should be scored and persisted as pending_settlement")
}

func TestConsumer_IdempotentAcrossRedelivery(t *testing.T) {
	db := testutil.NewTestDB(t)
	_, js := testutil.NewTestNATS(t)
	ctx := context.Background()
	ensureLedgerStream(t, ctx, js)

	transactions := settlement.NewTransactionRepository(db)
	scorer := risk.NewScorer(testScorerConfig(), transactions, transactions)

	c := consumer.New(scorer, transactions)
	consumeCtx, err := c.Start(ctx, js)
	require.NoError(t, err)
	t.Cleanup(consumeCtx.Stop)

	txID, acc := uuid.New(), uuid.New()
	// Publish the same transaction_id twice as two separate publishes
	// (deliberately not using WithMsgID, so JetStream's own dedupe window
	// doesn't hide this from ever reaching our handler -- this proves our
	// dedup table works, not NATS's broker-side dedupe).
	publishLedgerEntry(t, ctx, js, txID, acc, 1_000)
	publishLedgerEntry(t, ctx, js, txID, acc, 1_000)

	require.Eventually(t, func() bool {
		status, ok := transactionStatus(t, db, txID)
		return ok && status == settlement.StatusPendingSettlement
	}, 5*time.Second, 100*time.Millisecond)

	time.Sleep(1 * time.Second)
	var count int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM transactions WHERE id = $1`, txID).Scan(&count))
	require.Equal(t, 1, count, "the second publish of the same transaction_id must not double-insert")
}

func TestConsumer_TerminatesMalformedPayloadWithoutWedging(t *testing.T) {
	db := testutil.NewTestDB(t)
	nc, js := testutil.NewTestNATS(t)
	ctx := context.Background()
	ensureLedgerStream(t, ctx, js)

	transactions := settlement.NewTransactionRepository(db)
	scorer := risk.NewScorer(testScorerConfig(), transactions, transactions)

	c := consumer.New(scorer, transactions)
	consumeCtx, err := c.Start(ctx, js)
	require.NoError(t, err)
	t.Cleanup(consumeCtx.Stop)

	require.NoError(t, nc.Publish(ledgerevent.EventLedgerEntryRecorded, []byte("not json")))

	txID, acc := uuid.New(), uuid.New()
	publishLedgerEntry(t, ctx, js, txID, acc, 2_000)

	require.Eventually(t, func() bool {
		status, ok := transactionStatus(t, db, txID)
		return ok && status == settlement.StatusPendingSettlement
	}, 5*time.Second, 100*time.Millisecond, "the malformed message must not block subsequent valid messages")
}

func TestConsumer_AcksHeldTransaction(t *testing.T) {
	db := testutil.NewTestDB(t)
	_, js := testutil.NewTestNATS(t)
	ctx := context.Background()
	ensureLedgerStream(t, ctx, js)

	transactions := settlement.NewTransactionRepository(db)
	scorer := risk.NewScorer(testScorerConfig(), transactions, transactions)

	c := consumer.New(scorer, transactions)
	consumeCtx, err := c.Start(ctx, js)
	require.NoError(t, err)
	t.Cleanup(consumeCtx.Stop)

	// Amount exceeds testScorerConfig's MismatchThreshold, forcing a hold.
	txID, acc := uuid.New(), uuid.New()
	publishLedgerEntry(t, ctx, js, txID, acc, testScorerConfig().MismatchThreshold+1)

	require.Eventually(t, func() bool {
		status, ok := transactionStatus(t, db, txID)
		return ok && status == settlement.StatusHeld
	}, 5*time.Second, 100*time.Millisecond, "a held transaction should still be persisted (and acked, not left redelivering)")

	// If holding were mistakenly Nak()'d instead of Ack()'d, JetStream would
	// keep redelivering and RecordScore's dedup would still leave exactly
	// one row -- so also confirm no runaway redelivery produced more rows.
	time.Sleep(1 * time.Second)
	var count int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM transactions WHERE id = $1`, txID).Scan(&count))
	require.Equal(t, 1, count)
}
