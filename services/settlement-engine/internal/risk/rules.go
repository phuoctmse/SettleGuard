package risk

import (
	"context"
	"fmt"
)

// Fixed audit-score weights per rule, summed and capped at maxScore.
// Decision (hold/pass) is OR-based across rules and does not depend on
// these weights -- Score is an informational/audit field only.
const (
	weightVelocityLimit     = 40
	weightMismatchThreshold = 30
	weightBlocklist         = 100
	maxScore                = 100
)

// Score runs all three rules against tx and combines their outcomes into
// a RiskScore. Decision is Hold if any rule triggered; Score is the
// triggered rules' weights summed and capped at maxScore.
//
// TODO(you): implement. Call checkVelocityLimit, checkMismatchThreshold,
// checkBlocklist; on any rule error, return it immediately (wrapped with
// fmt.Errorf("...: %w", err)) rather than swallowing it -- see
// accounts-service's consumer.handleMessage for the same
// don't-swallow-errors convention applied to a different layer.
func (s *Scorer) Score(ctx context.Context, tx TransactionInput) (RiskScore, error) {
	return RiskScore{}, fmt.Errorf("risk: Score not implemented")
}

// checkVelocityLimit triggers if any account in tx.AccountIDs has more
// than s.cfg.VelocityLimit transactions since
// tx.OccurredAt.Add(-s.cfg.VelocityWindow).
//
// TODO(you): implement.
func (s *Scorer) checkVelocityLimit(ctx context.Context, tx TransactionInput) (RuleOutcome, error) {
	return RuleOutcome{}, fmt.Errorf("risk: checkVelocityLimit not implemented")
}

// checkMismatchThreshold triggers if tx.Amount exceeds
// s.cfg.MismatchThreshold. Pure -- no I/O, so no error return.
//
// TODO(you): implement.
func (s *Scorer) checkMismatchThreshold(tx TransactionInput) RuleOutcome {
	return RuleOutcome{}
}

// checkBlocklist triggers if any account in tx.AccountIDs has a
// blocklist entry.
//
// TODO(you): implement.
func (s *Scorer) checkBlocklist(ctx context.Context, tx TransactionInput) (RuleOutcome, error) {
	return RuleOutcome{}, fmt.Errorf("risk: checkBlocklist not implemented")
}
