// Package testutil starts real infrastructure for tests. No mocks: the
// gateway is tested against the same Postgres image it runs on.
package testutil

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/phuoctmse/settleguard/gateway/internal/db"
)

// NewTestDB starts a throwaway Postgres container, runs all migrations
// against it, and returns a connected *sql.DB. The container and connection
// are torn down automatically when the test completes.
func NewTestDB(t *testing.T) *sql.DB {
	t.Helper()
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "postgres:18-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "gateway",
			"POSTGRES_PASSWORD": "gateway",
			"POSTGRES_DB":       "gateway",
		},
		// The official Postgres image restarts its server once internally after
		// initdb; the port can be listening during that restart window before the
		// server actually accepts connections. Waiting for the readiness log line's
		// 2nd occurrence (once for the internal setup pass, once for the real start)
		// avoids the "database system is starting up" flake port-listening alone caused.
		WaitingFor: wait.ForLog("database system is ready to accept connections").WithOccurrence(2),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	t.Cleanup(func() {
		_ = container.Terminate(ctx)
	})

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("get container host: %v", err)
	}
	port, err := container.MappedPort(ctx, "5432")
	if err != nil {
		t.Fatalf("get container port: %v", err)
	}

	dsn := fmt.Sprintf("postgres://gateway:gateway@%s:%s/gateway?sslmode=disable", host, port.Port())

	conn, err := db.Connect(dsn)
	if err != nil {
		t.Fatalf("connect to test db: %v", err)
	}
	t.Cleanup(func() {
		conn.Close()
	})

	if err := db.Migrate(conn); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}

	return conn
}
