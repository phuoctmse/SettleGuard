# accounts-service MVP Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.
>
> **Execution mode override:** per `docs/superpowers/specs/2026-08-08-accounts-service-mvp-design.md` §1, this service is **subagent-driven**, not mentor mode — it repeats the simple CRUD pattern `ledger-service` already established, with no new concept to teach. **Exception:** Task 6 (Account repository) and Task 8 (Account HTTP handlers) implement the one business rule that reaches into balance-of-obligation territory (blocking account creation under a suspended client). For those two tasks specifically, the human must personally read the diff and confirm correctness before merge — do not rely on a subagent's "tests pass" self-report alone.

**Goal:** Build the accounts-service MVP: an HTTP API backed by Postgres that owns `ClientBusiness` (tenant) and `Account` (end-user) identity and status, enforcing that accounts can't be created under a suspended client. No balance-of-obligation, no event publishing, no auth — those are explicitly deferred (see bottom of this plan).

**Architecture:** Same layout as `ledger-service`: `cmd/server/main.go` + `internal/{api,db,account,testutil}/`. `account` holds both domain types (`ClientBusiness`, `Account`) and their Postgres-backed repositories, no I/O in the plain domain code. `db` holds the Postgres connection + embedded golang-migrate migrations. `api` holds chi handlers/router. `api` depends on `account`; `account`'s repositories depend on `db`'s `*sql.DB`. Tests touching Postgres use testcontainers-go against a real, throwaway Postgres instance, exactly like `ledger-service`.

**Tech Stack:** Go 1.23+, chi v5 (routing), pgx v5 (Postgres driver via `database/sql`), golang-migrate v4 (schema migrations, plain `.sql`, no code-gen), testify (assertions), testcontainers-go (integration test DB), Postgres 16.

## Global Constraints

- Module path: `github.com/phuoctmse/settleguard/accounts-service`
- Postgres: own schema/database (`accounts`), no sharing with `ledger-service` (`CLAUDE.md` Stack section)
- Standard layout: `cmd/server/main.go` + `internal/{api,db,account,testutil}/` (spec §2)
- `net/http` + chi v5 only — no heavier framework (`CLAUDE.md` Stack section)
- golang-migrate v4, plain `.sql` files, no code-gen tooling (`CLAUDE.md` Stack section)
- testcontainers-go for DB tests — real Postgres, never `sqlmock` (`CLAUDE.md` Stack section)
- Go package names: short, lowercase, no underscores (`docs/CODING_STANDARDS.md`)
- `gofmt`/`goimports` formatting is mandatory, non-negotiable (`docs/CODING_STANDARDS.md`)
- Imports grouped in three blocks separated by blank lines: stdlib → third-party → internal, with `goimports` local-prefix `github.com/phuoctmse/settleguard` (`docs/CODING_STANDARDS.md`, `.golangci.yml`)
- Acronyms keep consistent casing: `ClientID`, not `ClientId` (`docs/CODING_STANDARDS.md`)
- Sentinel errors always prefixed `Err` (`docs/CODING_STANDARDS.md`)
- Test functions `TestXxx`, test files `_test.go` (`docs/CODING_STANDARDS.md`)
- No `panic` for expected error conditions (bad input, DB errors) — only for unrecoverable programming errors (`docs/CODING_STANDARDS.md`)
- Known domain errors are sentinel errors (`errors.New`) declared next to the domain type they belong to (`docs/CODING_STANDARDS.md`)
- Wrap errors crossing a layer with `fmt.Errorf("...: %w", err)`, never `%v` (`docs/CODING_STANDARDS.md`)
- Never compare errors by string; always `errors.Is` (`docs/CODING_STANDARDS.md`)
- Never silently drop an error (`_ = err`) without an explanatory comment (`docs/CODING_STANDARDS.md`, enforced by `errcheck` in `.golangci.yml`)
- At the HTTP boundary (`api/handlers*.go`), map domain errors to HTTP status explicitly via `errors.Is`; never leak internal error text in the response (`docs/CODING_STANDARDS.md`)
- `.golangci.yml` enables: `errcheck`, `govet`, `staticcheck`, `unused`, `revive` (var-naming, error-naming, exported), `gofmt`, `goimports` — must pass clean

---

## Prerequisites

- Go 1.23+ installed (verified: `go version` reports `go1.26.2`)
- Docker running locally (required by testcontainers-go for Tasks 4, 5, 6, 7, 8)
- `golangci-lint` installed (verified: `v1.64.8` available at `$(go env GOPATH)/bin/golangci-lint`)
- Work happens on the `service/accounts-service` branch (already exists locally, currently contains only the design-spec commit `f66636f`). Check it out before Task 1: `git checkout service/accounts-service`.
- Module path: `github.com/phuoctmse/settleguard/accounts-service`
- **Before every commit step in this plan**, run `git branch --show-current` and confirm it prints `service/accounts-service`. The session may have drifted to another branch (e.g. left over from a prior `/clear`) — never commit accounts-service work onto `step/harness-hardening`, `main`, or any other branch.

---

### Task 1: Initialize Go module and project skeleton

**Files:**
- Create: `services/accounts-service/go.mod`
- Create: `services/accounts-service/.gitignore`
- Create directories: `services/accounts-service/cmd/server/`, `services/accounts-service/internal/api/`, `services/accounts-service/internal/db/migrations/`, `services/accounts-service/internal/account/`, `services/accounts-service/internal/testutil/`

- [ ] **Step 1: Create the directory skeleton**

Run:
```bash
mkdir -p services/accounts-service/cmd/server
mkdir -p services/accounts-service/internal/api
mkdir -p services/accounts-service/internal/db/migrations
mkdir -p services/accounts-service/internal/account
mkdir -p services/accounts-service/internal/testutil
```

- [ ] **Step 2: Initialize the Go module**

Run:
```bash
cd services/accounts-service && go mod init github.com/phuoctmse/settleguard/accounts-service
```
Expected: creates `services/accounts-service/go.mod` containing `module github.com/phuoctmse/settleguard/accounts-service` and a `go 1.23` (or later) directive.

- [ ] **Step 3: Add dependencies**

Run (from `services/accounts-service/`):
```bash
go get github.com/go-chi/chi/v5
go get github.com/google/uuid
go get github.com/jackc/pgx/v5
go get github.com/golang-migrate/migrate/v4
go get github.com/stretchr/testify
go get github.com/testcontainers/testcontainers-go
```
Expected: `go.mod` gains `require` entries for each module; `go.sum` is created.

