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
)

func ValidateBalanced(entries []Entry) error {
	if len(entries) == 0 {
		return ErrNoEntries
	}

	var debitTotal, creditTotal int64
	for _, e := range entries {
		if e.Amount <= 0 {
			return ErrInvalidAmount
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
