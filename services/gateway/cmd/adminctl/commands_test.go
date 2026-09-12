package main

import (
	"bytes"
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/phuoctmse/settleguard/gateway/internal/auth"
	"github.com/phuoctmse/settleguard/gateway/internal/testutil"
)

var rawKeyLine = regexp.MustCompile(`(?m)^key:\s+(sg_live_[A-Za-z0-9_-]{43})$`)

func TestRunCreate_PrintsRawKeyOnceAndItWorks(t *testing.T) {
	repo := auth.NewRepository(testutil.NewTestDB(t))
	ctx := context.Background()
	client := uuid.New()

	var out bytes.Buffer
	require.NoError(t, runCreate(ctx, repo, client, "mobile-app prod", &out))

	m := rawKeyLine.FindStringSubmatch(out.String())
	require.Len(t, m, 2, "output must contain exactly one 'key: sg_live_...' line:\n%s", out.String())
	assert.Contains(t, out.String(), client.String())
	assert.Contains(t, out.String(), "mobile-app prod")

	got, err := repo.Lookup(ctx, m[1])
	require.NoError(t, err)
	assert.Equal(t, client, got)
}

func TestRunRevoke_MakesTheKeyUnusable(t *testing.T) {
	repo := auth.NewRepository(testutil.NewTestDB(t))
	ctx := context.Background()
	key, raw, err := repo.Create(ctx, uuid.New(), "x")
	require.NoError(t, err)

	var out bytes.Buffer
	require.NoError(t, runRevoke(ctx, repo, key.ID, &out))
	assert.Contains(t, out.String(), key.ID.String())

	_, err = repo.Lookup(ctx, raw)
	assert.ErrorIs(t, err, auth.ErrKeyRevoked)
}

func TestRunList_ShowsMetadataNeverTheKey(t *testing.T) {
	repo := auth.NewRepository(testutil.NewTestDB(t))
	ctx := context.Background()
	client := uuid.New()
	key, raw, err := repo.Create(ctx, client, "listed")
	require.NoError(t, err)

	var out bytes.Buffer
	require.NoError(t, runList(ctx, repo, client, &out))
	assert.Contains(t, out.String(), key.ID.String())
	assert.Contains(t, out.String(), "listed")
	assert.False(t, strings.Contains(out.String(), raw), "list must never print a raw key")
	assert.False(t, strings.Contains(out.String(), auth.KeyPrefix), "not even the prefix of one")
}