- [ ] **Step 4: Add `.gitignore`**

`services/accounts-service/.gitignore`:
```
/bin/
*.test
```

- [ ] **Step 5: Commit**

```bash
git add services/accounts-service/go.mod services/accounts-service/go.sum services/accounts-service/.gitignore
git commit -m "chore(accounts-service): initialize Go module and dependencies"
```

---

### Task 2: Domain models — ClientBusiness and Account (TDD)

**Files:**
- Create: `services/accounts-service/internal/account/client.go`
- Create: `services/accounts-service/internal/account/account.go`
- Test: `services/accounts-service/internal/account/client_test.go`
- Test: `services/accounts-service/internal/account/account_test.go`

**Interfaces:**
- Produces: `account.ClientStatus` (`ClientStatusActive`, `ClientStatusSuspended`, `.Valid() bool`), `account.ClientBusiness{ID, Name, Status, CreatedAt}`, `account.NewClientBusiness(name string) (ClientBusiness, error)`, `account.ErrEmptyClientName`, `account.ErrInvalidClientStatus`, `account.ErrClientNotFound`; `account.AccountStatus` (`AccountStatusActive`, `AccountStatusSuspended`, `AccountStatusClosed`, `.Valid() bool`), `account.Account{ID, ClientID, ExternalRef, Status, CreatedAt, UpdatedAt}`, `account.NewAccount(clientID uuid.UUID, externalRef string) Account`, `account.CanCreateAccount(clientStatus ClientStatus) bool`, `account.ErrInvalidAccountStatus`, `account.ErrClientSuspended`, `account.ErrAccountNotFound`. These names are used unchanged by Tasks 5–8.

- [ ] **Step 1: Write the failing tests**

`services/accounts-service/internal/account/client_test.go`:
```go
package account_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/phuoctmse/settleguard/accounts-service/internal/account"
)

func TestNewClientBusiness_Valid(t *testing.T) {
	client, err := account.NewClientBusiness("Acme Corp")

	assert.NoError(t, err)
	assert.Equal(t, "Acme Corp", client.Name)
	assert.Equal(t, account.ClientStatusActive, client.Status)
	assert.NotEqual(t, uuid.Nil, client.ID)
}

func TestNewClientBusiness_RejectsEmptyName(t *testing.T) {
	for _, name := range []string{"", "   "} {
		_, err := account.NewClientBusiness(name)
		assert.ErrorIs(t, err, account.ErrEmptyClientName)
	}
}

func TestClientStatus_Valid(t *testing.T) {
	tests := []struct {
		status account.ClientStatus
		want   bool
	}{
		{account.ClientStatusActive, true},
		{account.ClientStatusSuspended, true},
		{account.ClientStatus("bogus"), false},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.want, tt.status.Valid())
	}
}
```

`services/accounts-service/internal/account/account_test.go`:
```go
package account_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/phuoctmse/settleguard/accounts-service/internal/account"
)

func TestNewAccount_Defaults(t *testing.T) {
	clientID := uuid.New()
	acc := account.NewAccount(clientID, "ext-123")

	assert.Equal(t, clientID, acc.ClientID)
	assert.Equal(t, "ext-123", acc.ExternalRef)
	assert.Equal(t, account.AccountStatusActive, acc.Status)
	assert.NotEqual(t, uuid.Nil, acc.ID)
}

func TestAccountStatus_Valid(t *testing.T) {
	tests := []struct {
		status account.AccountStatus
		want   bool
	}{
		{account.AccountStatusActive, true},
		{account.AccountStatusSuspended, true},
		{account.AccountStatusClosed, true},
		{account.AccountStatus("bogus"), false},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.want, tt.status.Valid())
	}
}

func TestCanCreateAccount(t *testing.T) {
	tests := []struct {
		name         string
		clientStatus account.ClientStatus
		want         bool
	}{
		{"active client allows creation", account.ClientStatusActive, true},
		{"suspended client blocks creation", account.ClientStatusSuspended, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, account.CanCreateAccount(tt.clientStatus))
		})
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd services/accounts-service && go test ./internal/account/... -v`
Expected: FAIL — compile error, `account.ClientBusiness`, `account.NewClientBusiness`, `account.Account`, `account.NewAccount`, `account.CanCreateAccount`, etc. undefined.

- [ ] **Step 3: Write the implementation**

`services/accounts-service/internal/account/client.go`:
```go
package account

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

type ClientStatus string

const (
	ClientStatusActive    ClientStatus = "active"
	ClientStatusSuspended ClientStatus = "suspended"
)

func (s ClientStatus) Valid() bool {
	switch s {
	case ClientStatusActive, ClientStatusSuspended:
		return true
	default:
		return false
	}
}

type ClientBusiness struct {
	ID        uuid.UUID
	Name      string
	Status    ClientStatus
	CreatedAt time.Time
}

var (
	ErrEmptyClientName     = errors.New("account: client name is required")
	ErrInvalidClientStatus = errors.New("account: invalid client status")
	ErrClientNotFound      = errors.New("account: client not found")
)

// NewClientBusiness builds a new ClientBusiness in the active status.
func NewClientBusiness(name string) (ClientBusiness, error) {
	if strings.TrimSpace(name) == "" {
		return ClientBusiness{}, ErrEmptyClientName
	}
	return ClientBusiness{
		ID:     uuid.New(),
		Name:   name,
		Status: ClientStatusActive,
	}, nil
}
```

`services/accounts-service/internal/account/account.go`:
```go
package account

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type AccountStatus string

const (
	AccountStatusActive    AccountStatus = "active"
	AccountStatusSuspended AccountStatus = "suspended"
	AccountStatusClosed    AccountStatus = "closed"
)

func (s AccountStatus) Valid() bool {
	switch s {
	case AccountStatusActive, AccountStatusSuspended, AccountStatusClosed:
		return true
	default:
		return false
	}
}

type Account struct {
	ID          uuid.UUID
	ClientID    uuid.UUID
	ExternalRef string
	Status      AccountStatus
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

var (
	ErrInvalidAccountStatus = errors.New("account: invalid account status")
	ErrClientSuspended      = errors.New("account: cannot create account under a suspended client")
	ErrAccountNotFound      = errors.New("account: account not found")
)

// NewAccount builds a new Account in the active status under clientID.
func NewAccount(clientID uuid.UUID, externalRef string) Account {
	return Account{
		ID:          uuid.New(),
		ClientID:    clientID,
		ExternalRef: externalRef,
		Status:      AccountStatusActive,
	}
}

// CanCreateAccount reports whether a new Account may be created under a
// ClientBusiness currently in clientStatus. This is the one MVP rule that
// reaches into balance-of-obligation territory (spec §1) — both this
// function and its callers (AccountRepository.Create, the CreateAccount
// handler) require manual review before merge, not just passing tests.
func CanCreateAccount(clientStatus ClientStatus) bool {
	return clientStatus == ClientStatusActive
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd services/accounts-service && go test ./internal/account/... -v`
Expected: PASS — all subtests in both files pass.

