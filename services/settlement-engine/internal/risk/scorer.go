package risk

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Decision is settlement-engine's hold/pass verdict for a transaction.
type Decision string

const (
	DecisionPass Decision = "pass"
	DecisionHold Decision = "hold"
)

// Config holds the env-configurable knobs each rule checks against.
type Config struct {
	VelocityLimit     int
	VelocityWindow    time.Duration
	MismatchThreshold int64
}

// TransactionInput is the scoring input for one ledger transaction.
type TransactionInput struct {
	ID         uuid.UUID
	AccountIDs []uuid.UUID
	Amount     int64
	OccurredAt time.Time
}

// RuleOutcome records whether one rule triggered for a TransactionInput.
type RuleOutcome struct {
	Rule      string
	Triggered bool
	Detail    string
}

// RiskScore is the result of scoring one TransactionInput.
type RiskScore struct {
	TransactionID uuid.UUID
	Score         int
	Decision      Decision
	Outcomes      []RuleOutcome
}

// VelocityLimiter counts how many transactions an account has been party
// to since a point in time. Satisfied structurally by
// settlement.TransactionRepository -- this package never imports
// settlement, avoiding a risk<->settlement import cycle.
type VelocityLimiter interface {
	CountRecentTransactions(ctx context.Context, accountID uuid.UUID, since time.Time) (int, error)
}

// BlocklistChecker reports whether any of the given accounts has a
// blocklist entry. Satisfied structurally, same reasoning as
// VelocityLimiter.
type BlocklistChecker interface {
	IsBlocked(ctx context.Context, accountIDs []uuid.UUID) (bool, error)
}

// Scorer applies settlement-engine's rule-based risk checks to a
// transaction.
type Scorer struct {
	cfg       Config
	velocity  VelocityLimiter
	blocklist BlocklistChecker
}

func NewScorer(cfg Config, velocity VelocityLimiter, blocklist BlocklistChecker) *Scorer {
	return &Scorer{cfg: cfg, velocity: velocity, blocklist: blocklist}
}
