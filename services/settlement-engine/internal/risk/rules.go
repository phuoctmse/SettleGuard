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
func (s *Scorer) Score(ctx context.Context, tx TransactionInput) (RiskScore, error) {
	velocityOutcome, err := s.checkVelocityLimit(ctx, tx)
	if err != nil {
		return RiskScore{}, err
	}

	mismatchOutcome := s.checkMismatchThreshold(tx)

	blocklistOutcome, err := s.checkBlocklist(ctx, tx)
	if err != nil {
		return RiskScore{}, err
	}

	outcomes := []RuleOutcome{velocityOutcome, mismatchOutcome, blocklistOutcome}

	decision := DecisionPass
	score := 0
	for _, o := range outcomes {
		if !o.Triggered {
			continue
		}
		decision = DecisionHold
		switch o.Rule {
		case "velocity_limit":
			score += weightVelocityLimit
		case "mismatch_threshold":
			score += weightMismatchThreshold
		case "blocklist":
			score += weightBlocklist
		}
	}
	if score > maxScore {
		score = maxScore
	}

	return RiskScore{
		TransactionID: tx.ID,
		Score:         score,
		Decision:      decision,
		Outcomes:      outcomes,
	}, nil
}

// checkVelocityLimit triggers if any account in tx.AccountIDs has more
// than s.cfg.VelocityLimit transactions since
// tx.OccurredAt.Add(-s.cfg.VelocityWindow).
func (s *Scorer) checkVelocityLimit(ctx context.Context, tx TransactionInput) (RuleOutcome, error) {
	since := tx.OccurredAt.Add(-s.cfg.VelocityWindow)
	for _, accountID := range tx.AccountIDs {
		count, err := s.velocity.CountRecentTransactions(ctx, accountID, since)
		if err != nil {
			return RuleOutcome{}, fmt.Errorf("risk: checkVelocityLimit: %w", err)
		}
		if count > s.cfg.VelocityLimit {
			return RuleOutcome{
				Rule:      "velocity_limit",
				Triggered: true,
				Detail:    fmt.Sprintf("account %s has %d transactions in window, exceeds limit %d", accountID, count, s.cfg.VelocityLimit),
			}, nil
		}
	}
	return RuleOutcome{Rule: "velocity_limit"}, nil
}

// checkMismatchThreshold triggers if tx.Amount exceeds
// s.cfg.MismatchThreshold. Pure -- no I/O, so no error return.
func (s *Scorer) checkMismatchThreshold(tx TransactionInput) RuleOutcome {
	ruleOutcome := RuleOutcome{
		Rule:      "mismatch_threshold",
		Triggered: false,
		Detail:    "",
	}

	if tx.Amount > s.cfg.MismatchThreshold {
		ruleOutcome.Triggered = true
		ruleOutcome.Detail = fmt.Sprintf("amount %d exceeds threshold %d", tx.Amount, s.cfg.MismatchThreshold)
		return ruleOutcome
	}
	return ruleOutcome
}

// checkBlocklist triggers if any account in tx.AccountIDs has a
// blocklist entry.
func (s *Scorer) checkBlocklist(ctx context.Context, tx TransactionInput) (RuleOutcome, error) {
	blocked, err := s.blocklist.IsBlocked(ctx, tx.AccountIDs)
	if err != nil {
		return RuleOutcome{}, fmt.Errorf("risk: checkBlocklist: %w", err)
	}
	if blocked {
		return RuleOutcome{
			Rule:      "blocklist",
			Triggered: true,
			Detail:    "one or more accounts are on the blocklist",
		}, nil
	}
	return RuleOutcome{Rule: "blocklist"}, nil
}
