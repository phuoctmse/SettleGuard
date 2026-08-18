package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/phuoctmse/settleguard/settlement-engine/internal/broker"
	"github.com/phuoctmse/settleguard/settlement-engine/internal/ledgerevent"
	"github.com/phuoctmse/settleguard/settlement-engine/internal/risk"
	"github.com/phuoctmse/settleguard/settlement-engine/internal/settlement"
)

// DurableName is the JetStream durable consumer name settlement-engine
// uses on the LEDGER_EVENTS stream. Changing FilterSubject/AckPolicy for
// an existing durable of this name later requires deleting and
// recreating it server-side -- CreateOrUpdateConsumer reconciles
// compatible changes but not every field is mutable in place.
const DurableName = "settlement-engine-risk-scoring"

const scoreTimeout = 10 * time.Second

// Consumer scores each ledger.entry-recorded event and persists the
// result via TransactionRepository.RecordScore.
type Consumer struct {
	scorer       *risk.Scorer
	transactions *settlement.TransactionRepository
}

func New(scorer *risk.Scorer, transactions *settlement.TransactionRepository) *Consumer {
	return &Consumer{scorer: scorer, transactions: transactions}
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

	accountIDs := ledgerevent.AccountIDs(payload.Entries)
	amount := ledgerevent.TotalAmount(payload.Entries)

	ctx, cancel := context.WithTimeout(context.Background(), scoreTimeout)
	defer cancel()

	score, err := c.scorer.Score(ctx, risk.TransactionInput{
		ID:         payload.TransactionID,
		AccountIDs: accountIDs,
		Amount:     amount,
		OccurredAt: ledgerevent.OccurredAt(payload.Entries),
	})
	if err != nil {
		log.Printf("consumer: score transaction %s: %v", payload.TransactionID, err)
		_ = msg.Nak()
		return
	}

	// A held transaction is a successful, expected outcome of scoring --
	// not an error -- so it still gets Ack()'d here like a passing one.
	if err := c.transactions.RecordScore(ctx, accountIDs, amount, score); err != nil {
		log.Printf("consumer: record score for transaction %s: %v", payload.TransactionID, err)
		_ = msg.Nak()
		return
	}

	if err := msg.Ack(); err != nil {
		log.Printf("consumer: ack transaction %s: %v", payload.TransactionID, err)
	}
}
