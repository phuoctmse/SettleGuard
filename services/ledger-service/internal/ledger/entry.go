package ledger

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type Direction string

const (
	Debit  Direction = "debit"
	Credit Direction = "credit"
)

type Entry struct {
	ID            uuid.UUID
	TransactionID uuid.UUID
	AccountID     uuid.UUID
	Direction     Direction
	Amount        int64
	Reason        string
	CreatedAt     time.Time
}

var (
	ErrUnbalancedTransaction = errors.New("ledger: transaction entries do not balance")
	ErrInvalidDirection      = errors.New("ledger: entry direction must be debit or credit")
	ErrInvalidAmount         = errors.New("ledger: entry amount must be positive")
	ErrNoEntries             = errors.New("ledger: transaction must have at least one entry")
	ErrAmountTooLarge        = errors.New("ledger: entry amount exceeds the per-entry ceiling")
	ErrTooManyEntries        = errors.New("ledger: transaction has too many entries")
)

// Bounds on a single transaction. Together they are what make the plain
// int64 sums in ValidateBalanced safe: the largest reachable total is
// MaxEntriesPerTransaction * MaxEntryAmount = 10^18, well inside int64's
// 9.22*10^18. Raise either one and the product must still fit, or a caller
// can wrap the sums back to equality and post an unbalanced transaction
// (see TestValidateBalanced_RejectsOverflowingSums).
const (
	// MaxEntryAmount is the largest single entry, in VND dong (MONEY-01).
	// 10^15 dong is roughly forty billion USD -- no single obligation
	// SettleGuard settles should come near it.
	MaxEntryAmount int64 = 1_000_000_000_000_000
	// MaxEntriesPerTransaction bounds the legs of one transaction. A
	// batch payout is one debit fanning out to many credits; 1000 legs is
	// generous for that and still keeps the sum bound above.
	MaxEntriesPerTransaction = 1000
)

func ValidateBalanced(entries []Entry) error {
	if len(entries) == 0 {
		return ErrNoEntries
	}
	if len(entries) > MaxEntriesPerTransaction {
		return ErrTooManyEntries
	}

	var debitTotal, creditTotal int64
	for _, e := range entries {
		if e.Amount <= 0 {
			return ErrInvalidAmount
		}
		if e.Amount > MaxEntryAmount {
			return ErrAmountTooLarge
		}
		switch e.Direction {
		case Debit:
			debitTotal += e.Amount
		case Credit:
			creditTotal += e.Amount
		default:
			return ErrInvalidDirection
		}
	}

	if debitTotal != creditTotal {
		return ErrUnbalancedTransaction
	}

	return nil
}
