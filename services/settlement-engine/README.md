# settlement-engine

The core orchestrator: consumes `ledger-service`'s `ledger.entry-recorded`
events, runs rule-based risk scoring on each transaction (publishing
`transaction.risk-scored`), and on a schedule batches all risk-passed
transactions into a settlement (publishing `settlement.finalized`). See
`docs/superpowers/plans/2026-08-10-settlement-engine-mvp.md` for the full
design and `docs/PROJECT_CHARTER.md` for system-wide context.

## Run locally

Requires a reachable Postgres instance and a NATS server with JetStream
enabled (`docker compose -f infra/docker/docker-compose.yml up -d
settlement-postgres ledger-postgres nats`), plus `ledger-service` running
so there's something to consume from.

```bash
export DATABASE_URL="postgres://settlement:settlement@localhost:5435/settlement?sslmode=disable"
export NATS_URL="nats://localhost:4222"
go run ./cmd/server
```

Migrations run automatically on startup. Listens on `:8082` by default
(override with `LISTEN_ADDR`) so it can run alongside `ledger-service`
(`:8080`) and `accounts-service` (`:8081`) locally.

## Risk scoring

Every consumed transaction is checked against three rules, independent of
each other (OR-based: any one triggering holds the transaction):

- **Velocity limit** — an account has more than `SETTLEMENT_VELOCITY_LIMIT`
  (default `5`) transactions in the trailing `SETTLEMENT_VELOCITY_WINDOW_MINUTES`
  (default `5`) minutes.
- **Mismatch threshold** — the transaction amount exceeds
  `SETTLEMENT_MISMATCH_THRESHOLD` (default `10000000`, i.e. 100,000.00 in
  the ledger's base unit).
- **Blocklist** — any touched account has a row in the `blocklist` table
  (`entity_type = 'account'`). Client-level blocking is schema-ready but
  inert in this MVP — `ledger.entry-recorded` never carries `client_id`.

`Score` (0-100, weights `velocity=40`, `mismatch=30`, `blocklist=100`,
summed and capped) is a separate informational/audit field — it does not
drive the hold/pass decision.

**Held transactions are resolved via `POST /transactions/{id}/approve` or
`/reject`** (see API section below) — `approve` moves a transaction back
into the normal settlement flow (`pending_settlement`, behaves identically
to a transaction that passed risk-scoring outright); `reject` moves it to
the terminal `rejected` status, which never enters a batch. Both publish a
`transaction.resolved` event. See `SETTLEMENT-05` in
`docs/BUSINESS_RULES.md`.

## Settlement batching

A background scheduler (`internal/settlement.Scheduler`) calls `RunBatch`
every `SETTLEMENT_BATCH_INTERVAL_SECONDS` (default `60`), grouping every
`pending_settlement` transaction into one new `settlements` row and
flipping them to `settled`. No time window is needed — `pending_settlement`
status itself means "not yet batched." A batch only ever contains
already-passed transactions, so it always publishes `settlement.finalized`
— there's no `settlement.held-for-review` in this MVP.

## Event contracts

- **Consumes** `ledger.entry-recorded` (stream `LEDGER_EVENTS`, owned by
  `ledger-service`) on durable consumer `settlement-engine-risk-scoring`.
  Idempotent via `processed_ledger_transactions`, dedup-keyed on
  `transaction_id`.
- **Publishes** `transaction.risk-scored`, `settlement.finalized`, and
  `transaction.resolved` (on `Approve`/`Reject`), all on stream
  `SETTLEMENT_EVENTS` (owned by this service), via the same
  transactional-outbox + relay pattern as `ledger-service`/`accounts-service`
  (`internal/outbox`), at-least-once with dedupe via `Nats-Msg-Id`.
  `notification-service` consumes `transaction.risk-scored` (hold decisions
  only) and `settlement.finalized`; nothing consumes `transaction.resolved`
  yet.

## Build

```bash
go build ./...
```

## Lint

```bash
golangci-lint run ./...
```

## Test

Requires Docker (tests use testcontainers-go to run against real Postgres
and NATS instances — no mocks).

```bash
go test ./...
```

Run a single test:

```bash
go test ./internal/risk/... -run TestScorer_Score_HoldOnVelocityLimit -v
```

## API

- `GET /health` — health check.
- `GET /transactions?status=<status>` — `200` list of Transaction filtered
  by status (`pending_settlement` | `held` | `settled` | `rejected`),
  most recently scored first. `400` if `status` is missing.
- `GET /transactions/{id}` — `200` Transaction or `404`.
- `POST /transactions/{id}/approve` — moves a `held` transaction to
  `pending_settlement`. `200` updated Transaction, `404` if not found,
  `409` if not currently `held`.
- `POST /transactions/{id}/reject` — moves a `held` transaction to the
  terminal `rejected` status. `200` updated Transaction, `404` if not
  found, `409` if not currently `held`.
- `GET /settlements` — `200` list of Settlement, most recently created
  first.
- `GET /settlements/{id}` — `200` Settlement or `404`.
