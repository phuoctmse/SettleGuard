package auth

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

var (
	// ErrKeyNotFound means no api_keys row matches the presented key or id.
	ErrKeyNotFound = errors.New("auth: api key not found")
	// ErrKeyRevoked means the key exists but revoked_at is set.
	ErrKeyRevoked = errors.New("auth: api key revoked")
)

// Key is one api_keys row. It never carries the raw key; that is returned
// once, by Create, and then exists only in the operator's hands.
type Key struct {
	ID        uuid.UUID
	ClientID  uuid.UUID
	Label     string
	CreatedAt time.Time
	RevokedAt *time.Time
}

// Repository reads and writes api_keys.
type Repository struct {
	db *sql.DB
}

// NewRepository wraps an open connection to the gateway's database.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Create issues a new key for clientID and returns its metadata together
// with the raw key. The raw key is not stored and cannot be recovered.
func (r *Repository) Create(ctx context.Context, clientID uuid.UUID, label string) (Key, string, error) {
	raw, hash, err := GenerateKey()
	if err != nil {
		return Key{}, "", err
	}

	k := Key{ID: uuid.New(), ClientID: clientID, Label: label}
	err = r.db.QueryRowContext(ctx, `
		INSERT INTO api_keys (id, client_id, key_hash, label)
		VALUES ($1, $2, $3, $4)
		RETURNING created_at
	`, k.ID, k.ClientID, hash, k.Label).Scan(&k.CreatedAt)
	if err != nil {
		return Key{}, "", fmt.Errorf("auth: insert api key: %w", err)
	}
	return k, raw, nil
}

// Lookup resolves a presented raw key to its client_id. Not-found and
// revoked are distinct errors here so the caller can log them apart; the
// HTTP layer collapses both into one 401 so a caller cannot tell which.
func (r *Repository) Lookup(ctx context.Context, raw string) (uuid.UUID, error) {
	hash := HashKey(raw)

	var (
		clientID   uuid.UUID
		storedHash string
		revokedAt  sql.NullTime
	)
	err := r.db.QueryRowContext(ctx, `
		SELECT client_id, key_hash, revoked_at FROM api_keys WHERE key_hash = $1
	`, hash).Scan(&clientID, &storedHash, &revokedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return uuid.Nil, ErrKeyNotFound
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("auth: lookup api key: %w", err)
	}

	// The index already matched on equality; re-checking in constant time is
	// belt and braces against a future change that fetches by something else.
	if subtle.ConstantTimeCompare([]byte(storedHash), []byte(hash)) != 1 {
		return uuid.Nil, ErrKeyNotFound
	}
	if revokedAt.Valid {
		return uuid.Nil, ErrKeyRevoked
	}
	return clientID, nil
}

// Revoke marks a key unusable. The row is kept for audit; only revoked_at
// changes. Revoking an already-revoked key is a no-op that succeeds.
func (r *Repository) Revoke(ctx context.Context, id uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE api_keys SET revoked_at = COALESCE(revoked_at, now()) WHERE id = $1
	`, id)
	if err != nil {
		return fmt.Errorf("auth: revoke api key: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("auth: revoke api key rows affected: %w", err)
	}
	if n == 0 {
		return ErrKeyNotFound
	}
	return nil
}

// List returns every key ever issued to clientID, newest first, including
// revoked ones.
func (r *Repository) List(ctx context.Context, clientID uuid.UUID) ([]Key, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, client_id, label, created_at, revoked_at
		FROM api_keys WHERE client_id = $1 ORDER BY created_at DESC
	`, clientID)
	if err != nil {
		return nil, fmt.Errorf("auth: list api keys: %w", err)
	}
	defer rows.Close()

	var keys []Key
	for rows.Next() {
		var (
			k         Key
			revokedAt sql.NullTime
		)
		if err := rows.Scan(&k.ID, &k.ClientID, &k.Label, &k.CreatedAt, &revokedAt); err != nil {
			return nil, fmt.Errorf("auth: scan api key: %w", err)
		}
		if revokedAt.Valid {
			t := revokedAt.Time
			k.RevokedAt = &t
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}
