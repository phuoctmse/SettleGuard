package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/phuoctmse/settleguard/accounts-service/internal/account"
	"github.com/phuoctmse/settleguard/accounts-service/internal/broker"
	"github.com/phuoctmse/settleguard/accounts-service/internal/ledgerevent"
)

// DurableName is the JetStream durable consumer name accounts-service uses
// on the LEDGER_EVENTS stream. Changing FilterSubject/AckPolicy for an
// existing durable of this name later requires deleting and recreating it
// server-side -- CreateOrUpdateConsumer reconciles compatible changes but
// not every field is mutable in place.
const DurableName = "accounts-service-balance"

const applyTimeout = 10 * time.Second

// Consumer applies balance updates from ledger-service's
// ledger.entry-recorded events to local Account balances.
type Consumer struct {
	accounts *account.AccountRepository
}

func New(accounts *account.AccountRepository) *Consumer {
	return &Consumer{accounts: accounts}
}

// Start creates (or attaches to) the durable JetStream consumer and begins
// consuming. The caller must Stop() the returned ConsumeContext on
// shutdown.
func (c *Consumer) Start(ctx context.Context, js jetstream.JetStream) (jetstream.ConsumeContext, error) {
	cons, err := js.CreateOrUpdateConsumer(ctx, broker.LedgerEventsStream, jetstream.ConsumerConfig{
		Durable:       DurableName,
		FilterSubject: ledgerevent.EventLedgerEntryRecorded,
		AckPolicy:     jetstream.AckExplicitPolicy,
		DeliverPolicy: jetstream.DeliverAllPolicy,
	})
	if err != nil {
		return nil, fmt.Errorf("create/attach consumer %s: %w", DurableName, err)
	}

	return cons.Consume(c.handleMessage)
}

func (c *Consumer) handleMessage(msg jetstream.Msg) {
	var payload ledgerevent.OutboxPayload
	if err := json.Unmarshal(msg.Data(), &payload); err != nil {
		log.Printf("consumer: malformed payload, terminating message: %v", err)
		_ = msg.Term()
		return
	}

	deltas, err := ledgerevent.BalanceDeltas(payload.Entries)
	if err != nil {
		log.Printf("consumer: %v, terminating message", err)
		_ = msg.Term()
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), applyTimeout)
	defer cancel()

	if err := c.accounts.ApplyLedgerTransaction(ctx, payload.TransactionID, deltas); err != nil {
		log.Printf("consumer: apply transaction %s: %v", payload.TransactionID, err)
		_ = msg.Nak()
		return
	}

	if err := msg.Ack(); err != nil {
		log.Printf("consumer: ack transaction %s: %v", payload.TransactionID, err)
	}
}
