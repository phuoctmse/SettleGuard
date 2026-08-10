package consumer_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/require"

	"github.com/phuoctmse/settleguard/accounts-service/internal/account"
	"github.com/phuoctmse/settleguard/accounts-service/internal/broker"
	"github.com/phuoctmse/settleguard/accounts-service/internal/consumer"
	"github.com/phuoctmse/settleguard/accounts-service/internal/ledgerevent"
	"github.com/phuoctmse/settleguard/accounts-service/internal/testutil"
)

func TestConsumer_AppliesBalanceOnPublish(t *testing.T) {
	db := testutil.NewTestDB(t)
	_, js := testutil.NewTestNATS(t)
	ctx := context.Background()
	require.NoError(t, broker.EnsureStream(ctx, js, jetstream.StreamConfig{
		Name:     broker.LedgerEventsStream,
		Subjects: []string{"ledger.>"},
		Storage:  jetstream.FileStorage,
	}))

	accounts := account.NewAccountRepository(db)
	clients := account.NewClientRepository(db)

	c := consumer.New(accounts)
	consumeCtx, err := c.Start(ctx, js)
	require.NoError(t, err)
	t.Cleanup(consumeCtx.Stop)

	client, err := clients.Create(ctx, "Acme Corp")
	require.NoError(t, err)
	acc, err := accounts.Create(ctx, client.ID, "ext-1")
	require.NoError(t, err)

	payload, err := json.Marshal(ledgerevent.OutboxPayload{
		TransactionID: uuid.New(),
		Entries: []ledgerevent.OutboxPayloadEntry{
			{ID: uuid.New(), AccountID: acc.ID, Direction: "credit", Amount: 750, CreatedAt: time.Now()},
		},
	})
	require.NoError(t, err)

	_, err = js.Publish(ctx, ledgerevent.EventLedgerEntryRecorded, payload)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		fetched, err := accounts.Get(ctx, acc.ID)
		return err == nil && fetched.Balance == 750
	}, 5*time.Second, 100*time.Millisecond, "balance should reflect the published entry")
}

func TestConsumer_IdempotentAcrossRedelivery(t *testing.T) {
	db := testutil.NewTestDB(t)
	_, js := testutil.NewTestNATS(t)
	ctx := context.Background()
	require.NoError(t, broker.EnsureStream(ctx, js, jetstream.StreamConfig{
		Name:     broker.LedgerEventsStream,
		Subjects: []string{"ledger.>"},
		Storage:  jetstream.FileStorage,
	}))

	accounts := account.NewAccountRepository(db)
	clients := account.NewClientRepository(db)

	c := consumer.New(accounts)
	consumeCtx, err := c.Start(ctx, js)
	require.NoError(t, err)
	t.Cleanup(consumeCtx.Stop)

	client, err := clients.Create(ctx, "Acme Corp")
	require.NoError(t, err)
	acc, err := accounts.Create(ctx, client.ID, "ext-1")
	require.NoError(t, err)

	txID := uuid.New()
	payload, err := json.Marshal(ledgerevent.OutboxPayload{
		TransactionID: txID,
		Entries: []ledgerevent.OutboxPayloadEntry{
			{ID: uuid.New(), AccountID: acc.ID, Direction: "credit", Amount: 300, CreatedAt: time.Now()},
		},
	})
	require.NoError(t, err)

	// Publish the same transaction_id twice as two separate publishes
	// (deliberately not using WithMsgID, so JetStream's own dedupe window
	// doesn't hide this from ever reaching our handler -- this proves our
	// dedup table works, not NATS's broker-side dedupe).
	_, err = js.Publish(ctx, ledgerevent.EventLedgerEntryRecorded, payload)
	require.NoError(t, err)
	_, err = js.Publish(ctx, ledgerevent.EventLedgerEntryRecorded, payload)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		fetched, err := accounts.Get(ctx, acc.ID)
		return err == nil && fetched.Balance == 300
	}, 5*time.Second, 100*time.Millisecond, "balance should reflect one application")

	// Give the second (redelivered) message plenty of time to have been
	// processed too, then confirm the balance hasn't moved further.
	time.Sleep(1 * time.Second)
	fetched, err := accounts.Get(ctx, acc.ID)
	require.NoError(t, err)
	require.Equal(t, int64(300), fetched.Balance, "the second publish of the same transaction_id must not double-apply")
}

