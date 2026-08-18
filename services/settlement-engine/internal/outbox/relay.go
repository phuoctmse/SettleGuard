package outbox

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"
)

const (
	defaultPollInterval = 500 * time.Millisecond
	defaultBatchSize    = 20
)

// Relay polls outbox_events for unpublished rows and publishes them to
// JetStream, marking each row published once its ack is received.
// At-least-once: a row that fails to publish is simply retried on the next
// poll -- there is no retry backoff or dead-letter handling yet.
type Relay struct {
	db           *sql.DB
	js           jetstream.JetStream
	pollInterval time.Duration
	batchSize    int
}

func NewRelay(db *sql.DB, js jetstream.JetStream) *Relay {
	return &Relay{
		db:           db,
		js:           js,
		pollInterval: defaultPollInterval,
		batchSize:    defaultBatchSize,
	}
}

// Run polls and publishes on a fixed interval until ctx is done, returning
// ctx.Err() at that point.
func (r *Relay) Run(ctx context.Context) error {
	ticker := time.NewTicker(r.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if _, err := r.PublishBatch(ctx); err != nil {
				log.Printf("outbox relay: publish batch: %v", err)
			}
		}
	}
}

type outboxRow struct {
	id      uuid.UUID
	subject string
	payload []byte
}

// PublishBatch publishes up to one batch of unpublished outbox events and
// returns how many were successfully published and marked. A publish or
// mark failure on one row is logged and the row is left unpublished for a
// later call; it does not abort the rest of the batch.
func (r *Relay) PublishBatch(ctx context.Context) (int, error) {
	pending, err := r.fetchUnpublished(ctx)
	if err != nil {
		return 0, err
	}

	published := 0
	for _, row := range pending {
		if _, err := r.js.Publish(ctx, row.subject, row.payload, jetstream.WithMsgID(row.id.String())); err != nil {
			log.Printf("outbox relay: publish event %s: %v", row.id, err)
			continue
		}
		if _, err := r.db.ExecContext(ctx, `
			UPDATE outbox_events SET published_at = now() WHERE id = $1
		`, row.id); err != nil {
			log.Printf("outbox relay: mark event %s published: %v", row.id, err)
			continue
		}
		published++
	}

	return published, nil
}

func (r *Relay) fetchUnpublished(ctx context.Context) ([]outboxRow, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, subject, payload FROM outbox_events
		WHERE published_at IS NULL
		ORDER BY created_at
		LIMIT $1
	`, r.batchSize)
	if err != nil {
		return nil, fmt.Errorf("query unpublished outbox events: %w", err)
	}
	defer rows.Close()

	var pending []outboxRow
	for rows.Next() {
		var row outboxRow
		if err := rows.Scan(&row.id, &row.subject, &row.payload); err != nil {
			return nil, fmt.Errorf("scan outbox event: %w", err)
		}
		pending = append(pending, row)
	}
	return pending, rows.Err()
}
