package ledger

import (
	"time"

	"github.com/google/uuid"
)

// EventLedgerEntryRecorded is the event name/subject published once per
// balanced transaction recorded via Repository.InsertTransaction.
const EventLedgerEntryRecorded = "ledger.entry-recorded"

// OutboxPayload is the JSON body written to outbox_events (and, once
// relayed, published to NATS JetStream) for a ledger.entry-recorded event.
// One event covers a whole transaction, not a single entry, since that's
// InsertTransaction's natural transactional boundary and the unit
// downstream risk scoring cares about.
type OutboxPayload struct {
	TransactionID uuid.UUID            `json:"transaction_id"`
	Entries       []OutboxPayloadEntry `json:"entries"`
}

type OutboxPayloadEntry struct {
	ID        uuid.UUID `json:"id"`
	AccountID uuid.UUID `json:"account_id"`
	Direction string    `json:"direction"`
	Amount    int64     `json:"amount"`
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"created_at"`
}

func newOutboxPayload(transactionID uuid.UUID, entries []Entry) OutboxPayload {
	payloadEntries := make([]OutboxPayloadEntry, len(entries))
	for i, e := range entries {
		payloadEntries[i] = OutboxPayloadEntry{
			ID:        e.ID,
			AccountID: e.AccountID,
			Direction: string(e.Direction),
			Amount:    e.Amount,
			Reason:    e.Reason,
			CreatedAt: e.CreatedAt,
		}
	}
	return OutboxPayload{TransactionID: transactionID, Entries: payloadEntries}
}
