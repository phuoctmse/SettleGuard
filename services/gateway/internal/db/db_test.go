package db_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/phuoctmse/settleguard/gateway/internal/testutil"
)

func TestMigrate_CreatesAPIKeysTable(t *testing.T) {
	conn := testutil.NewTestDB(t)

	var exists bool
	err := conn.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables WHERE table_name = 'api_keys'
		)
	`).Scan(&exists)
	require.NoError(t, err)
	assert.True(t, exists, "api_keys table should exist after migration")
}

func TestMigrate_KeyHashIsUnique(t *testing.T) {
	conn := testutil.NewTestDB(t)

	_, err := conn.Exec(`INSERT INTO api_keys (id, client_id, key_hash, label)
		VALUES (gen_random_uuid(), gen_random_uuid(), 'same', 'a')`)
	require.NoError(t, err)

	_, err = conn.Exec(`INSERT INTO api_keys (id, client_id, key_hash, label)
		VALUES (gen_random_uuid(), gen_random_uuid(), 'same', 'b')`)
	assert.Error(t, err, "a second row with the same key_hash must violate the UNIQUE constraint")
}
