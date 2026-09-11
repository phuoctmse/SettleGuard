package ledgerevent

import (
	"fmt"
	"math"
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
//
// This value feeds the mismatch_threshold rule, the backstop against
// oversized transactions, so it deliberately does not trust the publisher:
// a sum that would wrap int64, a non-positive amount, or a direction it
// cannot interpret all return an error for the consumer to terminate on.
// Any of those, if tolerated, would collapse to a small or zero total that
// sails under the threshold.
func TotalAmount(entries []OutboxPayloadEntry) (int64, error) {
	var total int64
	for _, e := range entries {
		if e.Amount <= 0 {
			return 0, fmt.Errorf("ledgerevent: entry amount %d is not positive", e.Amount)
		}
		switch e.Direction {
		case "debit":
			if total > math.MaxInt64-e.Amount {
				return 0, fmt.Errorf("ledgerevent: debit total overflows int64")
			}
			total += e.Amount
		case "credit":
			// Not summed: the credit side equals the debit side by LEDGER-01.
		default:
			return 0, fmt.Errorf("ledgerevent: unrecognized entry direction %q", e.Direction)
		}
	}
	return total, nil
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
