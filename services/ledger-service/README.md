# ledger-service

Append-only source of truth for double-entry ledger obligations. See
`docs/PROJECT_CHARTER.md` and
`docs/superpowers/specs/2026-07-29-project-charter-design.md` for the
system-wide context.

## Run locally

Requires a reachable Postgres instance and a NATS server with JetStream
enabled (`docker compose -f infra/docker/docker-compose.yml up -d
ledger-postgres nats`).

```bash
export DATABASE_URL="postgres://ledger:ledger@localhost:5432/ledger?sslmode=disable"
export NATS_URL="nats://localhost:4222"
go run ./cmd/server
```

Migrations run automatically on startup.

## Event publishing

Every `POST /transactions` call publishes one `ledger.entry-recorded`
event (covering the whole balanced transaction, not one event per entry)
to NATS JetStream, subject `ledger.entry-recorded` on stream
`LEDGER_EVENTS`. Delivery uses the transactional outbox pattern:

- `Repository.InsertTransaction` writes the ledger entries and one
  `outbox_events` row in the same DB transaction (`internal/ledger`).
- A background relay goroutine (`internal/outbox`) polls unpublished
  `outbox_events` rows every 500ms and publishes them to JetStream,
  marking `published_at` once the publish is acked.

This gives at-least-once delivery (JetStream dedupes by `Nats-Msg-Id` =
the outbox row's UUID, within its dedupe window) — consumers must still
be idempotent. There is no retry/backoff or dead-letter handling for rows
that fail to publish yet; they're retried every poll tick indefinitely.
See `docs/superpowers/plans/2026-08-10-ledger-service-nats-outbox.md` for
the full design.

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
