# ledger-service MVP Implementation Plan

> **Execution mode:** Mentor mode per `CLAUDE.md` — this overrides the usual subagent-driven-development/executing-plans execution flow. For each step below: explain the concept/pattern first, then the user writes the code. Review what they write and point out mistakes/edge cases rather than silently fixing them. The code blocks in this plan are an answer key for you (the reviewer) to check against — not something to paste into service files. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the first working slice of ledger-service: an HTTP API backed by Postgres that records double-entry ledger entries (rejecting unbalanced transactions) and lets callers query entries by account or transaction. No event publishing in this plan — that is deferred to a later plan once the event broker is chosen.

**Architecture:** Standard Go project layout (`cmd/` + `internal/`) with three internal layers: `ledger` (domain model + balance validation, no I/O), `db` (Postgres connection + embedded golang-migrate migrations), and `api` (chi router + HTTP handlers). `api` depends on `ledger`, `ledger`'s repository depends on `db`'s `*sql.DB`. Tests for anything touching Postgres use testcontainers-go to run against a real, throwaway Postgres instance.

**Tech Stack:** Go 1.23, chi v5 (routing), pgx v5 (Postgres driver via `database/sql`), golang-migrate v4 (schema migrations), testify (assertions), testcontainers-go (integration test DB), Postgres 16.

---

## Prerequisites

- Go 1.23+ installed (verified: `go version` reports `go1.26.2` on this machine, which is forward-compatible)
- Docker running locally (required by testcontainers-go for Tasks 4, 6, 7) — **not currently running** on this machine (`docker version` failed to reach the daemon at session start). Start Docker Desktop before Task 4.
- Module path: `github.com/phuoctmse/settleguard/ledger-service`

---

### Task 1: Initialize Go module and project skeleton

**Files:**
- Create: `services/ledger-service/go.mod`
- Create: `services/ledger-service/.gitignore`
- Create directories: `services/ledger-service/cmd/server/`, `services/ledger-service/internal/api/`, `services/ledger-service/internal/db/migrations/`, `services/ledger-service/internal/ledger/`, `services/ledger-service/internal/testutil/`

- [X] **Step 1: Create the directory skeleton**

Run:
```bash
mkdir -p services/ledger-service/cmd/server
mkdir -p services/ledger-service/internal/api
mkdir -p services/ledger-service/internal/db/migrations
mkdir -p services/ledger-service/internal/ledger
mkdir -p services/ledger-service/internal/testutil
```

- [X] **Step 2: Initialize the Go module**

Run:
```bash
cd services/ledger-service && go mod init github.com/phuoctmse/settleguard/ledger-service
```
Expected: creates `services/ledger-service/go.mod` containing `module github.com/phuoctmse/settleguard/ledger-service` and a `go 1.23` (or later) directive.

- [X] **Step 3: Add dependencies**

Run (from `services/ledger-service/`):
```bash
go get github.com/go-chi/chi/v5
go get github.com/google/uuid
go get github.com/jackc/pgx/v5
go get github.com/golang-migrate/migrate/v4
go get github.com/stretchr/testify
go get github.com/testcontainers/testcontainers-go
```
Expected: `go.mod` gains `require` entries for each module; `go.sum` is created.

- [X] **Step 4: Add `.gitignore`**

`services/ledger-service/.gitignore`:
```
/bin/
*.test
```

- [X] **Step 5: Commit**

```bash
git add services/ledger-service/go.mod services/ledger-service/go.sum services/ledger-service/.gitignore
git commit -m "chore(ledger-service): initialize Go module and dependencies"
```

---

### Task 2: Ledger domain model and balance validation (TDD)

**Files:**
- Create: `services/ledger-service/internal/ledger/entry.go`
- Test: `services/ledger-service/internal/ledger/entry_test.go`

- [X] **Step 1: Write the failing test**

