package ledgerevent

import (
	"fmt"
	"math"
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
		// A non-positive amount would silently invert the direction's
		// meaning; the ledger never emits one, so treat it as malformed.
		if e.Amount <= 0 {
			return nil, fmt.Errorf("ledgerevent: entry amount %d is not positive", e.Amount)
		}
		cur := deltas[e.AccountID]
		switch e.Direction {
		case "credit":
			if cur > math.MaxInt64-e.Amount {
				return nil, fmt.Errorf("ledgerevent: balance delta for account %s overflows int64", e.AccountID)
			}
			deltas[e.AccountID] = cur + e.Amount
		case "debit":
			if cur < math.MinInt64+e.Amount {
				return nil, fmt.Errorf("ledgerevent: balance delta for account %s overflows int64", e.AccountID)
			}
			deltas[e.AccountID] = cur - e.Amount
		default:
			return nil, fmt.Errorf("ledgerevent: unrecognized entry direction %q", e.Direction)
		}
	}
	return deltas, nil
}
