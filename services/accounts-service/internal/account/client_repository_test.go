package account_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/phuoctmse/settleguard/accounts-service/internal/account"
	"github.com/phuoctmse/settleguard/accounts-service/internal/testutil"
)

func TestClientRepository_CreateAndGet(t *testing.T) {
	conn := testutil.NewTestDB(t)
	repo := account.NewClientRepository(conn)

	created, err := repo.Create(context.Background(), "Acme Corp")
	require.NoError(t, err)
	assert.False(t, created.CreatedAt.IsZero())
	assert.Equal(t, account.ClientStatusActive, created.Status)

	fetched, err := repo.Get(context.Background(), created.ID)
	require.NoError(t, err)
	assert.Equal(t, created, fetched)
}

func TestClientRepository_Get_NotFound(t *testing.T) {
	conn := testutil.NewTestDB(t)
	repo := account.NewClientRepository(conn)

	_, err := repo.Get(context.Background(), uuid.New())
	assert.ErrorIs(t, err, account.ErrClientNotFound)
}

func TestClientRepository_UpdateStatus(t *testing.T) {
	conn := testutil.NewTestDB(t)
	repo := account.NewClientRepository(conn)

	created, err := repo.Create(context.Background(), "Acme Corp")
	require.NoError(t, err)

	updated, err := repo.UpdateStatus(context.Background(), created.ID, account.ClientStatusSuspended)
	require.NoError(t, err)
	assert.Equal(t, account.ClientStatusSuspended, updated.Status)
}

func TestClientRepository_UpdateStatus_RejectsInvalid(t *testing.T) {
	conn := testutil.NewTestDB(t)
	repo := account.NewClientRepository(conn)

	created, err := repo.Create(context.Background(), "Acme Corp")
	require.NoError(t, err)

	_, err = repo.UpdateStatus(context.Background(), created.ID, account.ClientStatus("bogus"))
	assert.ErrorIs(t, err, account.ErrInvalidClientStatus)
}

func TestClientRepository_UpdateStatus_NotFound(t *testing.T) {
	conn := testutil.NewTestDB(t)
	repo := account.NewClientRepository(conn)

	_, err := repo.UpdateStatus(context.Background(), uuid.New(), account.ClientStatusSuspended)
	assert.ErrorIs(t, err, account.ErrClientNotFound)
}
