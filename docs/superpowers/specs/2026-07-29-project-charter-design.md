# SettleGuard Project Charter — Design

Date: 2026-07-29
Status: Approved (pending final spec review)

## 1. Vision & Scope

**SettleGuard** is a B2B platform that tracks and settles financial obligations
between parties on top of external payment rails — it never moves real money
itself — while actively guarding the settlement process with rule-based and
ML-based risk scoring. Client businesses integrate via API to settle
payments/payouts between their own users. SettleGuard's own mobile app lets
end-users (or the client's ops team) track settlement status and see
fraud/risk alerts in real time.

### In scope (v1)

- Single-currency obligation tracking and settlement (accounts, ledger,
  settlement-engine)
- Real-time rule + ML risk scoring on transactions as they arrive
- Batched settlement runs that finalize risk-cleared obligations
- Notifications (push/email) for settlement events and risk holds
- Mobile app for settlement/alert visibility
- Cloud deployment (Kubernetes + Terraform)

### Out of scope (v1) — explicit non-goals

- Direct movement of real funds (no custody, no payout execution — delegated
  to an external payment processor)
- Multi-currency / FX
- On-premise deployment

## 2. Architecture & Domain Model

### Services (event-driven core)

- **accounts-service** (Go) — owns party/account identity, balances-of-obligation
  (not real money), and account status. Publishes `account.updated` events.
- **ledger-service** (Go) — the append-only source of truth for every
  obligation entry (double-entry style: who owes whom, why, current state).
  Publishes `ledger.entry-recorded` events. Postgres-backed.
- **settlement-engine** (Go) — the core orchestrator. Consumes ledger events,
  runs real-time rule + ML risk scoring per transaction, publishes
  `transaction.risk-scored` events (carrying score + hold/clear decision). On
  a schedule, batches all cleared entries since the last run into a
  settlement, publishing `settlement.finalized` or
  `settlement.held-for-review` events.
- **notification-service** (Python) — subscribes to risk-hold and
  settlement-finalized events; sends push/email alerts to the mobile app
  and/or client webhooks. Never called synchronously by other services.

### Data ownership

Each service owns its own schema; no cross-service direct database access.
Postgres is used by services that need persistence (at minimum ledger-service
and accounts-service).

### Event bus

An event broker (exact technology, e.g. Kafka or a managed equivalent, is an
implementation-plan decision, not a charter decision) is the backbone
connecting the four services. This is what makes the real-time-scoring +
batched-settlement hybrid work naturally: scoring reacts continuously to a
stream of transactions, and settlement periodically drains a window of
risk-cleared entries from that same stream.

### Key domain entities

Account, LedgerEntry, Transaction, RiskScore, Settlement (batch),
Alert/Notification.

### Mobile app (React Native)

Talks to accounts/ledger/settlement-engine via a read-oriented API (fronted
by a gateway or served directly by settlement-engine — an implementation-plan
decision) and receives push notifications from notification-service. The
mobile app is never a source of truth for domain data.

## 3. Testing, Tooling & Success Criteria

### Test layout (matches existing `tests/` skeleton)

- `tests/api` — cross-service API/contract tests (Python), run against the
  OpenAPI spec (to be written per-service as a follow-up to this charter)
- `tests/security` — security-focused tests (fraud-bypass attempts, auth
  boundary checks)
- `tests/perf` — load/perf tests for settlement-engine's real-time scoring
  path and batch settlement throughput
- `tests/ai-tools` — Python tooling for AI-assisted test generation and CI
  failure triage (distinct from the risk-scoring ML model itself, which
  lives inside settlement-engine)

### Mobile testing

Appium (Python client) drives the React Native app for end-to-end tests.

### Success criteria for v1

- A transaction flows end-to-end: recorded in ledger → risk-scored in real
  time → included in the next batch settlement (or held) → notification sent
- Held/flagged transactions are visible and actionable (approve/reject) from
  the mobile app
- All four services are independently deployable via Kubernetes manifests
  generated from Terraform-provisioned infrastructure

## 4. Tech Stack Summary

| Component            | Stack                                   |
|-----------------------|------------------------------------------|
| accounts-service      | Go, Postgres                             |
| ledger-service        | Go, Postgres                             |
| settlement-engine     | Go, Postgres (or event-store), ML scoring model |
| notification-service  | Python                                   |
| mobile-app            | React Native                             |
| Test automation       | Python (API, security, perf, AI tooling); Appium for mobile E2E |
| Infra                 | Kubernetes (`infra/k8s`), Terraform (`infra/terraform`) |

## 5. Deferred to Implementation Plan(s)

The following are intentionally left open here and will be decided when each
sub-project (per-service) is planned:

- Exact event broker choice and topic/schema design
- Per-service OpenAPI contracts (`docs/openapi.yaml` is currently an empty
  placeholder; it should be split or populated per service)
- ML risk-scoring model choice, training data, and serving approach
- Gateway/API-facade design for the mobile app
- CI/CD pipeline details beyond the k8s/terraform skeleton already present
