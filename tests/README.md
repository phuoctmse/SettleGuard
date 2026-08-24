# tests/

Cross-service test suite for SettleGuard: `tests/api` (API/contract tests
across all 4 backend services), `tests/security` (fraud-bypass tests
against settlement-engine's risk rules), and `tests/perf` (Locust load
test for settlement-engine's real-time scoring path). All three run HTTP
requests against services you start yourself locally — no mocking, no
testcontainers here (that's how the individual Go/Python services test
themselves; this suite tests the system from the outside).

## Scope

**In scope (this suite):**

- `tests/api` — contract tests per service (`test_accounts_contract.py`,
  `test_ledger_contract.py`, `test_settlement_contract.py`,
  `test_notification_contract.py`) plus one cross-service end-to-end flow
  (`test_end_to_end_flow.py`) that posts a transaction through
  ledger-service and follows it through settlement-engine and
  notification-service.
- `tests/security` — fraud-bypass tests only (`test_fraud_bypass.py`):
  velocity limit, mismatch threshold, blocklist, double-approve, and
  held-transaction-excluded-from-settlement.
- `tests/perf` — a Locust file load-testing the real-time transaction
  scoring path via `POST /transactions` on ledger-service.

**Explicitly deferred (not in this suite, not accidentally skipped):**

- **`tests/ai-tools`** — blocked on two prerequisites that don't exist
  yet: a real OpenAPI spec (`docs/openapi.yaml` currently exists but is
  empty) and a CI pipeline (no `.github/workflows` yet) to triage. Both
  are large enough to be their own plan.
- **Auth boundary checks** (`tests/security`) — no service has auth yet;
  waiting on a project-wide auth decision.
- **Mobile Appium E2E** — waiting on `mobile-app` screens getting
  `testID`/`accessibilityLabel` attributes, which is `mobile-app`'s own
  work, not `tests/`.
- **CI wiring** to run `tests/api`/`tests/security` automatically —
  depends on CI existing at all, out of scope here.

See `docs/superpowers/specs/2026-08-24-testing-suite-design.md` §8 ("Để
lại cho việc khác") for the full rationale behind each of these.

Batch settlement *throughput* (as opposed to correctness) is also not
load-tested — see the note at the top of `locustfile_settlement_scoring.py`
for why (`RunBatch` is schedule-driven, not HTTP-triggered, so there's no
request to point Locust at).

## Prerequisites

- Docker (for Postgres + NATS)
- Go (to run `ledger-service`, `accounts-service`, `settlement-engine`)
- Python 3.11+ with a virtualenv of your choice

Install this suite's own dependencies:

```bash
pip install -r tests/requirements.txt
```

This pulls in `pytest`, `httpx`, `locust`, and `psycopg2-binary`.
`psycopg2-binary` exists solely for `tests/security`'s `seed_blocklist`
fixture (`tests/security/conftest.py`), which inserts rows directly into
settlement-engine's `blocklist` table — the one deliberate exception to
this suite's "talk to services only over HTTP" rule, because
settlement-engine has no HTTP endpoint to manage the blocklist.

## Starting the stack

All 4 services and NATS need to be up before running `tests/api` or
`tests/security`. `tests/perf` only needs `accounts-service` and
`ledger-service` (settlement-engine and notification-service can be up
too, but the load test itself doesn't call them directly).

**1. Infra — Postgres (x4) + NATS**, from the repo root:

```bash
docker compose -f infra/docker/docker-compose.yml up -d
```

This starts one Postgres container per service, each mapped to its own
host port (ledger `5433`, accounts `5434`, settlement `5435`,
notification `5436` — all mapping to container-internal `5432`), plus
NATS with JetStream enabled on `4222`. These port numbers come straight
from `infra/docker/docker-compose.yml`'s `ports:` mappings for each
service. **Note:** `services/ledger-service/README.md` and
`services/accounts-service/README.md` currently document `DATABASE_URL`
using port `5432` — that's a pre-existing inconsistency in those services'
own READMEs (unrelated to this test suite, not fixed here). Use the ports
below, which are what actually works against this compose file.

**2. Each service, in its own terminal, from the repo root:**

```bash
# Terminal 1 -- ledger-service
cd services/ledger-service
export DATABASE_URL="postgres://ledger:ledger@localhost:5433/ledger?sslmode=disable"
export NATS_URL="nats://localhost:4222"
go run ./cmd/server
```

```bash
# Terminal 2 -- accounts-service
cd services/accounts-service
export DATABASE_URL="postgres://accounts:accounts@localhost:5434/accounts?sslmode=disable"
export NATS_URL="nats://localhost:4222"
go run ./cmd/server
```

```bash
# Terminal 3 -- settlement-engine
cd services/settlement-engine
export DATABASE_URL="postgres://settlement:settlement@localhost:5435/settlement?sslmode=disable"
export NATS_URL="nats://localhost:4222"
export SETTLEMENT_BATCH_INTERVAL_SECONDS=5
go run ./cmd/server
```

`SETTLEMENT_BATCH_INTERVAL_SECONDS=5` matters: it defaults to 60s, and
both `tests/api/test_end_to_end_flow.py` and several `tests/security`
tests wait for a batch cycle to happen. At the default interval those
tests would still pass, just slowly (up to a minute per wait) — 5s keeps
the suite fast.

```bash
# Terminal 4 -- notification-service (Python; needs its own venv +
# golang-migrate's `migrate` CLI on PATH -- see
# services/notification-service/README.md)
cd services/notification-service
export DATABASE_URL="postgres://notification:notification@localhost:5436/notification?sslmode=disable"
export NATS_URL="nats://localhost:4222"
python main.py
```

Each service listens on its own default port: ledger `:8080`, accounts
`:8081`, settlement `:8082`, notification `:8083`. `tests/api/conftest.py`
and `tests/security/conftest.py` default to these same ports, overridable
via `ACCOUNTS_API_URL`, `LEDGER_API_URL`, `SETTLEMENT_API_URL`, and
`NOTIFICATION_API_URL` if you're running services elsewhere.

If a service isn't reachable at `/health` when a test session starts, the
corresponding session-scoped fixture in `tests/api/conftest.py` skips
(rather than errors) every test that depends on it, with a message
pointing back here.

## Running the suites

From the repo root (or from `tests/` — `tests/pyproject.toml` sets
`testpaths = ["api", "security"]` and `pythonpath = ["api"]` so
`tests/security` can import `tests/api`'s HTTP clients):

```bash
# API/contract tests, including the cross-service end-to-end flow
pytest tests/api

# Fraud-bypass security tests
pytest tests/security
```

Both suites share the same service fixtures (`accounts`, `ledger`,
`settlement`, `notifications` in `tests/api/conftest.py`); `tests/security`
additionally has `seed_blocklist` (`tests/security/conftest.py`), which
defaults to `SETTLEMENT_DATABASE_URL=postgres://settlement:settlement@localhost:5435/settlement?sslmode=disable`
(override if settlement-engine's Postgres is somewhere else).

Load test (point `--host` at ledger-service, since that's the actual
entry point settlement-engine's scoring path is exercised through —
settlement-engine itself has no HTTP endpoint that triggers scoring
directly):

```bash
locust -f tests/perf/locustfile_settlement_scoring.py --host=http://localhost:8080
```

This opens Locust's web UI (default `http://localhost:8089`) where you
set user count and spawn rate and watch live latency/throughput. Each
simulated user creates its own client + 2 accounts via
`accounts-service` on start, then repeatedly posts a 2-entry transaction
to `POST /transactions` on ledger-service.

## Troubleshooting

- **Tests skip with "not reachable at .../health"** — that service isn't
  running, or is on a different port than the suite expects; check the
  terminal it should be running in, or set the matching `*_API_URL` env
  var.
- **`tests/security` waits and then times out on a hold/settle
  assertion** — check `SETTLEMENT_BATCH_INTERVAL_SECONDS` was actually
  set to `5` before starting settlement-engine (the default 60s will
  eventually pass, just past most tests' `_wait_until` timeouts).
- **`seed_blocklist` fails to connect** — settlement-engine's Postgres
  isn't reachable at the default `SETTLEMENT_DATABASE_URL`; confirm
  `docker compose ... up -d` actually started `settlement-postgres` and
  that port `5435` is free on the host.