- [ ] **Step 5: Commit**

```bash
git add services/accounts-service/internal/account/client.go services/accounts-service/internal/account/account.go services/accounts-service/internal/account/client_test.go services/accounts-service/internal/account/account_test.go
git commit -m "feat(accounts-service): add ClientBusiness and Account domain models"
```

---

### Task 3: Database migrations

**Files:**
- Create: `services/accounts-service/internal/db/migrations/000001_create_client_businesses.up.sql`
- Create: `services/accounts-service/internal/db/migrations/000001_create_client_businesses.down.sql`
- Create: `services/accounts-service/internal/db/migrations/000002_create_accounts.up.sql`
- Create: `services/accounts-service/internal/db/migrations/000002_create_accounts.down.sql`

- [ ] **Step 1: Write the client_businesses migration**

`services/accounts-service/internal/db/migrations/000001_create_client_businesses.up.sql`:
```sql
CREATE TABLE client_businesses (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('active', 'suspended')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

`services/accounts-service/internal/db/migrations/000001_create_client_businesses.down.sql`:
```sql
DROP TABLE IF EXISTS client_businesses;
```

- [ ] **Step 2: Write the accounts migration**

`services/accounts-service/internal/db/migrations/000002_create_accounts.up.sql`:
```sql
CREATE TABLE accounts (
    id UUID PRIMARY KEY,
    client_id UUID NOT NULL REFERENCES client_businesses(id),
    external_ref TEXT,
    status TEXT NOT NULL CHECK (status IN ('active', 'suspended', 'closed')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_accounts_client_id ON accounts (client_id);
```

`services/accounts-service/internal/db/migrations/000002_create_accounts.down.sql`:
```sql
DROP TABLE IF EXISTS accounts;
```

IDs are generated in Go (`uuid.New()`) before insert, same as `ledger-service` — no DB-side UUID default needed.

- [ ] **Step 3: Commit**

```bash
git add services/accounts-service/internal/db/migrations
git commit -m "feat(accounts-service): add client_businesses and accounts table migrations"
```

(No test in this task — migrations are exercised by Task 4's connection/migration runner test.)

---

### Task 4: Database connection and migration runner (TDD)

**Files:**
- Create: `services/accounts-service/internal/db/db.go`
- Create: `services/accounts-service/internal/testutil/postgres.go`
- Test: `services/accounts-service/internal/db/db_test.go`

**Interfaces:**
- Produces: `db.Connect(dsn string) (*sql.DB, error)`, `db.Migrate(conn *sql.DB) error`, `testutil.NewTestDB(t *testing.T) *sql.DB`. Used unchanged by Tasks 5–8.

- [ ] **Step 1: Write the test DB helper (used by this and later tasks)**

`services/accounts-service/internal/testutil/postgres.go`:
```go
package testutil

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/phuoctmse/settleguard/accounts-service/internal/db"
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
			"POSTGRES_USER":     "accounts",
			"POSTGRES_PASSWORD": "accounts",
			"POSTGRES_DB":       "accounts",
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

	dsn := fmt.Sprintf("postgres://accounts:accounts@%s:%s/accounts?sslmode=disable", host, port.Port())

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

`services/accounts-service/internal/db/db_test.go`:
```go
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
```

- [ ] **Step 3: Run test to verify it fails**

Run: `cd services/accounts-service && go test ./internal/db/... -v`
Expected: FAIL — compile error, `db.Connect` and `db.Migrate` undefined.

- [ ] **Step 4: Write the implementation**

`services/accounts-service/internal/db/db.go`:
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

Run: `cd services/accounts-service && go test ./internal/db/... -v`
Expected: PASS — both tests pass (requires Docker running).

- [ ] **Step 6: Commit**

```bash
git add services/accounts-service/internal/db/db.go services/accounts-service/internal/db/db_test.go services/accounts-service/internal/testutil/postgres.go
git commit -m "feat(accounts-service): add Postgres connection and migration runner"
```

---

### Task 5: ClientBusiness repository (TDD)

**Files:**
- Create: `services/accounts-service/internal/account/client_repository.go`
- Test: `services/accounts-service/internal/account/client_repository_test.go`

**Interfaces:**
- Consumes: `account.NewClientBusiness`, `account.ClientBusiness`, `account.ClientStatus`, `account.ErrInvalidClientStatus`, `account.ErrClientNotFound` (Task 2); `testutil.NewTestDB` (Task 4).
- Produces: `account.NewClientRepository(db *sql.DB) *ClientRepository`, `(*ClientRepository).Create(ctx, name string) (ClientBusiness, error)`, `(*ClientRepository).Get(ctx, id uuid.UUID) (ClientBusiness, error)`, `(*ClientRepository).UpdateStatus(ctx, id uuid.UUID, status ClientStatus) (ClientBusiness, error)`. Used by Task 6 (Account creation looks up client status) and Task 7 (handlers).

- [ ] **Step 1: Write the failing test**

`services/accounts-service/internal/account/client_repository_test.go`:
```go
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

func TestClientRepository_CreateAndGet(t *testing.T) {
	conn := testutil.NewTestDB(t)
	repo := account.NewClientRepository(conn)

	created, err := repo.Create(context.Background(), "Acme Corp")
	require.NoError(t, err)
	assert.False(t, created.CreatedAt.IsZero())
	assert.Equal(t, account.ClientStatusActive, created.Status)

	fetched, err := repo.Get(context.Background(), created.ID)
	require.NoError(t, err)
	assert.Equal(t, created, fetched)
}

func TestClientRepository_Get_NotFound(t *testing.T) {
	conn := testutil.NewTestDB(t)
	repo := account.NewClientRepository(conn)

	_, err := repo.Get(context.Background(), uuid.New())
	assert.ErrorIs(t, err, account.ErrClientNotFound)
}

func TestClientRepository_UpdateStatus(t *testing.T) {
	conn := testutil.NewTestDB(t)
	repo := account.NewClientRepository(conn)

	created, err := repo.Create(context.Background(), "Acme Corp")
	require.NoError(t, err)

	updated, err := repo.UpdateStatus(context.Background(), created.ID, account.ClientStatusSuspended)
	require.NoError(t, err)
	assert.Equal(t, account.ClientStatusSuspended, updated.Status)
}

func TestClientRepository_UpdateStatus_RejectsInvalid(t *testing.T) {
	conn := testutil.NewTestDB(t)
	repo := account.NewClientRepository(conn)

	created, err := repo.Create(context.Background(), "Acme Corp")
	require.NoError(t, err)

	_, err = repo.UpdateStatus(context.Background(), created.ID, account.ClientStatus("bogus"))
	assert.ErrorIs(t, err, account.ErrInvalidClientStatus)
}

func TestClientRepository_UpdateStatus_NotFound(t *testing.T) {
	conn := testutil.NewTestDB(t)
	repo := account.NewClientRepository(conn)

	_, err := repo.UpdateStatus(context.Background(), uuid.New(), account.ClientStatusSuspended)
	assert.ErrorIs(t, err, account.ErrClientNotFound)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd services/accounts-service && go test ./internal/account/... -run TestClientRepository -v`
Expected: FAIL — compile error, `account.NewClientRepository` undefined.

- [ ] **Step 3: Write the implementation**

`services/accounts-service/internal/account/client_repository.go`:
```go
package account

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

type ClientRepository struct {
	db *sql.DB
}

func NewClientRepository(db *sql.DB) *ClientRepository {
	return &ClientRepository{db: db}
}

func (r *ClientRepository) Create(ctx context.Context, name string) (ClientBusiness, error) {
	client, err := NewClientBusiness(name)
	if err != nil {
		return ClientBusiness{}, err
	}

	err = r.db.QueryRowContext(ctx, `
		INSERT INTO client_businesses (id, name, status)
		VALUES ($1, $2, $3)
		RETURNING created_at
	`, client.ID, client.Name, string(client.Status)).Scan(&client.CreatedAt)
	if err != nil {
		return ClientBusiness{}, fmt.Errorf("insert client: %w", err)
	}

	return client, nil
}

func (r *ClientRepository) Get(ctx context.Context, id uuid.UUID) (ClientBusiness, error) {
	var (
		client ClientBusiness
		status string
	)
	err := r.db.QueryRowContext(ctx, `
		SELECT id, name, status, created_at FROM client_businesses WHERE id = $1
	`, id).Scan(&client.ID, &client.Name, &status, &client.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ClientBusiness{}, ErrClientNotFound
	}
	if err != nil {
		return ClientBusiness{}, fmt.Errorf("get client: %w", err)
	}
	client.Status = ClientStatus(status)
	return client, nil
}

func (r *ClientRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status ClientStatus) (ClientBusiness, error) {
	if !status.Valid() {
		return ClientBusiness{}, ErrInvalidClientStatus
	}

	var (
		client       ClientBusiness
		storedStatus string
	)
	err := r.db.QueryRowContext(ctx, `
		UPDATE client_businesses SET status = $1 WHERE id = $2
		RETURNING id, name, status, created_at
	`, string(status), id).Scan(&client.ID, &client.Name, &storedStatus, &client.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ClientBusiness{}, ErrClientNotFound
	}
	if err != nil {
		return ClientBusiness{}, fmt.Errorf("update client status: %w", err)
	}
	client.Status = ClientStatus(storedStatus)
	return client, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd services/accounts-service && go test ./internal/account/... -run TestClientRepository -v`
Expected: PASS — all 5 tests pass (requires Docker running).

- [ ] **Step 5: Commit**

```bash
git add services/accounts-service/internal/account/client_repository.go services/accounts-service/internal/account/client_repository_test.go
git commit -m "feat(accounts-service): add ClientBusiness repository"
```

---

### Task 6: Account repository — includes the suspended-client business rule (TDD, ⚠ manual review required)

**Files:**
- Create: `services/accounts-service/internal/account/account_repository.go`
- Test: `services/accounts-service/internal/account/account_repository_test.go`

**Interfaces:**
- Consumes: `account.NewAccount`, `account.CanCreateAccount`, `account.ClientStatus`, `account.ErrClientNotFound`, `account.ErrClientSuspended`, `account.ErrAccountNotFound`, `account.ErrInvalidAccountStatus` (Task 2); `testutil.NewTestDB` (Task 4); reads the `client_businesses` table populated by `ClientRepository` (Task 5).
- Produces: `account.NewAccountRepository(db *sql.DB) *AccountRepository`, `(*AccountRepository).Create(ctx, clientID uuid.UUID, externalRef string) (Account, error)`, `(*AccountRepository).Get(ctx, id uuid.UUID) (Account, error)`, `(*AccountRepository).ListByClient(ctx, clientID uuid.UUID) ([]Account, error)`, `(*AccountRepository).UpdateStatus(ctx, id uuid.UUID, status AccountStatus) (Account, error)`. Used by Task 8 (handlers).

⚠ **Before merging this task:** personally read `Create`'s implementation and confirm it rejects account creation when the parent client is suspended (`ErrClientSuspended`) and rejects it when the parent client doesn't exist (`ErrClientNotFound`) — in that order, since a nonexistent client has no status to check. Don't merge on a subagent's test-pass report alone.

- [ ] **Step 1: Write the failing test**

`services/accounts-service/internal/account/account_repository_test.go`:
```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd services/accounts-service && go test ./internal/account/... -run TestAccountRepository -v`
Expected: FAIL — compile error, `account.NewAccountRepository` undefined.

- [ ] **Step 3: Write the implementation**

`services/accounts-service/internal/account/account_repository.go`:
```go
package account

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

type AccountRepository struct {
	db *sql.DB
}

func NewAccountRepository(db *sql.DB) *AccountRepository {
	return &AccountRepository{db: db}
}

// Create inserts a new Account under clientID, after checking that the
// parent ClientBusiness exists and is not suspended (see CanCreateAccount).
func (r *AccountRepository) Create(ctx context.Context, clientID uuid.UUID, externalRef string) (Account, error) {
	var clientStatus string
	err := r.db.QueryRowContext(ctx, `
		SELECT status FROM client_businesses WHERE id = $1
	`, clientID).Scan(&clientStatus)
	if errors.Is(err, sql.ErrNoRows) {
		return Account{}, ErrClientNotFound
	}
	if err != nil {
		return Account{}, fmt.Errorf("lookup client: %w", err)
	}
	if !CanCreateAccount(ClientStatus(clientStatus)) {
		return Account{}, ErrClientSuspended
	}

	acc := NewAccount(clientID, externalRef)
	err = r.db.QueryRowContext(ctx, `
		INSERT INTO accounts (id, client_id, external_ref, status)
		VALUES ($1, $2, $3, $4)
		RETURNING created_at, updated_at
	`, acc.ID, acc.ClientID, acc.ExternalRef, string(acc.Status)).Scan(&acc.CreatedAt, &acc.UpdatedAt)
	if err != nil {
		return Account{}, fmt.Errorf("insert account: %w", err)
	}

	return acc, nil
}

func (r *AccountRepository) Get(ctx context.Context, id uuid.UUID) (Account, error) {
	var (
		acc    Account
		status string
	)
	err := r.db.QueryRowContext(ctx, `
		SELECT id, client_id, external_ref, status, created_at, updated_at
		FROM accounts WHERE id = $1
	`, id).Scan(&acc.ID, &acc.ClientID, &acc.ExternalRef, &status, &acc.CreatedAt, &acc.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Account{}, ErrAccountNotFound
	}
	if err != nil {
		return Account{}, fmt.Errorf("get account: %w", err)
	}
	acc.Status = AccountStatus(status)
	return acc, nil
}

func (r *AccountRepository) ListByClient(ctx context.Context, clientID uuid.UUID) ([]Account, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, client_id, external_ref, status, created_at, updated_at
		FROM accounts WHERE client_id = $1 ORDER BY created_at
	`, clientID)
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}
	defer rows.Close()

	var accounts []Account
	for rows.Next() {
		var (
			acc    Account
			status string
		)
		if err := rows.Scan(&acc.ID, &acc.ClientID, &acc.ExternalRef, &status, &acc.CreatedAt, &acc.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan account: %w", err)
		}
		acc.Status = AccountStatus(status)
		accounts = append(accounts, acc)
	}
	return accounts, rows.Err()
}