`services/ledger-service/internal/ledger/entry_test.go`:
```go
package ledger_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/phuoctmse/settleguard/ledger-service/internal/ledger"
)

func TestValidateBalanced(t *testing.T) {
	accountA := uuid.New()
	accountB := uuid.New()

	tests := []struct {
		name    string
		entries []ledger.Entry
		wantErr error
	}{
		{
			name: "balanced debit and credit",
			entries: []ledger.Entry{
				{AccountID: accountA, Direction: ledger.Debit, Amount: 500, Reason: "invoice"},
				{AccountID: accountB, Direction: ledger.Credit, Amount: 500, Reason: "invoice"},
			},
			wantErr: nil,
		},
		{
			name: "unbalanced amounts",
			entries: []ledger.Entry{
				{AccountID: accountA, Direction: ledger.Debit, Amount: 500, Reason: "invoice"},
				{AccountID: accountB, Direction: ledger.Credit, Amount: 400, Reason: "invoice"},
			},
			wantErr: ledger.ErrUnbalancedTransaction,
		},
		{
			name:    "no entries",
			entries: []ledger.Entry{},
			wantErr: ledger.ErrNoEntries,
		},
		{
			name: "zero amount",
			entries: []ledger.Entry{
				{AccountID: accountA, Direction: ledger.Debit, Amount: 0, Reason: "invoice"},
			},
			wantErr: ledger.ErrInvalidAmount,
		},
		{
			name: "invalid direction",
			entries: []ledger.Entry{
				{AccountID: accountA, Direction: "sideways", Amount: 100, Reason: "invoice"},
			},
			wantErr: ledger.ErrInvalidDirection,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ledger.ValidateBalanced(tt.entries)
			if tt.wantErr == nil {
				assert.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, tt.wantErr)
			}
		})
	}
}
```

- [X] **Step 2: Run test to verify it fails**

Run: `cd services/ledger-service && go test ./internal/ledger/... -v`
Expected: FAIL — compile error, `ledger.Entry`, `ledger.Direction`, `ledger.ValidateBalanced` etc. undefined.

- [X] **Step 3: Write the implementation**

`services/ledger-service/internal/ledger/entry.go`:
```go
package ledger

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type Direction string

const (
	Debit  Direction = "debit"
	Credit Direction = "credit"
)

type Entry struct {
	ID            uuid.UUID
	TransactionID uuid.UUID
	AccountID     uuid.UUID
	Direction     Direction
	Amount        int64
	Reason        string
	CreatedAt     time.Time
}

var (
	ErrUnbalancedTransaction = errors.New("ledger: transaction entries do not balance")
	ErrInvalidDirection      = errors.New("ledger: entry direction must be debit or credit")
	ErrInvalidAmount         = errors.New("ledger: entry amount must be positive")
	ErrNoEntries             = errors.New("ledger: transaction must have at least one entry")
)

// ValidateBalanced checks that a set of entries for a single transaction is
// well-formed and that total debits equal total credits.
func ValidateBalanced(entries []Entry) error {
	if len(entries) == 0 {
		return ErrNoEntries
	}

	var debitTotal, creditTotal int64
	for _, e := range entries {
		if e.Amount <= 0 {
			return ErrInvalidAmount
		}
		switch e.Direction {
		case Debit:
			debitTotal += e.Amount
		case Credit:
			creditTotal += e.Amount
		default:
			return ErrInvalidDirection
		}
	}

	if debitTotal != creditTotal {
		return ErrUnbalancedTransaction
	}

	return nil
}
```

- [X] **Step 4: Run test to verify it passes**

Run: `cd services/ledger-service && go test ./internal/ledger/... -v`
Expected: PASS — all 5 subtests pass.

- [X] **Step 5: Commit**

```bash
git add services/ledger-service/internal/ledger/entry.go services/ledger-service/internal/ledger/entry_test.go
git commit -m "feat(ledger-service): add ledger entry domain model and balance validation"
```

---

### Task 3: Database migrations

**Files:**
- Create: `services/ledger-service/internal/db/migrations/000001_create_ledger_entries.up.sql`
- Create: `services/ledger-service/internal/db/migrations/000001_create_ledger_entries.down.sql`

- [X] **Step 1: Write the up migration**

