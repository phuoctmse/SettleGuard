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
