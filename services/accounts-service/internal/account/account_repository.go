package account

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"

	"github.com/google/uuid"
)

// AccountRepository stutters with the package name by design, matching
// ClientRepository's naming, and is an exact interface name mandated by the
// MVP plan, consumed unchanged by Task 8.
//
//nolint:revive
type AccountRepository struct {
	db *sql.DB
}

func NewAccountRepository(db *sql.DB) *AccountRepository {
	return &AccountRepository{db: db}
}

// Create inserts a new Account under clientID, after checking that the
// parent ClientBusiness exists and is not suspended (see CanCreateAccount).
func (r *AccountRepository) Create(ctx context.Context, clientID uuid.UUID, externalRef string) (Account, error) {
	var clientStatus string
	err := r.db.QueryRowContext(ctx, `
		SELECT status FROM client_businesses WHERE id = $1
	`, clientID).Scan(&clientStatus)
	if errors.Is(err, sql.ErrNoRows) {
		return Account{}, ErrClientNotFound
	}
	if err != nil {
		return Account{}, fmt.Errorf("lookup client: %w", err)
	}
	if !CanCreateAccount(ClientStatus(clientStatus)) {
		return Account{}, ErrClientSuspended
	}

	acc := NewAccount(clientID, externalRef)
	err = r.db.QueryRowContext(ctx, `
		INSERT INTO accounts (id, client_id, external_ref, status)
		VALUES ($1, $2, $3, $4)
		RETURNING balance, created_at, updated_at
	`, acc.ID, acc.ClientID, acc.ExternalRef, string(acc.Status)).Scan(&acc.Balance, &acc.CreatedAt, &acc.UpdatedAt)
	if err != nil {
		return Account{}, fmt.Errorf("insert account: %w", err)
	}

	return acc, nil
}

func (r *AccountRepository) Get(ctx context.Context, id uuid.UUID) (Account, error) {
	var (
		acc    Account
		status string
	)
	err := r.db.QueryRowContext(ctx, `
		SELECT id, client_id, external_ref, status, balance, created_at, updated_at
		FROM accounts WHERE id = $1
	`, id).Scan(&acc.ID, &acc.ClientID, &acc.ExternalRef, &status, &acc.Balance, &acc.CreatedAt, &acc.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Account{}, ErrAccountNotFound
	}
	if err != nil {
		return Account{}, fmt.Errorf("get account: %w", err)
	}
	acc.Status = AccountStatus(status)
	return acc, nil
}

func (r *AccountRepository) ListByClient(ctx context.Context, clientID uuid.UUID) ([]Account, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, client_id, external_ref, status, balance, created_at, updated_at
		FROM accounts WHERE client_id = $1 ORDER BY created_at
	`, clientID)
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}
	defer rows.Close()

	var accounts []Account
	for rows.Next() {
		var (
			acc    Account
			status string
		)
		if err := rows.Scan(&acc.ID, &acc.ClientID, &acc.ExternalRef, &status, &acc.Balance, &acc.CreatedAt, &acc.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan account: %w", err)
		}
		acc.Status = AccountStatus(status)
		accounts = append(accounts, acc)
	}
	return accounts, rows.Err()
}

func (r *AccountRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status AccountStatus) (Account, error) {
	if !status.Valid() {
		return Account{}, ErrInvalidAccountStatus
	}

	var (
		acc          Account
		storedStatus string
	)
	err := r.db.QueryRowContext(ctx, `
		UPDATE accounts SET status = $1, updated_at = now() WHERE id = $2
		RETURNING id, client_id, external_ref, status, balance, created_at, updated_at
	`, string(status), id).Scan(&acc.ID, &acc.ClientID, &acc.ExternalRef, &storedStatus, &acc.Balance, &acc.CreatedAt, &acc.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Account{}, ErrAccountNotFound
	}
	if err != nil {
		return Account{}, fmt.Errorf("update account status: %w", err)
	}
	acc.Status = AccountStatus(storedStatus)
	return acc, nil
}

// ApplyLedgerTransaction idempotently applies the net balance deltas from
// one ledger transaction. If transactionID has already been processed
// (recorded in processed_ledger_transactions), it is a no-op — this is the
// dedup mechanism protecting against redelivery of an at-least-once event.
// A delta for an account_id with no matching local accounts row is
// skipped (logged, not an error): ledger-service and accounts-service are
// separate bounded contexts with no foreign key between them.
func (r *AccountRepository) ApplyLedgerTransaction(ctx context.Context, transactionID uuid.UUID, deltas map[uuid.UUID]int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx, `
		INSERT INTO processed_ledger_transactions (transaction_id) VALUES ($1)
		ON CONFLICT (transaction_id) DO NOTHING
	`, transactionID)
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

	for accountID, delta := range deltas {
		result, err := tx.ExecContext(ctx, `
			UPDATE accounts SET balance = balance + $1, updated_at = now() WHERE id = $2
		`, delta, accountID)
		if err != nil {
			return fmt.Errorf("apply balance delta for account %s: %w", accountID, err)
		}
		if n, _ := result.RowsAffected(); n == 0 {
			log.Printf("account: ledger transaction %s references unknown account %s, skipping", transactionID, accountID)
		}
	}

	return tx.Commit()
}
