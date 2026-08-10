package broker

import (
	"context"
	"fmt"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// LedgerEventsStream is the JetStream stream holding all ledger-service
// events, keyed off the "ledger.>" subject wildcard so new event types
// don't need a stream migration.
const LedgerEventsStream = "LEDGER_EVENTS"

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

// EnsureStream creates LedgerEventsStream if it doesn't exist, or updates
// it in place if it does. Safe to call on every startup.
func EnsureStream(ctx context.Context, js jetstream.JetStream) error {
	_, err := js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:     LedgerEventsStream,
		Subjects: []string{"ledger.>"},
		Storage:  jetstream.FileStorage,
	})
	if err != nil {
		return fmt.Errorf("ensure stream %s: %w", LedgerEventsStream, err)
	}
	return nil
}