`services/ledger-service/internal/db/migrations/000001_create_ledger_entries.up.sql`:
```sql
CREATE TABLE ledger_entries (
    id UUID PRIMARY KEY,
    transaction_id UUID NOT NULL,
    account_id UUID NOT NULL,
    direction TEXT NOT NULL CHECK (direction IN ('debit', 'credit')),
    amount BIGINT NOT NULL CHECK (amount > 0),
    reason TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_ledger_entries_transaction_id ON ledger_entries (transaction_id);
CREATE INDEX idx_ledger_entries_account_id ON ledger_entries (account_id);
```

Postgres 18 has `gen_random_uuid()` built into core (no `pgcrypto` extension needed — that was only required pre-13). It's dropped here entirely rather than just de-extensioned: the repository (Task 5) always sets `e.ID = uuid.New()` in Go before inserting, so the DB-side default would never actually run — keeping it would be dead code with no caller.

- [X] **Step 2: Write the down migration**

`services/ledger-service/internal/db/migrations/000001_create_ledger_entries.down.sql`:
```sql
DROP TABLE IF EXISTS ledger_entries;
```

- [X] **Step 3: Commit**

```bash
git add services/ledger-service/internal/db/migrations
git commit -m "feat(ledger-service): add ledger_entries table migration"
```

(No test in this task — migrations are exercised by Task 4's connection/migration runner test.)

---

### Task 4: Database connection and migration runner (TDD)

**Files:**
- Create: `services/ledger-service/internal/db/db.go`
- Create: `services/ledger-service/internal/testutil/postgres.go`
- Test: `services/ledger-service/internal/db/db_test.go`

- [ ] **Step 1: Write the test DB helper (used by this and later tasks)**

`services/ledger-service/internal/testutil/postgres.go`:
```go
package testutil

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/phuoctmse/settleguard/ledger-service/internal/db"
)

// NewTestDB starts a throwaway Postgres container, runs all migrations
// against it, and returns a connected *sql.DB. The container and connection
// are torn down automatically when the test completes.
func NewTestDB(t *testing.T) *sql.DB {
	t.Helper()
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "postgres:16-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "ledger",
			"POSTGRES_PASSWORD": "ledger",
			"POSTGRES_DB":       "ledger",
		},
		WaitingFor: wait.ForListeningPort("5432/tcp"),
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

	dsn := fmt.Sprintf("postgres://ledger:ledger@%s:%s/ledger?sslmode=disable", host, port.Port())

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
```

- [ ] **Step 2: Write the failing test**

`services/ledger-service/internal/db/db_test.go`:
```go
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
```

- [ ] **Step 3: Run test to verify it fails**

Run: `cd services/ledger-service && go test ./internal/db/... -v`
Expected: FAIL — compile error, `db.Connect` and `db.Migrate` undefined.

- [ ] **Step 4: Write the implementation**

`services/ledger-service/internal/db/db.go`:
```go
package db

import (
	"database/sql"
	"embed"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/jackc/pgx/v5/stdlib"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Connect opens and pings a Postgres connection pool using the pgx driver.
func Connect(dsn string) (*sql.DB, error) {
	conn, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}
	return conn, nil
}

// Migrate runs all embedded migrations against conn, up to the latest version.
func Migrate(conn *sql.DB) error {
	sourceDriver, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("load migrations: %w", err)
	}

	dbDriver, err := postgres.WithInstance(conn, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("create migrate driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", sourceDriver, "postgres", dbDriver)
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("run migrations: %w", err)
	}
	return nil
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `cd services/ledger-service && go test ./internal/db/... -v`
Expected: PASS — `TestMigrate_CreatesLedgerEntriesTable` passes (requires Docker running).

- [ ] **Step 6: Commit**

```bash
git add services/ledger-service/internal/db/db.go services/ledger-service/internal/db/db_test.go services/ledger-service/internal/testutil/postgres.go
git commit -m "feat(ledger-service): add Postgres connection and migration runner"
```

---

### Task 5: Ledger repository (TDD)

**Files:**
- Create: `services/ledger-service/internal/ledger/repository.go`
- Test: `services/ledger-service/internal/ledger/repository_test.go`

- [ ] **Step 1: Write the failing test**

`services/ledger-service/internal/ledger/repository_test.go`:
```go
package ledger_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/phuoctmse/settleguard/ledger-service/internal/ledger"
	"github.com/phuoctmse/settleguard/ledger-service/internal/testutil"
)

