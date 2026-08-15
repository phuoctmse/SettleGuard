package broker

import (
	"context"
	"fmt"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// LedgerEventsStream is the JetStream stream holding ledger-service's
// events. settlement-engine does not own this stream (ledger-service does)
// but ensures it exists defensively on its own startup too, since
// CreateOrUpdateStream is idempotent — this removes a startup-ordering
// dependency between the two services in local dev.
const LedgerEventsStream = "LEDGER_EVENTS"

// SettlementEventsStream is the JetStream stream settlement-engine owns,
// holding its own published events (transaction.risk-scored,
// settlement.finalized).
const SettlementEventsStream = "SETTLEMENT_EVENTS"

// Connect dials the NATS server at url and returns both the raw connection
// and a JetStream context built on top of it.
func Connect(url string) (*nats.Conn, jetstream.JetStream, error) {
	conn, err := nats.Connect(url)
	if err != nil {
		return nil, nil, fmt.Errorf("connect nats: %w", err)
	}

	js, err := jetstream.New(conn)
	if err != nil {
		conn.Close()
		return nil, nil, fmt.Errorf("create jetstream context: %w", err)
	}

	return conn, js, nil
}

// EnsureStream creates the stream described by cfg if it doesn't exist, or
// updates it in place if it does. Safe to call on every startup.
func EnsureStream(ctx context.Context, js jetstream.JetStream, cfg jetstream.StreamConfig) error {
	if _, err := js.CreateOrUpdateStream(ctx, cfg); err != nil {
		return fmt.Errorf("ensure stream %s: %w", cfg.Name, err)
	}
	return nil
}