func (r *AccountRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status AccountStatus) (Account, error) {
	if !status.Valid() {
		return Account{}, ErrInvalidAccountStatus
	}

	var (
		acc          Account
		storedStatus string
	)
	err := r.db.QueryRowContext(ctx, `
		UPDATE accounts SET status = $1, updated_at = now() WHERE id = $2
		RETURNING id, client_id, external_ref, status, created_at, updated_at
	`, string(status), id).Scan(&acc.ID, &acc.ClientID, &acc.ExternalRef, &storedStatus, &acc.CreatedAt, &acc.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Account{}, ErrAccountNotFound
	}
	if err != nil {
		return Account{}, fmt.Errorf("update account status: %w", err)
	}
	acc.Status = AccountStatus(storedStatus)
	return acc, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd services/accounts-service && go test ./internal/account/... -run TestAccountRepository -v`
Expected: PASS — all 8 tests pass (requires Docker running).

- [ ] **Step 5: Manual review**

Read `Create` end to end. Confirm: (a) client lookup happens before the insert, (b) a missing client returns `ErrClientNotFound` before the status check ever runs, (c) a suspended client returns `ErrClientSuspended`, (d) only an active client reaches the insert. Do not skip this because tests are green — this is the rule the spec calls out as balance-of-obligation-adjacent.

- [ ] **Step 6: Commit**

```bash
git add services/accounts-service/internal/account/account_repository.go services/accounts-service/internal/account/account_repository_test.go
git commit -m "feat(accounts-service): add Account repository with suspended-client rule"
```

---

### Task 7: HTTP API — ClientBusiness handlers, shared plumbing, and router (TDD)

**Files:**
- Create: `services/accounts-service/internal/api/handlers.go`
- Create: `services/accounts-service/internal/api/handlers_client.go`
- Create: `services/accounts-service/internal/api/router.go`
- Test: `services/accounts-service/internal/api/handlers_client_test.go`

**Interfaces:**
- Consumes: `account.NewClientRepository`, `account.NewAccountRepository` (Tasks 5, 6); `account.ClientBusiness`, `account.ClientStatus`, `account.ErrEmptyClientName`, `account.ErrInvalidClientStatus`, `account.ErrClientNotFound` (Task 2); `testutil.NewTestDB` (Task 4).
- Produces: `api.NewHandlers(clients *account.ClientRepository, accounts *account.AccountRepository) *Handlers`, `api.NewRouter(h *Handlers) http.Handler`, `(*Handlers).Health`, `(*Handlers).CreateClient`, `(*Handlers).GetClient`, `(*Handlers).UpdateClientStatus`, package-level `writeJSON`/`writeError`/`timeFormat`. Used unchanged by Task 8 (which adds Account handlers to the same `Handlers` struct and router) and Task 9 (`main.go`).

- [ ] **Step 1: Write the failing test**

`services/accounts-service/internal/api/handlers_client_test.go`:
```go
package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/phuoctmse/settleguard/accounts-service/internal/account"
	"github.com/phuoctmse/settleguard/accounts-service/internal/api"
	"github.com/phuoctmse/settleguard/accounts-service/internal/testutil"
)

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	conn := testutil.NewTestDB(t)
	clients := account.NewClientRepository(conn)
	accounts := account.NewAccountRepository(conn)
	handlers := api.NewHandlers(clients, accounts)
	server := httptest.NewServer(api.NewRouter(handlers))
	t.Cleanup(server.Close)
	return server
}

