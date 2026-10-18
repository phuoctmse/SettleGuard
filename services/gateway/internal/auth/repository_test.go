package auth_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/phuoctmse/settleguard/gateway/internal/auth"
	"github.com/phuoctmse/settleguard/gateway/internal/testutil"
)

func TestRepository_CreateThenLookupReturnsClientID(t *testing.T) {
	repo := auth.NewRepository(testutil.NewTestDB(t))
	ctx := context.Background()
	clientID := uuid.New()

	key, raw, err := repo.Create(ctx, clientID, "mobile-app prod")
	require.NoError(t, err)
	assert.Equal(t, clientID, key.ClientID)
	assert.Equal(t, "mobile-app prod", key.Label)
	assert.Nil(t, key.RevokedAt)
	assert.Regexp(t, `^sg_live_`, raw)

	got, err := repo.Lookup(ctx, raw)
	require.NoError(t, err)
	assert.Equal(t, clientID, got)
}

func TestRepository_LookupUnknownKeyIsNotFound(t *testing.T) {
	repo := auth.NewRepository(testutil.NewTestDB(t))

	_, err := repo.Lookup(context.Background(), "sg_live_not-a-real-key")
	assert.ErrorIs(t, err, auth.ErrKeyNotFound)
}

func TestRepository_RevokedKeyIsRejected(t *testing.T) {
	repo := auth.NewRepository(testutil.NewTestDB(t))
	ctx := context.Background()

	key, raw, err := repo.Create(ctx, uuid.New(), "to revoke")
	require.NoError(t, err)
	require.NoError(t, repo.Revoke(ctx, key.ID))

	_, err = repo.Lookup(ctx, raw)
	assert.ErrorIs(t, err, auth.ErrKeyRevoked)
}

func TestRepository_RevokeUnknownIDIsNotFound(t *testing.T) {
	repo := auth.NewRepository(testutil.NewTestDB(t))

	err := repo.Revoke(context.Background(), uuid.New())
	assert.ErrorIs(t, err, auth.ErrKeyNotFound)
}

func TestRepository_ListReturnsOnlyThatClientsKeys(t *testing.T) {
	repo := auth.NewRepository(testutil.NewTestDB(t))
	ctx := context.Background()
	mine, theirs := uuid.New(), uuid.New()

	_, _, err := repo.Create(ctx, mine, "a")
	require.NoError(t, err)
	_, _, err = repo.Create(ctx, mine, "b")
	require.NoError(t, err)
	_, _, err = repo.Create(ctx, theirs, "c")
	require.NoError(t, err)

	keys, err := repo.List(ctx, mine)
	require.NoError(t, err)
	assert.Len(t, keys, 2)
	for _, k := range keys {
		assert.Equal(t, mine, k.ClientID)
	}
}

// The raw key must never be reconstructible from what is stored: only the
// hash lives in the table.
func TestRepository_StoresOnlyTheHash(t *testing.T) {
	conn := testutil.NewTestDB(t)
	repo := auth.NewRepository(conn)

	_, raw, err := repo.Create(context.Background(), uuid.New(), "x")
	require.NoError(t, err)

	var stored string
	require.NoError(t, conn.QueryRow(`SELECT key_hash FROM api_keys`).Scan(&stored))
	assert.Equal(t, auth.HashKey(raw), stored)
	assert.NotEqual(t, raw, stored)
}
