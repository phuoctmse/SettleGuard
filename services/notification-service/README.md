# notification-service

Consumes `settlement-engine`'s `transaction.risk-scored` (hold decisions
only) and `settlement.finalized` events from NATS JetStream and persists
each as a row in `notifications` — an audit trail and insertion point for
a real delivery channel (email/push), neither of which exists yet. See
`docs/superpowers/specs/2026-08-16-notification-service-design.md` for
the full design rationale and `docs/PROJECT_CHARTER.md` for system-wide
context.

## Run locally

Requires a reachable Postgres instance, a NATS server with JetStream
enabled, and the `migrate` CLI (golang-migrate) on PATH:

```bash
docker compose -f infra/docker/docker-compose.yml up -d notification-postgres nats
cp services/notification-service/.env.example services/notification-service/.env  # first time only
```

```bash
cd services/notification-service
python -m venv .venv && source .venv/Scripts/activate  # if not already created
pip install -r requirements.txt
export DATABASE_URL="postgres://notification:notification@localhost:5436/notification?sslmode=disable"
export NATS_URL="nats://localhost:4222"
python main.py
```

Migrations run automatically on startup (via the `migrate` CLI). Listens
on `:8083` by default (override with `LISTEN_ADDR`) so it can run
alongside `ledger-service` (`:8080`), `accounts-service` (`:8081`), and
`settlement-engine` (`:8082`) locally.

## What gets notified

- **`transaction.risk-scored`** — only when `decision == "hold"`. Events
  with `decision == "pass"` are acknowledged and dropped (see
  `NOTIFICATION-01` in `docs/BUSINESS_RULES.md`).
- **`settlement.finalized`** — always.

Both are idempotent on `(type, subject_id)`: redelivery of the same event
(NATS JetStream is at-least-once) never creates a duplicate row.

## Event contracts

- **Consumes** `transaction.risk-scored` and `settlement.finalized`
  (stream `SETTLEMENT_EVENTS`, owned by `settlement-engine`), on durable
  consumers `notification-service-risk-hold` and
  `notification-service-settlement-finalized` respectively.
- **Publishes** nothing — this service is a terminal consumer in v1, so
  there is no outbox here.

## Test

Requires Docker (tests use `testcontainers-python` to run against real
Postgres and real NATS — no mocks):

```bash
pytest
```

Run a single test:

```bash
pytest internal/consumer/consumer_test.py::test_consumer_records_hold_skips_pass_and_dedupes -v
```

## API

- `GET /health` — health check. This service is event-driven end-to-end;
  there is no other HTTP surface.