func TestCreateClient(t *testing.T) {
	server := newTestServer(t)

	body, err := json.Marshal(map[string]any{"name": "Acme Corp"})
	require.NoError(t, err)

	resp, err := http.Post(server.URL+"/clients", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var created map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&created))
	assert.Equal(t, "Acme Corp", created["name"])
	assert.Equal(t, "active", created["status"])
}

func TestCreateClient_RejectsEmptyName(t *testing.T) {
	server := newTestServer(t)

	body, err := json.Marshal(map[string]any{"name": ""})
	require.NoError(t, err)

	resp, err := http.Post(server.URL+"/clients", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestGetClient_NotFound(t *testing.T) {
	server := newTestServer(t)

	resp, err := http.Get(server.URL + "/clients/00000000-0000-0000-0000-000000000000")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestUpdateClientStatus(t *testing.T) {
	server := newTestServer(t)

	createBody, err := json.Marshal(map[string]any{"name": "Acme Corp"})
	require.NoError(t, err)
	createResp, err := http.Post(server.URL+"/clients", "application/json", bytes.NewReader(createBody))
	require.NoError(t, err)
	defer createResp.Body.Close()
	var created map[string]any
	require.NoError(t, json.NewDecoder(createResp.Body).Decode(&created))

	statusBody, err := json.Marshal(map[string]any{"status": "suspended"})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPatch, server.URL+"/clients/"+created["id"].(string)+"/status", bytes.NewReader(statusBody))
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var updated map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&updated))
	assert.Equal(t, "suspended", updated["status"])
}