func TestRepository_InsertAndListTransaction(t *testing.T) {
	conn := testutil.NewTestDB(t)
	repo := ledger.NewRepository(conn)

	accountA := uuid.New()
	accountB := uuid.New()
	txID := uuid.New()

	entries := []ledger.Entry{
		{AccountID: accountA, Direction: ledger.Debit, Amount: 1000, Reason: "payout"},
		{AccountID: accountB, Direction: ledger.Credit, Amount: 1000, Reason: "payout"},
	}

	inserted, err := repo.InsertTransaction(context.Background(), txID, entries)
	require.NoError(t, err)
	require.Len(t, inserted, 2)
	for _, e := range inserted {
		assert.Equal(t, txID, e.TransactionID)
		assert.NotEqual(t, uuid.Nil, e.ID)
		assert.False(t, e.CreatedAt.IsZero())
	}

	byAccount, err := repo.ListByAccount(context.Background(), accountA)
	require.NoError(t, err)
	require.Len(t, byAccount, 1)
	assert.Equal(t, accountA, byAccount[0].AccountID)

	byTransaction, err := repo.ListByTransaction(context.Background(), txID)
	require.NoError(t, err)
	assert.Len(t, byTransaction, 2)
}

func TestRepository_InsertTransaction_RejectsUnbalanced(t *testing.T) {
	conn := testutil.NewTestDB(t)
	repo := ledger.NewRepository(conn)

	entries := []ledger.Entry{
		{AccountID: uuid.New(), Direction: ledger.Debit, Amount: 1000, Reason: "payout"},
		{AccountID: uuid.New(), Direction: ledger.Credit, Amount: 500, Reason: "payout"},
	}

	_, err := repo.InsertTransaction(context.Background(), uuid.New(), entries)
	assert.ErrorIs(t, err, ledger.ErrUnbalancedTransaction)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd services/ledger-service && go test ./internal/ledger/... -run TestRepository -v`
Expected: FAIL — compile error, `ledger.NewRepository` undefined.

- [ ] **Step 3: Write the implementation**

`services/ledger-service/internal/ledger/repository.go`:
```go
package ledger

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// InsertTransaction validates that entries balance, then inserts all of them
// under the given transactionID within a single DB transaction.
func (r *Repository) InsertTransaction(ctx context.Context, transactionID uuid.UUID, entries []Entry) ([]Entry, error) {
	if err := ValidateBalanced(entries); err != nil {
		return nil, err
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO ledger_entries (id, transaction_id, account_id, direction, amount, reason)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING created_at
	`)
	if err != nil {
		return nil, fmt.Errorf("prepare insert: %w", err)
	}
	defer stmt.Close()

	inserted := make([]Entry, len(entries))
	for i, e := range entries {
		e.ID = uuid.New()
		e.TransactionID = transactionID
		if err := stmt.QueryRowContext(ctx, e.ID, e.TransactionID, e.AccountID, string(e.Direction), e.Amount, e.Reason).Scan(&e.CreatedAt); err != nil {
			return nil, fmt.Errorf("insert entry: %w", err)
		}
		inserted[i] = e
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	return inserted, nil
}

func (r *Repository) ListByAccount(ctx context.Context, accountID uuid.UUID) ([]Entry, error) {
	return r.query(ctx, `
		SELECT id, transaction_id, account_id, direction, amount, reason, created_at
		FROM ledger_entries WHERE account_id = $1 ORDER BY created_at
	`, accountID)
}

func (r *Repository) ListByTransaction(ctx context.Context, transactionID uuid.UUID) ([]Entry, error) {
	return r.query(ctx, `
		SELECT id, transaction_id, account_id, direction, amount, reason, created_at
		FROM ledger_entries WHERE transaction_id = $1 ORDER BY created_at
	`, transactionID)
}

func (r *Repository) query(ctx context.Context, q string, arg uuid.UUID) ([]Entry, error) {
	rows, err := r.db.QueryContext(ctx, q, arg)
	if err != nil {
		return nil, fmt.Errorf("query entries: %w", err)
	}
	defer rows.Close()

	var entries []Entry
	for rows.Next() {
		var e Entry
		var direction string
		if err := rows.Scan(&e.ID, &e.TransactionID, &e.AccountID, &direction, &e.Amount, &e.Reason, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan entry: %w", err)
		}
		e.Direction = Direction(direction)
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd services/ledger-service && go test ./internal/ledger/... -v`
Expected: PASS — all tests in the `ledger` package pass (requires Docker running).

- [ ] **Step 5: Commit**

```bash
git add services/ledger-service/internal/ledger/repository.go services/ledger-service/internal/ledger/repository_test.go
git commit -m "feat(ledger-service): add Postgres-backed ledger repository"
```

---

### Task 6: HTTP API — handlers and router (TDD)

**Files:**
- Create: `services/ledger-service/internal/api/handlers.go`
- Create: `services/ledger-service/internal/api/router.go`
- Test: `services/ledger-service/internal/api/handlers_test.go`

- [ ] **Step 1: Write the failing test**

`services/ledger-service/internal/api/handlers_test.go`:
```go
package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/phuoctmse/settleguard/ledger-service/internal/api"
	"github.com/phuoctmse/settleguard/ledger-service/internal/ledger"
	"github.com/phuoctmse/settleguard/ledger-service/internal/testutil"
)

func TestCreateTransactionAndListEntries(t *testing.T) {
	conn := testutil.NewTestDB(t)
	repo := ledger.NewRepository(conn)
	handlers := api.NewHandlers(repo)
	server := httptest.NewServer(api.NewRouter(handlers))
	defer server.Close()

	accountA := uuid.New().String()
	accountB := uuid.New().String()

	body := map[string]any{
		"entries": []map[string]any{
			{"account_id": accountA, "direction": "debit", "amount": 1500, "reason": "invoice #1"},
			{"account_id": accountB, "direction": "credit", "amount": 1500, "reason": "invoice #1"},
		},
	}
	payload, err := json.Marshal(body)
	require.NoError(t, err)

	resp, err := http.Post(server.URL+"/transactions", "application/json", bytes.NewReader(payload))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var created []map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&created))
	require.Len(t, created, 2)

	listResp, err := http.Get(server.URL + "/entries?account_id=" + accountA)
	require.NoError(t, err)
	defer listResp.Body.Close()
	assert.Equal(t, http.StatusOK, listResp.StatusCode)

	var listed []map[string]any
	require.NoError(t, json.NewDecoder(listResp.Body).Decode(&listed))
	assert.Len(t, listed, 1)
}

func TestCreateTransaction_RejectsUnbalanced(t *testing.T) {
	conn := testutil.NewTestDB(t)
	repo := ledger.NewRepository(conn)
	handlers := api.NewHandlers(repo)
	server := httptest.NewServer(api.NewRouter(handlers))
	defer server.Close()

	body := map[string]any{
		"entries": []map[string]any{
			{"account_id": uuid.New().String(), "direction": "debit", "amount": 1000, "reason": "bad"},
			{"account_id": uuid.New().String(), "direction": "credit", "amount": 900, "reason": "bad"},
		},
	}
	payload, err := json.Marshal(body)
	require.NoError(t, err)

	resp, err := http.Post(server.URL+"/transactions", "application/json", bytes.NewReader(payload))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
}

func TestListEntries_RequiresQueryParam(t *testing.T) {
	conn := testutil.NewTestDB(t)
	repo := ledger.NewRepository(conn)
	handlers := api.NewHandlers(repo)
	server := httptest.NewServer(api.NewRouter(handlers))
	defer server.Close()

	resp, err := http.Get(server.URL + "/entries")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd services/ledger-service && go test ./internal/api/... -v`
Expected: FAIL — compile error, `api.NewHandlers`, `api.NewRouter` undefined.

- [ ] **Step 3: Write the handlers implementation**

`services/ledger-service/internal/api/handlers.go`:
```go
package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/phuoctmse/settleguard/ledger-service/internal/ledger"
)

const timeFormat = "2006-01-02T15:04:05.000Z07:00"

type Handlers struct {
	repo *ledger.Repository
}

func NewHandlers(repo *ledger.Repository) *Handlers {
	return &Handlers{repo: repo}
}

func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

type entryRequest struct {
	AccountID string `json:"account_id"`
	Direction string `json:"direction"`
	Amount    int64  `json:"amount"`
	Reason    string `json:"reason"`
}

type createTransactionRequest struct {
	Entries []entryRequest `json:"entries"`
}

type entryResponse struct {
	ID            string `json:"id"`
	TransactionID string `json:"transaction_id"`
	AccountID     string `json:"account_id"`
	Direction     string `json:"direction"`
	Amount        int64  `json:"amount"`
	Reason        string `json:"reason"`
	CreatedAt     string `json:"created_at"`
}

func (h *Handlers) CreateTransaction(w http.ResponseWriter, r *http.Request) {
	var req createTransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	entries := make([]ledger.Entry, 0, len(req.Entries))
	for _, e := range req.Entries {
		accountID, err := uuid.Parse(e.AccountID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid account_id")
			return
		}
		entries = append(entries, ledger.Entry{
			AccountID: accountID,
			Direction: ledger.Direction(e.Direction),
			Amount:    e.Amount,
			Reason:    e.Reason,
		})
	}

	inserted, err := h.repo.InsertTransaction(r.Context(), uuid.New(), entries)
	if err != nil {
		switch {
		case errors.Is(err, ledger.ErrUnbalancedTransaction),
			errors.Is(err, ledger.ErrInvalidAmount),
			errors.Is(err, ledger.ErrInvalidDirection),
			errors.Is(err, ledger.ErrNoEntries):
			writeError(w, http.StatusUnprocessableEntity, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	writeEntries(w, http.StatusCreated, inserted)
}

func (h *Handlers) ListEntries(w http.ResponseWriter, r *http.Request) {
	accountParam := r.URL.Query().Get("account_id")
	transactionParam := r.URL.Query().Get("transaction_id")

	var (
		entries []ledger.Entry
		err     error
	)

	switch {
	case accountParam != "":
		id, parseErr := uuid.Parse(accountParam)
		if parseErr != nil {
			writeError(w, http.StatusBadRequest, "invalid account_id")
			return
		}
		entries, err = h.repo.ListByAccount(r.Context(), id)
	case transactionParam != "":
		id, parseErr := uuid.Parse(transactionParam)
		if parseErr != nil {
			writeError(w, http.StatusBadRequest, "invalid transaction_id")
			return
		}
		entries, err = h.repo.ListByTransaction(r.Context(), id)
	default:
		writeError(w, http.StatusBadRequest, "account_id or transaction_id query param is required")
		return
	}

	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeEntries(w, http.StatusOK, entries)
}

func writeEntries(w http.ResponseWriter, status int, entries []ledger.Entry) {
	resp := make([]entryResponse, len(entries))
	for i, e := range entries {
		resp[i] = entryResponse{
			ID:            e.ID.String(),
			TransactionID: e.TransactionID.String(),
			AccountID:     e.AccountID.String(),
			Direction:     string(e.Direction),
			Amount:        e.Amount,
			Reason:        e.Reason,
			CreatedAt:     e.CreatedAt.Format(timeFormat),
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
```

- [ ] **Step 4: Write the router implementation**

`services/ledger-service/internal/api/router.go`:
```go
package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func NewRouter(h *Handlers) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/health", h.Health)
	r.Post("/transactions", h.CreateTransaction)
	r.Get("/entries", h.ListEntries)

	return r
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `cd services/ledger-service && go test ./internal/api/... -v`
Expected: PASS — all 3 tests pass (requires Docker running).

- [ ] **Step 6: Commit**

```bash
git add services/ledger-service/internal/api/handlers.go services/ledger-service/internal/api/router.go services/ledger-service/internal/api/handlers_test.go
git commit -m "feat(ledger-service): add HTTP API for creating and listing ledger entries"
```

---

### Task 7: Wire up main.go

**Files:**
- Create: `services/ledger-service/cmd/server/main.go`

- [ ] **Step 1: Write main.go**

`services/ledger-service/cmd/server/main.go`:
```go
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/phuoctmse/settleguard/ledger-service/internal/api"
	"github.com/phuoctmse/settleguard/ledger-service/internal/db"
	"github.com/phuoctmse/settleguard/ledger-service/internal/ledger"
)

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}

	conn, err := db.Connect(dsn)
	if err != nil {
		log.Fatalf("connect to db: %v", err)
	}
	defer conn.Close()

	if err := db.Migrate(conn); err != nil {
		log.Fatalf("run migrations: %v", err)
	}

	repo := ledger.NewRepository(conn)
	handlers := api.NewHandlers(repo)
	router := api.NewRouter(handlers)

	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	log.Printf("ledger-service listening on %s", addr)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatal(err)
	}
}
```

- [ ] **Step 2: Verify the whole module builds**

Run: `cd services/ledger-service && go build ./...`
Expected: exits with no output and status 0.

- [ ] **Step 3: Verify all tests still pass**

Run: `cd services/ledger-service && go vet ./... && go test ./... -v`
Expected: `go vet` produces no output; all tests across `internal/ledger`, `internal/db`, and `internal/api` PASS (requires Docker running).

- [ ] **Step 4: Commit**

```bash
git add services/ledger-service/cmd/server/main.go
git commit -m "feat(ledger-service): wire up main entrypoint"
```

---

### Task 8: Service README and CLAUDE.md update

**Files:**
- Create: `services/ledger-service/README.md`
- Modify: `CLAUDE.md` (root)

- [ ] **Step 1: Write the service README**

`services/ledger-service/README.md`:
```markdown
# ledger-service

