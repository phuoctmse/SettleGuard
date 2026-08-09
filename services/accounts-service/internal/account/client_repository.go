package account

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

type ClientRepository struct {
	db *sql.DB
}

func NewClientRepository(db *sql.DB) *ClientRepository {
	return &ClientRepository{db: db}
}

func (r *ClientRepository) Create(ctx context.Context, name string) (ClientBusiness, error) {
	client, err := NewClientBusiness(name)
	if err != nil {
		return ClientBusiness{}, err
	}

	err = r.db.QueryRowContext(ctx, `
		INSERT INTO client_businesses (id, name, status)
		VALUES ($1, $2, $3)
		RETURNING created_at
	`, client.ID, client.Name, string(client.Status)).Scan(&client.CreatedAt)
	if err != nil {
		return ClientBusiness{}, fmt.Errorf("insert client: %w", err)
	}

	return client, nil
}

func (r *ClientRepository) Get(ctx context.Context, id uuid.UUID) (ClientBusiness, error) {
	var (
		client ClientBusiness
		status string
	)
	err := r.db.QueryRowContext(ctx, `
		SELECT id, name, status, created_at FROM client_businesses WHERE id = $1
	`, id).Scan(&client.ID, &client.Name, &status, &client.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ClientBusiness{}, ErrClientNotFound
	}
	if err != nil {
		return ClientBusiness{}, fmt.Errorf("get client: %w", err)
	}
	client.Status = ClientStatus(status)
	return client, nil
}

func (r *ClientRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status ClientStatus) (ClientBusiness, error) {
	if !status.Valid() {
		return ClientBusiness{}, ErrInvalidClientStatus
	}

	var (
		client       ClientBusiness
		storedStatus string
	)
	err := r.db.QueryRowContext(ctx, `
		UPDATE client_businesses SET status = $1 WHERE id = $2
		RETURNING id, name, status, created_at
	`, string(status), id).Scan(&client.ID, &client.Name, &storedStatus, &client.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ClientBusiness{}, ErrClientNotFound
	}
	if err != nil {
		return ClientBusiness{}, fmt.Errorf("update client status: %w", err)
	}
	client.Status = ClientStatus(storedStatus)
	return client, nil
}
