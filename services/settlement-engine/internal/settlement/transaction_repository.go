package settlement

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/phuoctmse/settleguard/settlement-engine/internal/risk"
)

var _ risk.VelocityLimiter = (*TransactionRepository)(nil)
var _ risk.BlocklistChecker = (*TransactionRepository)(nil)

// Transaction status values. PendingSettlement transactions are picked up
// by the next settlement.RunBatch; Settled is set once batched. Held is
// resolved by Approve (-> PendingSettlement, behaves exactly like a
// transaction that passed risk-scoring) or Reject (-> Rejected, terminal).
// See SETTLEMENT-05 in docs/BUSINESS_RULES.md.
const (
	StatusPendingSettlement = "pending_settlement"
	StatusHeld              = "held"
	StatusSettled           = "settled"
	StatusRejected          = "rejected"
)

// ErrTransactionNotFound is returned by Get/Approve/Reject when the
// transaction id doesn't exist.
var ErrTransactionNotFound = errors.New("transaction not found")

// ErrTransactionNotHeld is returned by Approve/Reject when the transaction
// isn't currently held.
var ErrTransactionNotHeld = errors.New("transaction is not held")

// EventTransactionRiskScored is the JetStream subject published once per
// scored transaction (both pass and hold decisions -- a full snapshot,
// matching account.updated's convention in accounts-service).
const EventTransactionRiskScored = "transaction.risk-scored"

// EventTransactionResolved is the JetStream subject published once per
// Approve/Reject call.
const EventTransactionResolved = "transaction.resolved"

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

// ResolvedPayload is the JSON body written to outbox_events for a
// transaction.resolved event.
type ResolvedPayload struct {
	TransactionID uuid.UUID `json:"transaction_id"`
	Resolution    string    `json:"resolution"` // "approved" | "rejected"
	ResolvedAt    time.Time `json:"resolved_at"`
}

// Transaction is one risk-scored transaction as read back out of storage --
// the read-side counterpart to the write-only RecordScore path below.
type Transaction struct {
	ID             uuid.UUID
	AccountIDs     []uuid.UUID
	Amount         int64
	Score          int
	Decision       string
	Status         string
	TriggeredRules []string
	ScoredAt       time.Time
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

// CountRecentTransactions counts how many transactions accountID has been
// party to (via transaction_accounts) since the given time. Satisfies
// risk.VelocityLimiter.
func (r *TransactionRepository) CountRecentTransactions(ctx context.Context, accountID uuid.UUID, since time.Time) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `
		SELECT count(*) FROM transaction_accounts WHERE account_id = $1 AND created_at >= $2
	`, accountID, since).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count recent transactions for account %s: %w", accountID, err)
	}
	return count, nil
}

// IsBlocked reports whether any of accountIDs has an 'account'-scoped
// blocklist entry. Satisfies risk.BlocklistChecker.
func (r *TransactionRepository) IsBlocked(ctx context.Context, accountIDs []uuid.UUID) (bool, error) {
	ids := make([]string, len(accountIDs))
	for i, id := range accountIDs {
		ids[i] = id.String()
	}

	var blocked bool
	err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM blocklist WHERE entity_type = 'account' AND entity_id = ANY($1::uuid[])
		)
	`, ids).Scan(&blocked)
	if err != nil {
		return false, fmt.Errorf("check blocklist: %w", err)
	}
	return blocked, nil
}

// Get returns one transaction by id, including its account_ids joined
// from transaction_accounts.
func (r *TransactionRepository) Get(ctx context.Context, id uuid.UUID) (*Transaction, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, amount, score, decision, status, triggered_rules, scored_at
		FROM transactions WHERE id = $1
	`, id)

	t, err := scanTransaction(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTransactionNotFound
		}
		return nil, fmt.Errorf("get transaction: %w", err)
	}

	accountIDs, err := r.accountIDsFor(ctx, t.ID)
	if err != nil {
		return nil, err
	}
	t.AccountIDs = accountIDs
	return t, nil
}

