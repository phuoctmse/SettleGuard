package ledger

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) InsertTransaction(ctx context.Context, transactionID uuid.UUID, entries []Entry) ([]Entry, error) {
	if err := ValidateBalanced(entries); err != nil {
		return nil, err
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	// Rollback error is ignored: a no-op if Commit already succeeded, and if it
	// fails after a real error, the original error is what matters to the caller.
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO ledger_entries (id,  transaction_id, account_id,
		direction, amount, reason)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING created_at
	`)
	if err != nil {
		return nil, fmt.Errorf("prepare insert: %w", err)
	}
	defer stmt.Close()

	inserted := make([]Entry, len(entries))
	for i, e := range entries {
		e.ID = uuid.New()
		e.TransactionID = transactionID
		if err := stmt.QueryRowContext(ctx, e.ID, e.TransactionID, e.AccountID, string(e.Direction), e.Amount, e.Reason).Scan(&e.CreatedAt); err != nil {
			return nil, fmt.Errorf("insert entry: %w", err)
		}
		inserted[i] = e
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	return inserted, nil
}

func (r *Repository) ListByAccount(ctx context.Context, accountID uuid.UUID) ([]Entry, error) {
	return r.query(ctx, `
		SELECT id, transaction_id, account_id, direction, amount, reason, created_at
		FROM ledger_entries WHERE account_id = $1 ORDER BY created_at
	`, accountID)
}

func (r *Repository) ListByTransaction(ctx context.Context, transactionID uuid.UUID) ([]Entry, error) {
	return r.query(ctx, `
		SELECT id, transaction_id, account_id, direction, amount, reason, created_at
		FROM ledger_entries WHERE transaction_id = $1 ORDER BY created_at
	`, transactionID)
}

func (r *Repository) query(ctx context.Context, q string, arg uuid.UUID) ([]Entry, error) {
	rows, err := r.db.QueryContext(ctx, q, arg)
	if err != nil {
		return nil, fmt.Errorf("query entries: %w", err)
	}
	defer rows.Close()

	var entries []Entry
	for rows.Next() {
		var e Entry
		var direction string
		if err := rows.Scan(&e.ID, &e.TransactionID, &e.AccountID, &direction, &e.Amount, &e.Reason, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan entry: %w", err)
		}
		e.Direction = Direction(direction)
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
