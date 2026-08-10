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

func TestAccountRepository_CreateAndGet(t *testing.T) {
	conn := testutil.NewTestDB(t)
	clients := account.NewClientRepository(conn)
	accounts := account.NewAccountRepository(conn)

	client, err := clients.Create(context.Background(), "Acme Corp")
	require.NoError(t, err)

	created, err := accounts.Create(context.Background(), client.ID, "ext-123")
	require.NoError(t, err)
	assert.Equal(t, client.ID, created.ClientID)
	assert.Equal(t, "ext-123", created.ExternalRef)
	assert.Equal(t, account.AccountStatusActive, created.Status)
	assert.Equal(t, int64(0), created.Balance, "a freshly created account starts at a zero balance")

	fetched, err := accounts.Get(context.Background(), created.ID)
	require.NoError(t, err)
	assert.Equal(t, created, fetched)
}

func TestAccountRepository_Create_RejectsSuspendedClient(t *testing.T) {
	conn := testutil.NewTestDB(t)
	clients := account.NewClientRepository(conn)
	accounts := account.NewAccountRepository(conn)

	client, err := clients.Create(context.Background(), "Acme Corp")
	require.NoError(t, err)
	_, err = clients.UpdateStatus(context.Background(), client.ID, account.ClientStatusSuspended)
	require.NoError(t, err)

	_, err = accounts.Create(context.Background(), client.ID, "ext-123")
	assert.ErrorIs(t, err, account.ErrClientSuspended)
}

func TestAccountRepository_Create_UnknownClient(t *testing.T) {
	conn := testutil.NewTestDB(t)
	accounts := account.NewAccountRepository(conn)

	_, err := accounts.Create(context.Background(), uuid.New(), "ext-123")
	assert.ErrorIs(t, err, account.ErrClientNotFound)
}

func TestAccountRepository_Get_NotFound(t *testing.T) {
	conn := testutil.NewTestDB(t)
	accounts := account.NewAccountRepository(conn)

	_, err := accounts.Get(context.Background(), uuid.New())
	assert.ErrorIs(t, err, account.ErrAccountNotFound)
}

func TestAccountRepository_ListByClient(t *testing.T) {
	conn := testutil.NewTestDB(t)
	clients := account.NewClientRepository(conn)
	accounts := account.NewAccountRepository(conn)

	clientA, err := clients.Create(context.Background(), "Client A")
	require.NoError(t, err)
	clientB, err := clients.Create(context.Background(), "Client B")
	require.NoError(t, err)

	_, err = accounts.Create(context.Background(), clientA.ID, "a1")
	require.NoError(t, err)
	_, err = accounts.Create(context.Background(), clientA.ID, "a2")
	require.NoError(t, err)
	_, err = accounts.Create(context.Background(), clientB.ID, "b1")
	require.NoError(t, err)

	list, err := accounts.ListByClient(context.Background(), clientA.ID)
	require.NoError(t, err)
	assert.Len(t, list, 2)
}

func TestAccountRepository_UpdateStatus(t *testing.T) {
	conn := testutil.NewTestDB(t)
	clients := account.NewClientRepository(conn)
	accounts := account.NewAccountRepository(conn)

	client, err := clients.Create(context.Background(), "Acme Corp")
	require.NoError(t, err)
	created, err := accounts.Create(context.Background(), client.ID, "ext-123")
	require.NoError(t, err)

	updated, err := accounts.UpdateStatus(context.Background(), created.ID, account.AccountStatusClosed)
	require.NoError(t, err)
	assert.Equal(t, account.AccountStatusClosed, updated.Status)
	assert.True(t, updated.UpdatedAt.After(created.UpdatedAt) || updated.UpdatedAt.Equal(created.UpdatedAt))
}

func TestAccountRepository_UpdateStatus_RejectsInvalid(t *testing.T) {
	conn := testutil.NewTestDB(t)
	clients := account.NewClientRepository(conn)
	accounts := account.NewAccountRepository(conn)

	client, err := clients.Create(context.Background(), "Acme Corp")
	require.NoError(t, err)
	created, err := accounts.Create(context.Background(), client.ID, "ext-123")
	require.NoError(t, err)

	_, err = accounts.UpdateStatus(context.Background(), created.ID, account.AccountStatus("bogus"))
	assert.ErrorIs(t, err, account.ErrInvalidAccountStatus)
}

func TestAccountRepository_UpdateStatus_NotFound(t *testing.T) {
	conn := testutil.NewTestDB(t)
	accounts := account.NewAccountRepository(conn)

	_, err := accounts.UpdateStatus(context.Background(), uuid.New(), account.AccountStatusClosed)
	assert.ErrorIs(t, err, account.ErrAccountNotFound)
}
