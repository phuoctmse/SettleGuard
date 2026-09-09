# SettleGuard

SettleGuard is a B2B platform that tracks and settles financial obligations
between parties on top of external payment rails (it never moves real money
itself), while guarding the settlement process with rule-based and ML-based
risk scoring. Client businesses integrate via API to settle payments/payouts
between their own users; SettleGuard's own mobile app lets end-users (or the
client's ops team) track settlement status and see fraud/risk alerts in real
time.

## Status

All four backend services and the mobile app have working MVPs merged to
`main`, plus a cross-service test suite in `tests/`. See `CLAUDE.md` for
architecture, stack, and working conventions, and each service's own
`README.md` for how to run, test, and call it.

## Services

Every backend service owns its own Postgres database (no shared schema) and
communicates over NATS JetStream rather than synchronous calls.

- `services/ledger-service` (Go, Postgres) — append-only ledger of
  obligations; publishes `ledger.entry-recorded`. Listens on `:8080`.
- `services/accounts-service` (Go, Postgres) — party/account identity and
  balances; publishes `account.updated`. Listens on `:8081`.
- `services/settlement-engine` (Go, Postgres) — rule-based risk scoring +
  batch settlement orchestration; publishes `transaction.risk-scored`,
  `settlement.finalized`, and `transaction.resolved`. Listens on `:8082`.
- `services/notification-service` (Python, Postgres) — terminal consumer of
  risk-hold and settlement-finalized events, persisted as a notification
  audit trail. Listens on `:8083`.
- `mobile-app` (Expo/TypeScript) — read-oriented client for the backend
  services, with approve/reject actions on held transactions.

## Local development

Postgres (one instance per service) and NATS JetStream run via
`infra/docker/docker-compose.yml`:

```bash
docker compose -f infra/docker/docker-compose.yml up -d
```

Copy `.env.example` (repo root) for the variable names each service reads,
and each service's own `.env.example` for its local Postgres credentials.

## Tests

- `tests/api` — cross-service contract tests plus one end-to-end flow
- `tests/security` — fraud-bypass tests against the risk rules
- `tests/perf` — Locust load test for the real-time scoring path

These run HTTP requests against services you start yourself; see
`tests/README.md`.
