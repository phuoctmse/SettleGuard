package settlement

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// EventSettlementFinalized is the JetStream subject published once per
// settlement batch. This MVP has no settlement.held-for-review: a batch
// only ever contains transactions that already passed risk-scoring, so it
// always finalizes.
const EventSettlementFinalized = "settlement.finalized"

// Settlement is one batch of transactions grouped for payout.
type Settlement struct {
	ID               uuid.UUID
	TransactionIDs   []uuid.UUID
	TransactionCount int
	TotalAmount      int64
	CreatedAt        time.Time
}

// SettlementFinalizedPayload is the JSON body written to outbox_events for
// a settlement.finalized event.
type SettlementFinalizedPayload struct {
	SettlementID     uuid.UUID   `json:"settlement_id"`
	TransactionIDs   []uuid.UUID `json:"transaction_ids"`
	TransactionCount int         `json:"transaction_count"`
	TotalAmount      int64       `json:"total_amount"`
	FinalizedAt      time.Time   `json:"finalized_at"`
}

// SettlementRepository batches pending transactions into Settlements.
type SettlementRepository struct {
	db *sql.DB
}

func NewSettlementRepository(db *sql.DB) *SettlementRepository {
	return &SettlementRepository{db: db}
}

// RunBatch groups every currently pending_settlement transaction into one
// new Settlement, flips them to settled, and writes a settlement.finalized
// outbox event -- all within one sql.Tx. Returns (nil, nil) when nothing is
// pending.
//
// No time window is needed: pending_settlement status itself means "not
// yet batched," so nothing is skipped or double-included even under
// concurrent consumer inserts -- a transaction enters that status exactly
// once (in RecordScore) and leaves it exactly once (here). The FOR UPDATE
// lock is defensive, not load-bearing, until settlement-engine runs with
// more than one instance.
func (r *SettlementRepository) RunBatch(ctx context.Context) (*Settlement, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	rows, err := tx.QueryContext(ctx, `
		SELECT id, amount FROM transactions WHERE status = $1 FOR UPDATE
	`, StatusPendingSettlement)
	if err != nil {
		return nil, fmt.Errorf("select pending transactions: %w", err)
	}

	var (
		ids         []uuid.UUID
		totalAmount int64
	)
	for rows.Next() {
		var (
			id     uuid.UUID
			amount int64
		)
		if err := rows.Scan(&id, &amount); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan pending transaction: %w", err)
		}
		ids = append(ids, id)
		totalAmount += amount
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate pending transactions: %w", err)
	}
	rows.Close()

	if len(ids) == 0 {
		return nil, nil
	}

	settlementID := uuid.New()
	createdAt := time.Now().UTC()

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO settlements (id, transaction_count, total_amount, created_at)
		VALUES ($1, $2, $3, $4)
	`, settlementID, len(ids), totalAmount, createdAt); err != nil {
		return nil, fmt.Errorf("insert settlement: %w", err)
	}

	for _, id := range ids {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO settlement_transactions (settlement_id, transaction_id) VALUES ($1, $2)
		`, settlementID, id); err != nil {
			return nil, fmt.Errorf("insert settlement_transactions for transaction %s: %w", id, err)
		}
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE transactions SET status = $1 WHERE id = ANY($2::uuid[])
	`, StatusSettled, uuidsToStrings(ids)); err != nil {
		return nil, fmt.Errorf("mark transactions settled: %w", err)
	}

	if err := insertSettlementFinalizedOutboxEvent(ctx, tx, SettlementFinalizedPayload{
		SettlementID:     settlementID,
		TransactionIDs:   ids,
		TransactionCount: len(ids),
		TotalAmount:      totalAmount,
		FinalizedAt:      createdAt,
	}); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	return &Settlement{
		ID:               settlementID,
		TransactionIDs:   ids,
		TransactionCount: len(ids),
		TotalAmount:      totalAmount,
		CreatedAt:        createdAt,
	}, nil
}

func uuidsToStrings(ids []uuid.UUID) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = id.String()
	}
	return out
}

func insertSettlementFinalizedOutboxEvent(ctx context.Context, tx *sql.Tx, payload SettlementFinalizedPayload) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal outbox payload: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO outbox_events (id, event_type, subject, payload)
		VALUES ($1, $2, $3, $4)
	`, uuid.New(), EventSettlementFinalized, EventSettlementFinalized, body); err != nil {
		return fmt.Errorf("insert outbox event: %w", err)
	}
	return nil
}