func TestUpdateClientStatus_RejectsInvalid(t *testing.T) {
	server := newTestServer(t)

	createBody, err := json.Marshal(map[string]any{"name": "Acme Corp"})
	require.NoError(t, err)
	createResp, err := http.Post(server.URL+"/clients", "application/json", bytes.NewReader(createBody))
	require.NoError(t, err)
	defer createResp.Body.Close()
	var created map[string]any
	require.NoError(t, json.NewDecoder(createResp.Body).Decode(&created))

	statusBody, err := json.Marshal(map[string]any{"status": "bogus"})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPatch, server.URL+"/clients/"+created["id"].(string)+"/status", bytes.NewReader(statusBody))
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd services/accounts-service && go test ./internal/api/... -v`
Expected: FAIL — compile error, `api.NewHandlers`, `api.NewRouter` undefined.

- [ ] **Step 3: Write the shared handlers plumbing**

`services/accounts-service/internal/api/handlers.go`:
```go
package api

import (
	"encoding/json"
	"net/http"

	"github.com/phuoctmse/settleguard/accounts-service/internal/account"
)

const timeFormat = "2006-01-02T15:04:05.000Z07:00"

type Handlers struct {
	clients  *account.ClientRepository
	accounts *account.AccountRepository
}

func NewHandlers(clients *account.ClientRepository, accounts *account.AccountRepository) *Handlers {
	return &Handlers{clients: clients, accounts: accounts}
}

func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
```

- [ ] **Step 4: Write the ClientBusiness handlers**

`services/accounts-service/internal/api/handlers_client.go`:
```go
package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/phuoctmse/settleguard/accounts-service/internal/account"
)

type clientResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

func toClientResponse(c account.ClientBusiness) clientResponse {
	return clientResponse{
		ID:        c.ID.String(),
		Name:      c.Name,
		Status:    string(c.Status),
		CreatedAt: c.CreatedAt.Format(timeFormat),
	}
}

type createClientRequest struct {
	Name string `json:"name"`
}

