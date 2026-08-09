package db_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/phuoctmse/settleguard/ledger-service/internal/testutil"
)

func TestMigrate_CreatesLedgerEntriesTable(t *testing.T) {
	conn := testutil.NewTestDB(t)

	var exists bool
	err := conn.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_name = 'ledger_entries'
		)
	`).Scan(&exists)
	require.NoError(t, err)
	assert.True(t, exists, "ledger_entries table should exist after migration")
}
