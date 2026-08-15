package db_test

import (
	"testing"

	"github.com/phuoctmse/settleguard/settlement-engine/internal/testutil"
)

// TestMigrate_AppliesCleanly exercises the full migration set against a
// real Postgres container via testutil.NewTestDB (which itself calls
// db.Migrate). A failure here means the SQL is broken, not that
// something downstream is missing.
func TestMigrate_AppliesCleanly(t *testing.T) {
	conn := testutil.NewTestDB(t)
	if err := conn.Ping(); err != nil {
		t.Fatalf("ping after migrate: %v", err)
	}
}