func (h *Handlers) CreateClient(w http.ResponseWriter, r *http.Request) {
	var req createClientRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	client, err := h.clients.Create(r.Context(), req.Name)
	if err != nil {
		if errors.Is(err, account.ErrEmptyClientName) {
			writeError(w, http.StatusBadRequest, "name is required")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusCreated, toClientResponse(client))
}

func (h *Handlers) GetClient(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid client id")
		return
	}

	client, err := h.clients.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, account.ErrClientNotFound) {
			writeError(w, http.StatusNotFound, "client not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, toClientResponse(client))
}

type updateClientStatusRequest struct {
	Status string `json:"status"`
}

func (h *Handlers) UpdateClientStatus(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid client id")
		return
	}

	var req updateClientStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	client, err := h.clients.UpdateStatus(r.Context(), id, account.ClientStatus(req.Status))
	if err != nil {
		switch {
		case errors.Is(err, account.ErrInvalidClientStatus):
			writeError(w, http.StatusBadRequest, "invalid status")
		case errors.Is(err, account.ErrClientNotFound):
			writeError(w, http.StatusNotFound, "client not found")
		default:
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	writeJSON(w, http.StatusOK, toClientResponse(client))
}
```

- [ ] **Step 5: Write the router**

`services/accounts-service/internal/api/router.go`:
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

	r.Post("/clients", h.CreateClient)
	r.Get("/clients/{id}", h.GetClient)
	r.Patch("/clients/{id}/status", h.UpdateClientStatus)

	return r
}
```

- [ ] **Step 6: Run test to verify it passes**

Run: `cd services/accounts-service && go test ./internal/api/... -v`
Expected: PASS — all 5 tests pass (requires Docker running).

- [ ] **Step 7: Commit**

```bash
git add services/accounts-service/internal/api/handlers.go services/accounts-service/internal/api/handlers_client.go services/accounts-service/internal/api/router.go services/accounts-service/internal/api/handlers_client_test.go
git commit -m "feat(accounts-service): add HTTP API for ClientBusiness"
```

---

### Task 8: HTTP API — Account handlers (TDD, ⚠ manual review required)

**Files:**
- Create: `services/accounts-service/internal/api/handlers_account.go`
- Modify: `services/accounts-service/internal/api/router.go`
- Test: `services/accounts-service/internal/api/handlers_account_test.go`

**Interfaces:**
- Consumes: `Handlers`, `writeJSON`, `writeError`, `timeFormat` (Task 7); `account.Account`, `account.AccountStatus`, `account.ErrClientNotFound`, `account.ErrClientSuspended`, `account.ErrAccountNotFound`, `account.ErrInvalidAccountStatus` (Task 2); `AccountRepository` methods (Task 6).
- Produces: `(*Handlers).CreateAccount`, `(*Handlers).GetAccount`, `(*Handlers).ListAccounts`, `(*Handlers).UpdateAccountStatus`, wired into `NewRouter`. Used by Task 9 (`main.go`) indirectly via `NewRouter`.

⚠ **Before merging this task:** confirm `CreateAccount` maps `ErrClientSuspended` to `422` and `ErrClientNotFound` to `404` — these are the HTTP-boundary expression of the same rule flagged in Task 6, and a status-code mixup here would silently let a client-side integration treat "blocked by policy" the same as "doesn't exist."

- [ ] **Step 1: Write the failing test**

`services/accounts-service/internal/api/handlers_account_test.go`:
```go
package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createClientFixture(t *testing.T, server *httptest.Server, name string) map[string]any {
	t.Helper()
	body, err := json.Marshal(map[string]any{"name": name})
	require.NoError(t, err)
	resp, err := http.Post(server.URL+"/clients", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var created map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&created))
	return created
}

func TestCreateAccount(t *testing.T) {
	server := newTestServer(t)
	client := createClientFixture(t, server, "Acme Corp")

	body, err := json.Marshal(map[string]any{"client_id": client["id"], "external_ref": "ext-1"})
	require.NoError(t, err)
	resp, err := http.Post(server.URL+"/accounts", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var created map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&created))
	assert.Equal(t, client["id"], created["client_id"])
	assert.Equal(t, "active", created["status"])
}

func TestCreateAccount_RejectsSuspendedClient(t *testing.T) {
	server := newTestServer(t)
	client := createClientFixture(t, server, "Acme Corp")

	statusBody, err := json.Marshal(map[string]any{"status": "suspended"})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPatch, server.URL+"/clients/"+client["id"].(string)+"/status", bytes.NewReader(statusBody))
	require.NoError(t, err)
	statusResp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer statusResp.Body.Close()
	require.Equal(t, http.StatusOK, statusResp.StatusCode)

	body, err := json.Marshal(map[string]any{"client_id": client["id"]})
	require.NoError(t, err)
	resp, err := http.Post(server.URL+"/accounts", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
}

func TestCreateAccount_UnknownClient(t *testing.T) {
	server := newTestServer(t)

	body, err := json.Marshal(map[string]any{"client_id": "00000000-0000-0000-0000-000000000000"})
	require.NoError(t, err)
	resp, err := http.Post(server.URL+"/accounts", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestListAccounts_ByClient(t *testing.T) {
	server := newTestServer(t)
	client := createClientFixture(t, server, "Acme Corp")

	body, err := json.Marshal(map[string]any{"client_id": client["id"]})
	require.NoError(t, err)
	createResp, err := http.Post(server.URL+"/accounts", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer createResp.Body.Close()
	require.Equal(t, http.StatusCreated, createResp.StatusCode)

	resp, err := http.Get(server.URL + "/accounts?client_id=" + client["id"].(string))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var listed []map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&listed))
	assert.Len(t, listed, 1)
}

func TestListAccounts_RequiresClientID(t *testing.T) {
	server := newTestServer(t)

	resp, err := http.Get(server.URL + "/accounts")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestUpdateAccountStatus(t *testing.T) {
	server := newTestServer(t)
	client := createClientFixture(t, server, "Acme Corp")

	body, err := json.Marshal(map[string]any{"client_id": client["id"]})
	require.NoError(t, err)
	createResp, err := http.Post(server.URL+"/accounts", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer createResp.Body.Close()
	var created map[string]any
	require.NoError(t, json.NewDecoder(createResp.Body).Decode(&created))

	statusBody, err := json.Marshal(map[string]any{"status": "closed"})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPatch, server.URL+"/accounts/"+created["id"].(string)+"/status", bytes.NewReader(statusBody))
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var updated map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&updated))
	assert.Equal(t, "closed", updated["status"])
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd services/accounts-service && go test ./internal/api/... -run "TestCreateAccount|TestListAccounts|TestUpdateAccountStatus" -v`
Expected: FAIL — compile error, `h.CreateAccount` etc. undefined, and `/accounts` routes don't exist yet.

- [ ] **Step 3: Write the Account handlers**

`services/accounts-service/internal/api/handlers_account.go`:
```go
package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/phuoctmse/settleguard/accounts-service/internal/account"
)

type accountResponse struct {
	ID          string `json:"id"`
	ClientID    string `json:"client_id"`
	ExternalRef string `json:"external_ref,omitempty"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

func toAccountResponse(a account.Account) accountResponse {
	return accountResponse{
		ID:          a.ID.String(),
		ClientID:    a.ClientID.String(),
		ExternalRef: a.ExternalRef,
		Status:      string(a.Status),
		CreatedAt:   a.CreatedAt.Format(timeFormat),
		UpdatedAt:   a.UpdatedAt.Format(timeFormat),
	}
}

type createAccountRequest struct {
	ClientID    string `json:"client_id"`
	ExternalRef string `json:"external_ref"`
}

func (h *Handlers) CreateAccount(w http.ResponseWriter, r *http.Request) {
	var req createAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	clientID, err := uuid.Parse(req.ClientID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid client_id")
		return
	}

	acc, err := h.accounts.Create(r.Context(), clientID, req.ExternalRef)
	if err != nil {
		switch {
		case errors.Is(err, account.ErrClientNotFound):
			writeError(w, http.StatusNotFound, "client not found")
		case errors.Is(err, account.ErrClientSuspended):
			writeError(w, http.StatusUnprocessableEntity, "client is suspended")
		default:
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	writeJSON(w, http.StatusCreated, toAccountResponse(acc))
}

func (h *Handlers) GetAccount(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid account id")
		return
	}

	acc, err := h.accounts.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, account.ErrAccountNotFound) {
			writeError(w, http.StatusNotFound, "account not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, toAccountResponse(acc))
}

func (h *Handlers) ListAccounts(w http.ResponseWriter, r *http.Request) {
	clientParam := r.URL.Query().Get("client_id")
	if clientParam == "" {
		writeError(w, http.StatusBadRequest, "client_id query param is required")
		return
	}

	clientID, err := uuid.Parse(clientParam)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid client_id")
		return
	}

	accounts, err := h.accounts.ListByClient(r.Context(), clientID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	resp := make([]accountResponse, len(accounts))
	for i, a := range accounts {
		resp[i] = toAccountResponse(a)
	}
	writeJSON(w, http.StatusOK, resp)
}

type updateAccountStatusRequest struct {
	Status string `json:"status"`
}

func (h *Handlers) UpdateAccountStatus(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid account id")
		return
	}

	var req updateAccountStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	acc, err := h.accounts.UpdateStatus(r.Context(), id, account.AccountStatus(req.Status))
	if err != nil {
		switch {
		case errors.Is(err, account.ErrInvalidAccountStatus):
			writeError(w, http.StatusBadRequest, "invalid status")
		case errors.Is(err, account.ErrAccountNotFound):
			writeError(w, http.StatusNotFound, "account not found")
		default:
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	writeJSON(w, http.StatusOK, toAccountResponse(acc))
}
```

- [ ] **Step 4: Wire the Account routes into the router**

In `services/accounts-service/internal/api/router.go`, add these three lines inside `NewRouter`, after the existing `/clients` routes:
```go
	r.Post("/accounts", h.CreateAccount)
	r.Get("/accounts", h.ListAccounts)
	r.Get("/accounts/{id}", h.GetAccount)
	r.Patch("/accounts/{id}/status", h.UpdateAccountStatus)
```

- [ ] **Step 5: Run test to verify it passes**

Run: `cd services/accounts-service && go test ./internal/api/... -v`
Expected: PASS — all tests across `handlers_client_test.go` and `handlers_account_test.go` pass (requires Docker running).

- [ ] **Step 6: Manual review**

Confirm the `CreateAccount` switch in Step 3 checks `ErrClientNotFound` and `ErrClientSuspended` before falling through to the generic `500` case, and that the status codes are `404` and `422` respectively (not swapped).

- [ ] **Step 7: Commit**

```bash
git add services/accounts-service/internal/api/handlers_account.go services/accounts-service/internal/api/router.go services/accounts-service/internal/api/handlers_account_test.go
git commit -m "feat(accounts-service): add HTTP API for Account, enforcing suspended-client rule"
```

---

### Task 9: Wire up main.go and verify the whole module

**Files:**
- Create: `services/accounts-service/cmd/server/main.go`

- [ ] **Step 1: Write main.go**

`services/accounts-service/cmd/server/main.go`:
```go
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/phuoctmse/settleguard/accounts-service/internal/account"
	"github.com/phuoctmse/settleguard/accounts-service/internal/api"
	"github.com/phuoctmse/settleguard/accounts-service/internal/db"
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

	clients := account.NewClientRepository(conn)
	accounts := account.NewAccountRepository(conn)
	handlers := api.NewHandlers(clients, accounts)
	router := api.NewRouter(handlers)

	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = ":8081"
	}

	log.Printf("accounts-service listening on %s", addr)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatal(err)
	}
}
```

`LISTEN_ADDR` defaults to `:8081` (not `:8080`) so accounts-service and ledger-service can run side by side locally without a port clash.

- [ ] **Step 2: Verify the whole module builds**

Run: `cd services/accounts-service && go build ./...`
Expected: exits with no output and status 0.

- [ ] **Step 3: Verify lint is clean**

Run: `cd services/accounts-service && golangci-lint run ./...`
Expected: no issues reported (enforces `docs/CODING_STANDARDS.md` via `.golangci.yml`: `errcheck`, `govet`, `staticcheck`, `unused`, `revive`, `gofmt`, `goimports`).

- [ ] **Step 4: Verify all tests still pass**

Run: `cd services/accounts-service && go vet ./... && go test ./... -v`
Expected: `go vet` produces no output; all tests across `internal/account`, `internal/db`, and `internal/api` PASS (requires Docker running).

- [ ] **Step 5: Commit**

```bash
git add services/accounts-service/cmd/server/main.go
git commit -m "feat(accounts-service): wire up main entrypoint"
```

---

### Task 10: Service README and CLAUDE.md update

**Files:**
- Create: `services/accounts-service/README.md`
- Modify: `CLAUDE.md` (root)

- [ ] **Step 1: Write the service README**

`services/accounts-service/README.md`:
```markdown
# accounts-service

Owns party/account identity and status: `ClientBusiness` (tenant) and
`Account` (a `ClientBusiness`'s end-user — the same `account_id` that
`ledger-service` records entries against). See
`docs/superpowers/specs/2026-08-08-accounts-service-mvp-design.md` for the
full design and `docs/PROJECT_CHARTER.md` for system-wide context.

## Run locally

Requires a reachable Postgres instance.

```bash
export DATABASE_URL="postgres://accounts:accounts@localhost:5432/accounts?sslmode=disable"
go run ./cmd/server
```

Migrations run automatically on startup. Listens on `:8081` by default
(override with `LISTEN_ADDR`) so it can run alongside `ledger-service`
locally.

## Build

```bash
go build ./...
```

## Lint

```bash
golangci-lint run ./...
```

## Test

Requires Docker (tests use testcontainers-go to run against a real Postgres
instance).

```bash
go test ./...
```

Run a single test:

```bash
go test ./internal/account/... -run TestCanCreateAccount -v
```

## API

- `GET /health` — health check
- `POST /clients` — body `{"name": "<string>"}` → `201` ClientBusiness. `400` if `name` is empty.
- `GET /clients/{id}` — `200` ClientBusiness or `404`.
- `PATCH /clients/{id}/status` — body `{"status": "active"|"suspended"}` → `200` updated ClientBusiness, `400` if status is invalid, `404` if not found.
- `POST /accounts` — body `{"client_id": "<uuid>", "external_ref": "<string, optional>"}` → `201` Account. `400` if `client_id` isn't a valid UUID, `404` if the client doesn't exist, `422` if the client is suspended.
- `GET /accounts/{id}` — `200` Account or `404`.
- `GET /accounts?client_id=<uuid>` — `200` list of Accounts under that client (empty list, not an error, if none). `400` if `client_id` is missing or invalid.
- `PATCH /accounts/{id}/status` — body `{"status": "active"|"suspended"|"closed"}` → `200` updated Account, `400` if status is invalid, `404` if not found.
```

- [ ] **Step 2: Update root CLAUDE.md**

In `CLAUDE.md`, replace the "Project Status" section with:

```markdown
## Project Status

`services/ledger-service` and `services/accounts-service` have working
MVPs (see each service's README.md for build/lint/test/run commands). The
remaining two services (`notification-service`, `settlement-engine`) and
`mobile-app` are still scaffolds with no code. As each is implemented, add
its build/lint/test commands to this file rather than assuming conventions
from the stack.
```

- [ ] **Step 3: Commit**

```bash
git add services/accounts-service/README.md CLAUDE.md
git commit -m "docs(accounts-service): add service README and update root CLAUDE.md"
```

---

## Explicitly Deferred (not in this plan)

Per spec §7:

- Balance-of-obligation on `Account` — needs the event broker decision (`ledger.entry-recorded` → balance update)
- Publishing `account.updated` events — same event-broker dependency
- Authentication/authorization on the HTTP API (client business auth via API key) — deferred alongside the same decision for `ledger-service`
- Dockerfile / k8s manifest for accounts-service
- A generalized `Party` union type replacing the concrete `ClientBusiness` + `Account` split — no concrete need for it yet
- A state machine constraining status transitions (e.g. disallowing `closed` → `active`) — any valid status value is accepted for now; add constraints when a real use case demands them
- `settlement-engine`, `notification-service`, `mobile-app` (separate plans, later in the roadmap)
