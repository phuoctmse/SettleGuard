# accounts-service

Owns party/account identity and status: `ClientBusiness` (tenant) and
`Account` (a `ClientBusiness`'s end-user — the same `account_id` that
`ledger-service` records entries against). See
`docs/superpowers/specs/2026-08-08-accounts-service-mvp-design.md` for the
full design and `docs/PROJECT_CHARTER.md` for system-wide context.

## Run locally

Requires a reachable Postgres instance and a NATS server with JetStream
enabled (`docker compose -f infra/docker/docker-compose.yml up -d
accounts-postgres nats`).

```bash
export DATABASE_URL="postgres://accounts:accounts@localhost:5432/accounts?sslmode=disable"
export NATS_URL="nats://localhost:4222"
go run ./cmd/server
```

Migrations run automatically on startup. Listens on `:8081` by default
(override with `LISTEN_ADDR`) so it can run alongside `ledger-service`
locally.

## Balance tracking

`Account.balance` is **eventually consistent**, not set synchronously on
account creation. It starts at `0` and is updated asynchronously by a
JetStream consumer (`internal/consumer`) that applies `ledger-service`'s
`ledger.entry-recorded` events: `balance += Σcredit − Σdebit` per
transaction. Idempotent against redelivery via `processed_ledger_transactions`
(dedup keyed on `transaction_id`). See
`docs/superpowers/plans/2026-08-10-accounts-service-balance-consumer.md`
for the full design.

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

Account responses include `"balance": <int64>` — see "Balance tracking"
above for how/when it updates.
