# SettleGuard

SettleGuard is a B2B platform that tracks and settles financial obligations
between parties on top of external payment rails (it never moves real money
itself), while guarding the settlement process with rule-based and ML-based
risk scoring. Client businesses integrate via API to settle payments/payouts
between their own users; SettleGuard's own mobile app lets end-users (or the
client's ops team) track settlement status and see fraud/risk alerts in real
time.

## Status

Scaffold only — no services are implemented yet. See `CLAUDE.md` for
architecture, stack, and working conventions.

## Planned Services

- `services/accounts-service` (Go, Postgres) — party/account identity, balances
- `services/ledger-service` (Go, Postgres) — append-only ledger of obligations
- `services/settlement-engine` (Go) — risk scoring + batch settlement orchestration
- `services/notification-service` (Python) — risk/settlement alerts
- `mobile-app` (React Native) — read-oriented client for end-users/ops