// ListByStatus returns every transaction currently at the given status,
// most recently scored first.
func (r *TransactionRepository) ListByStatus(ctx context.Context, status string) ([]Transaction, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, amount, score, decision, status, triggered_rules, scored_at
		FROM transactions WHERE status = $1 ORDER BY scored_at DESC
	`, status)
	if err != nil {
		return nil, fmt.Errorf("list transactions by status: %w", err)
	}
	defer rows.Close()

	var out []Transaction
	for rows.Next() {
		t, err := scanTransaction(rows)
		if err != nil {
			return nil, fmt.Errorf("scan transaction: %w", err)
		}
		accountIDs, err := r.accountIDsFor(ctx, t.ID)
		if err != nil {
			return nil, err
		}
		t.AccountIDs = accountIDs
		out = append(out, *t)
	}
	return out, rows.Err()
}

// Approve moves a held transaction back into the normal settlement flow
// (behaves identically to a transaction that passed risk-scoring outright).
// See SETTLEMENT-05 in docs/BUSINESS_RULES.md.
func (r *TransactionRepository) Approve(ctx context.Context, id uuid.UUID) (*Transaction, error) {
	return r.resolve(ctx, id, StatusPendingSettlement, "approved")
}

// Reject moves a held transaction to the terminal rejected status. See
// SETTLEMENT-05 in docs/BUSINESS_RULES.md.
func (r *TransactionRepository) Reject(ctx context.Context, id uuid.UUID) (*Transaction, error) {
	return r.resolve(ctx, id, StatusRejected, "rejected")
}

// resolve moves a held transaction to newStatus, publishing a
// transaction.resolved outbox event. Both status checks and the update
// happen under FOR UPDATE within one sql.Tx to avoid a race between two
// concurrent Approve/Reject calls on the same transaction.
func (r *TransactionRepository) resolve(ctx context.Context, id uuid.UUID, newStatus, resolution string) (*Transaction, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var currentStatus string
	err = tx.QueryRowContext(ctx, `SELECT status FROM transactions WHERE id = $1 FOR UPDATE`, id).Scan(&currentStatus)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTransactionNotFound
		}
		return nil, fmt.Errorf("lock transaction: %w", err)
	}
	if currentStatus != StatusHeld {
		return nil, ErrTransactionNotHeld
	}

	if _, err := tx.ExecContext(ctx, `UPDATE transactions SET status = $1 WHERE id = $2`, newStatus, id); err != nil {
		return nil, fmt.Errorf("update transaction status: %w", err)
	}

	resolvedAt := time.Now().UTC()
	body, err := json.Marshal(ResolvedPayload{TransactionID: id, Resolution: resolution, ResolvedAt: resolvedAt})
	if err != nil {
		return nil, fmt.Errorf("marshal outbox payload: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO outbox_events (id, event_type, subject, payload) VALUES ($1, $2, $3, $4)
	`, uuid.New(), EventTransactionResolved, EventTransactionResolved, body); err != nil {
		return nil, fmt.Errorf("insert outbox event: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	return r.Get(ctx, id)
}

// rowScanner is satisfied by both *sql.Row and *sql.Rows, letting
// scanTransaction serve Get (single row) and ListByStatus (row cursor)
// with one implementation.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanTransaction(row rowScanner) (*Transaction, error) {
	var t Transaction
	var triggeredRules []string
	if err := row.Scan(&t.ID, &t.Amount, &t.Score, &t.Decision, &t.Status, &triggeredRules, &t.ScoredAt); err != nil {
		return nil, err
	}
	t.TriggeredRules = triggeredRules
	return &t, nil
}

func (r *TransactionRepository) accountIDsFor(ctx context.Context, transactionID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT account_id FROM transaction_accounts WHERE transaction_id = $1
	`, transactionID)
	if err != nil {
		return nil, fmt.Errorf("list account_ids for transaction %s: %w", transactionID, err)
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan account_id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
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