func TestConsumer_SkipsMessageReferencingUnknownAccountWithoutWedging(t *testing.T) {
	db := testutil.NewTestDB(t)
	_, js := testutil.NewTestNATS(t)
	ctx := context.Background()
	require.NoError(t, broker.EnsureStream(ctx, js, jetstream.StreamConfig{
		Name:     broker.LedgerEventsStream,
		Subjects: []string{"ledger.>"},
		Storage:  jetstream.FileStorage,
	}))

	accounts := account.NewAccountRepository(db)
	clients := account.NewClientRepository(db)

	c := consumer.New(accounts)
	consumeCtx, err := c.Start(ctx, js)
	require.NoError(t, err)
	t.Cleanup(consumeCtx.Stop)

	client, err := clients.Create(ctx, "Acme Corp")
	require.NoError(t, err)
	acc, err := accounts.Create(ctx, client.ID, "ext-1")
	require.NoError(t, err)

	unknownPayload, err := json.Marshal(ledgerevent.OutboxPayload{
		TransactionID: uuid.New(),
		Entries: []ledgerevent.OutboxPayloadEntry{
			{ID: uuid.New(), AccountID: uuid.New(), Direction: "credit", Amount: 100, CreatedAt: time.Now()},
		},
	})
	require.NoError(t, err)
	_, err = js.Publish(ctx, ledgerevent.EventLedgerEntryRecorded, unknownPayload)
	require.NoError(t, err)

	validPayload, err := json.Marshal(ledgerevent.OutboxPayload{
		TransactionID: uuid.New(),
		Entries: []ledgerevent.OutboxPayloadEntry{
			{ID: uuid.New(), AccountID: acc.ID, Direction: "credit", Amount: 200, CreatedAt: time.Now()},
		},
	})
	require.NoError(t, err)
	_, err = js.Publish(ctx, ledgerevent.EventLedgerEntryRecorded, validPayload)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		fetched, err := accounts.Get(ctx, acc.ID)
		return err == nil && fetched.Balance == 200
	}, 5*time.Second, 100*time.Millisecond, "the message after the unknown-account one should still be processed")
}

func TestConsumer_TerminatesMalformedPayloadWithoutWedging(t *testing.T) {
	db := testutil.NewTestDB(t)
	nc, js := testutil.NewTestNATS(t)
	ctx := context.Background()
	require.NoError(t, broker.EnsureStream(ctx, js, jetstream.StreamConfig{
		Name:     broker.LedgerEventsStream,
		Subjects: []string{"ledger.>"},
		Storage:  jetstream.FileStorage,
	}))

	accounts := account.NewAccountRepository(db)
	clients := account.NewClientRepository(db)

	c := consumer.New(accounts)
	consumeCtx, err := c.Start(ctx, js)
	require.NoError(t, err)
	t.Cleanup(consumeCtx.Stop)

	require.NoError(t, nc.Publish(ledgerevent.EventLedgerEntryRecorded, []byte("not json")))

	client, err := clients.Create(ctx, "Acme Corp")
	require.NoError(t, err)
	acc, err := accounts.Create(ctx, client.ID, "ext-1")
	require.NoError(t, err)

	validPayload, err := json.Marshal(ledgerevent.OutboxPayload{
		TransactionID: uuid.New(),
		Entries: []ledgerevent.OutboxPayloadEntry{
			{ID: uuid.New(), AccountID: acc.ID, Direction: "credit", Amount: 400, CreatedAt: time.Now()},
		},
	})
	require.NoError(t, err)
	_, err = js.Publish(ctx, ledgerevent.EventLedgerEntryRecorded, validPayload)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		fetched, err := accounts.Get(ctx, acc.ID)
		return err == nil && fetched.Balance == 400
	}, 5*time.Second, 100*time.Millisecond, "the malformed message must not block subsequent valid messages")
}