Append-only source of truth for double-entry ledger obligations. See
`docs/PROJECT_CHARTER.md` and
`docs/superpowers/specs/2026-07-29-project-charter-design.md` for the
system-wide context.

## Run locally

Requires a reachable Postgres instance.

```bash
export DATABASE_URL="postgres://ledger:ledger@localhost:5432/ledger?sslmode=disable"
go run ./cmd/server
```

Migrations run automatically on startup.

## Build

```bash
go build ./...
```

## Lint

```bash
go vet ./...
```

## Test

Requires Docker (tests use testcontainers-go to run against a real Postgres
instance).

```bash
go test ./...
```

Run a single test:

```bash
go test ./internal/ledger/... -run TestValidateBalanced -v
```

## API

- `GET /health` — health check
- `POST /transactions` — body: `{"entries": [{"account_id": "<uuid>", "direction": "debit"|"credit", "amount": <int64 minor units>, "reason": "<string>"}, ...]}`. Rejects with `422` if debits and credits don't balance.
- `GET /entries?account_id=<uuid>` or `GET /entries?transaction_id=<uuid>` — list entries.
```

- [ ] **Step 2: Update root CLAUDE.md**

In `CLAUDE.md`, replace the "Project Status" section (which currently says no service has been implemented) with:

```markdown
## Project Status

`services/ledger-service` has a working MVP (record + query ledger entries;
see `services/ledger-service/README.md` for build/lint/test/run commands).
The other three services (`accounts-service`, `notification-service`,
`settlement-engine`) and `mobile-app` are still scaffolds with no code. As
each is implemented, add its build/lint/test commands to this file rather
than assuming conventions from the stack.
```

- [ ] **Step 3: Commit**

```bash
git add services/ledger-service/README.md CLAUDE.md
git commit -m "docs(ledger-service): add service README and update root CLAUDE.md"
```

---

## Explicitly Deferred (not in this plan)

- Publishing `ledger.entry-recorded` events (needs the event broker decision from the charter's deferred list)
- Dockerfile / k8s manifests for ledger-service
- Authentication/authorization on the HTTP API
- accounts-service, settlement-engine, notification-service, mobile-app (separate plans)
