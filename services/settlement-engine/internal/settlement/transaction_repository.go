package settlement

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/phuoctmse/settleguard/settlement-engine/internal/risk"
)

// Transaction status values. Hold is a terminal-for-now state (no
// resolution path yet in this MVP); PendingSettlement transactions are
// picked up by the next settlement.RunBatch; Settled is set once batched.
const (
	StatusPendingSettlement = "pending_settlement"
	StatusHeld              = "held"
	StatusSettled           = "settled"
)

// EventTransactionRiskScored is the JetStream subject published once per
// scored transaction (both pass and hold decisions -- a full snapshot,
// matching account.updated's convention in accounts-service).
const EventTransactionRiskScored = "transaction.risk-scored"

// RiskScoredPayload is the JSON body written to outbox_events (and, once
// relayed, published to NATS JetStream) for a transaction.risk-scored event.
type RiskScoredPayload struct {
	TransactionID  uuid.UUID   `json:"transaction_id"`
	AccountIDs     []uuid.UUID `json:"account_ids"`
	Amount         int64       `json:"amount"`
	Score          int         `json:"score"`
	Decision       string      `json:"decision"`
	TriggeredRules []string    `json:"triggered_rules"`
	ScoredAt       time.Time   `json:"scored_at"`
}

// TransactionRepository persists risk-scored transactions. From Task 9 it
// also answers the DB-facing queries risk.Scorer needs
// (CountRecentTransactions, IsBlocked) -- satisfying risk.VelocityLimiter
// and risk.BlocklistChecker structurally, without this package importing risk
// the other way.
type TransactionRepository struct {
	db *sql.DB
}

func NewTransactionRepository(db *sql.DB) *TransactionRepository {
	return &TransactionRepository{db: db}
}

// RecordScore idempotently persists one transaction's risk.RiskScore.
// Dedups on score.TransactionID via processed_ledger_transactions -- the
// same pattern AccountRepository.ApplyLedgerTransaction uses in
// accounts-service to guard against JetStream's at-least-once redelivery:
// on conflict, this is a no-op. On first insert, writes the transactions
// row, one transaction_accounts row per touched account, and a
// transaction.risk-scored outbox row, all within one sql.Tx.
func (r *TransactionRepository) RecordScore(ctx context.Context, accountIDs []uuid.UUID, amount int64, score risk.RiskScore) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx, `
		INSERT INTO processed_ledger_transactions (transaction_id) VALUES ($1)
		ON CONFLICT (transaction_id) DO NOTHING
	`, score.TransactionID)
	if err != nil {
		return fmt.Errorf("mark transaction processed: %w", err)
	}
	inserted, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("check processed-transaction insert: %w", err)
	}
	if inserted == 0 {
		return tx.Commit()
	}

	status := statusForDecision(score.Decision)
	triggeredRules := triggeredRuleNames(score.Outcomes)
	scoredAt := time.Now().UTC()

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO transactions (id, amount, score, decision, status, triggered_rules, scored_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, score.TransactionID, amount, score.Score, string(score.Decision), status, triggeredRules, scoredAt); err != nil {
		return fmt.Errorf("insert transaction: %w", err)
	}

	for _, accountID := range accountIDs {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO transaction_accounts (transaction_id, account_id) VALUES ($1, $2)
		`, score.TransactionID, accountID); err != nil {
			return fmt.Errorf("insert transaction_accounts for account %s: %w", accountID, err)
		}
	}

	if err := insertRiskScoredOutboxEvent(ctx, tx, RiskScoredPayload{
		TransactionID:  score.TransactionID,
		AccountIDs:     accountIDs,
		Amount:         amount,
		Score:          score.Score,
		Decision:       string(score.Decision),
		TriggeredRules: triggeredRules,
		ScoredAt:       scoredAt,
	}); err != nil {
		return err
	}

	return tx.Commit()
}

func statusForDecision(d risk.Decision) string {
	if d == risk.DecisionHold {
		return StatusHeld
	}
	return StatusPendingSettlement
}

// triggeredRuleNames always returns a non-nil slice (possibly empty) --
// the triggered_rules column is NOT NULL.
func triggeredRuleNames(outcomes []risk.RuleOutcome) []string {
	rules := make([]string, 0, len(outcomes))
	for _, o := range outcomes {
		if o.Triggered {
			rules = append(rules, o.Rule)
		}
	}
	return rules
}

func insertRiskScoredOutboxEvent(ctx context.Context, tx *sql.Tx, payload RiskScoredPayload) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal outbox payload: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO outbox_events (id, event_type, subject, payload)
		VALUES ($1, $2, $3, $4)
	`, uuid.New(), EventTransactionRiskScored, EventTransactionRiskScored, body); err != nil {
		return fmt.Errorf("insert outbox event: %w", err)
	}
	return nil
}
