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

func TestApplyLedgerTransaction_UpdatesBalances(t *testing.T) {
	conn := testutil.NewTestDB(t)
	clients := account.NewClientRepository(conn)
	accounts := account.NewAccountRepository(conn)

	client, err := clients.Create(context.Background(), "Acme Corp")
	require.NoError(t, err)
	accA, err := accounts.Create(context.Background(), client.ID, "a")
	require.NoError(t, err)
	accB, err := accounts.Create(context.Background(), client.ID, "b")
	require.NoError(t, err)

	err = accounts.ApplyLedgerTransaction(context.Background(), uuid.New(), map[uuid.UUID]int64{
		accA.ID: -500,
		accB.ID: 500,
	})
	require.NoError(t, err)

	fetchedA, err := accounts.Get(context.Background(), accA.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(-500), fetchedA.Balance)

	fetchedB, err := accounts.Get(context.Background(), accB.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(500), fetchedB.Balance)
}

func TestApplyLedgerTransaction_IdempotentOnSameTransactionID(t *testing.T) {
	conn := testutil.NewTestDB(t)
	clients := account.NewClientRepository(conn)
	accounts := account.NewAccountRepository(conn)

	client, err := clients.Create(context.Background(), "Acme Corp")
	require.NoError(t, err)
	acc, err := accounts.Create(context.Background(), client.ID, "a")
	require.NoError(t, err)

	txID := uuid.New()
	deltas := map[uuid.UUID]int64{acc.ID: 500}

	require.NoError(t, accounts.ApplyLedgerTransaction(context.Background(), txID, deltas))
	require.NoError(t, accounts.ApplyLedgerTransaction(context.Background(), txID, deltas))

	fetched, err := accounts.Get(context.Background(), acc.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(500), fetched.Balance, "applying the same transaction twice must not double-count")
}

func TestApplyLedgerTransaction_SkipsUnknownAccountGracefully(t *testing.T) {
	conn := testutil.NewTestDB(t)
	clients := account.NewClientRepository(conn)
	accounts := account.NewAccountRepository(conn)

	client, err := clients.Create(context.Background(), "Acme Corp")
	require.NoError(t, err)
	acc, err := accounts.Create(context.Background(), client.ID, "a")
	require.NoError(t, err)

	unknownAccountID := uuid.New()
	txID := uuid.New()

	err = accounts.ApplyLedgerTransaction(context.Background(), txID, map[uuid.UUID]int64{
		acc.ID:           500,
		unknownAccountID: -500,
	})
	require.NoError(t, err, "an unknown account_id in the deltas must not fail the whole apply")

	fetched, err := accounts.Get(context.Background(), acc.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(500), fetched.Balance)

	// A second call with the same transactionID is still a no-op (proves
	// the transaction was correctly marked processed even though it
	// touched an unknown account).
	require.NoError(t, accounts.ApplyLedgerTransaction(context.Background(), txID, map[uuid.UUID]int64{acc.ID: 500}))
	fetched, err = accounts.Get(context.Background(), acc.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(500), fetched.Balance)
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
