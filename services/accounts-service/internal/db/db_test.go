package db_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/phuoctmse/settleguard/accounts-service/internal/testutil"
)

func TestMigrate_CreatesClientBusinessesTable(t *testing.T) {
	conn := testutil.NewTestDB(t)

	var exists bool
	err := conn.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_name = 'client_businesses'
		)
	`).Scan(&exists)
	require.NoError(t, err)
	assert.True(t, exists, "client_businesses table should exist after migration")
}

func TestMigrate_CreatesAccountsTable(t *testing.T) {
	conn := testutil.NewTestDB(t)

	var exists bool
	err := conn.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_name = 'accounts'
		)
	`).Scan(&exists)
	require.NoError(t, err)
	assert.True(t, exists, "accounts table should exist after migration")
}
