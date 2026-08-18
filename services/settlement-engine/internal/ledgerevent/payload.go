package ledgerevent

import (
	"time"

	"github.com/google/uuid"
)

// EventLedgerEntryRecorded is the JetStream subject ledger-service
// publishes to (services/ledger-service/internal/ledger/outbox.go).
// Mirrored here as a literal constant since settlement-engine, a separate
// Go module, cannot import ledger-service's package.
const EventLedgerEntryRecorded = "ledger.entry-recorded"

// OutboxPayload structurally mirrors ledger-service's OutboxPayload JSON
// shape (one event per balanced transaction).
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

// TotalAmount returns the sum of debit-side entry amounts, which equals
// the credit-side sum per ledger-service's ValidateBalanced invariant --
// either side is an equivalent measure of the transaction's total amount.
func TotalAmount(entries []OutboxPayloadEntry) int64 {
	var total int64
	for _, e := range entries {
		if e.Direction == "debit" {
			total += e.Amount
		}
	}
	return total
}

// OccurredAt returns the earliest entry's CreatedAt -- ledger-service's own
// record of when the transaction actually happened, as opposed to whenever
// a consumer gets around to processing the event. This distinction matters
// for consumers computing time-windowed rules (e.g. settlement-engine's
// velocity limit): using processing time instead would make a fast replay
// of old events (consumer.DeliverAllPolicy on first startup, or catch-up
// after downtime) collapse days of real activity into one scoring window.
func OccurredAt(entries []OutboxPayloadEntry) time.Time {
	var earliest time.Time
	for i, e := range entries {
		if i == 0 || e.CreatedAt.Before(earliest) {
			earliest = e.CreatedAt
		}
	}
	return earliest
}

// AccountIDs returns the distinct account IDs touched across entries, in
// first-seen order.
func AccountIDs(entries []OutboxPayloadEntry) []uuid.UUID {
	seen := make(map[uuid.UUID]struct{}, len(entries))
	var ids []uuid.UUID
	for _, e := range entries {
		if _, ok := seen[e.AccountID]; ok {
			continue
		}
		seen[e.AccountID] = struct{}{}
		ids = append(ids, e.AccountID)
	}
	return ids
}
