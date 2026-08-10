package account

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// EventAccountUpdated is the event name/subject published once per
// meaningful change to an Account: creation, status update, or a balance
// update applied via ApplyLedgerTransaction.
const EventAccountUpdated = "account.updated"

// OutboxPayload is the JSON body written to outbox_events (and, once
// relayed, published to NATS JetStream) for an account.updated event. It
// carries the account's full post-write state -- a snapshot, not a diff --
// so downstream consumers don't need to reconstruct state from partial
// updates.
type OutboxPayload struct {
	AccountID   uuid.UUID `json:"account_id"`
	ClientID    uuid.UUID `json:"client_id"`
	ExternalRef string    `json:"external_ref"`
	Status      string    `json:"status"`
	Balance     int64     `json:"balance"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func newOutboxPayload(a Account) OutboxPayload {
	return OutboxPayload{
		AccountID:   a.ID,
		ClientID:    a.ClientID,
		ExternalRef: a.ExternalRef,
		Status:      string(a.Status),
		Balance:     a.Balance,
		UpdatedAt:   a.UpdatedAt,
	}
}

// insertOutboxEvent writes one account.updated outbox row for a, within
// tx, so it's atomic with whatever mutation to a just happened.
func insertOutboxEvent(ctx context.Context, tx *sql.Tx, a Account) error {
	payload, err := json.Marshal(newOutboxPayload(a))
	if err != nil {
		return fmt.Errorf("marshal outbox payload: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO outbox_events (id, event_type, subject, payload)
		VALUES ($1, $2, $3, $4)
	`, uuid.New(), EventAccountUpdated, EventAccountUpdated, payload); err != nil {
		return fmt.Errorf("insert outbox event: %w", err)
	}
	return nil
}
