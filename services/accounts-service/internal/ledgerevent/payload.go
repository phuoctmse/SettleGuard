package ledgerevent

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// EventLedgerEntryRecorded is the JetStream subject ledger-service
// publishes to (services/ledger-service/internal/ledger/outbox.go).
// Mirrored here as a literal constant since accounts-service, a separate
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

// BalanceDeltas computes, per account_id, the net balance change implied
// by entries: credit increases balance (+amount), debit decreases it
// (-amount). Returns an error for any entry with an unrecognized
// direction — the caller should treat this as a permanent, non-retryable
// failure.
func BalanceDeltas(entries []OutboxPayloadEntry) (map[uuid.UUID]int64, error) {
	deltas := make(map[uuid.UUID]int64, len(entries))
	for _, e := range entries {
		switch e.Direction {
		case "credit":
			deltas[e.AccountID] += e.Amount
		case "debit":
			deltas[e.AccountID] -= e.Amount
		default:
			return nil, fmt.Errorf("ledgerevent: unrecognized entry direction %q", e.Direction)
		}
	}
	return deltas, nil
}
