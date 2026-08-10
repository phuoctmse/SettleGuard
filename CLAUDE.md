# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Status

`services/ledger-service` has a working MVP (record + query ledger entries;
see `services/ledger-service/README.md` for build/lint/test/run commands).
The other three services (`accounts-service`, `notification-service`,
`settlement-engine`) and `mobile-app` are still scaffolds with no code. As
each is implemented, add its build/lint/test commands to this file rather
than assuming conventions from the stack.

## What SettleGuard Is

SettleGuard is a B2B platform that tracks and settles financial obligations
between parties on top of external payment rails (it never moves real money
itself), while guarding the settlement process with rule-based and ML-based
risk scoring. Client businesses integrate via API to settle payments/payouts
between their own users; SettleGuard's own mobile app lets end-users (or the
client's ops team) track settlement status and see fraud/risk alerts in real
time.

v1 explicitly does not: move real funds directly (that's delegated to an
external payment processor), support multi-currency/FX, or support
on-premise deployment.

Full charter: `docs/PROJECT_CHARTER.md`. Full architecture/domain-model design:
`docs/superpowers/specs/2026-07-29-project-charter-design.md`.

## Architecture

SettleGuard is event-driven: each service owns its own data and schema, with
no cross-service direct database access. Services communicate by publishing
and consuming events over a broker rather than calling each other
synchronously. Broker: **NATS JetStream** (decided 2026-08-10 — durability
+ per-subject ordering for ledger correctness, replay for risk-model
re-runs/audits, independent consumer groups, lighter ops than Kafka,
pure-Go client). `ledger-service` publishes `ledger.entry-recorded` via a
transactional outbox pattern (see `services/ledger-service/README.md`).
Other services' publishers/consumers are still pending.

- **`services/accounts-service`** (Go, Postgres) — owns party/account
  identity, balances-of-obligation, and account status. Publishes
  `account.updated`.
- **`services/ledger-service`** (Go, Postgres) — append-only source of truth
  for every obligation entry (double-entry style). Publishes
  `ledger.entry-recorded`.
- **`services/settlement-engine`** (Go) — the core orchestrator. Consumes
  ledger events, runs real-time rule + ML risk scoring per transaction
  (publishing `transaction.risk-scored`), and on a schedule batches all
  risk-cleared entries into a settlement (publishing `settlement.finalized`
  or `settlement.held-for-review`).
- **`services/notification-service`** (Python) — subscribes to risk-hold and
  settlement-finalized events; sends push/email alerts. Never called
  synchronously by other services.
- **`mobile-app`** (React Native) — read-oriented client for
  accounts/ledger/settlement-engine, receives push notifications from
  notification-service. Never a source of truth for domain data.

Key domain entities shared across services: Account, LedgerEntry,
Transaction, RiskScore, Settlement (batch), Alert/Notification.

## Testing Layout

- **`tests/api`** (Python) — cross-service API/contract tests, run against
  each service's OpenAPI spec once written.
- **`tests/security`** — fraud-bypass attempts, auth boundary checks.
- **`tests/perf`** — load/perf tests for settlement-engine's real-time
  scoring path and batch settlement throughput.
- **`tests/ai-tools`** (Python) — AI-assisted test generation and CI-failure
  triage tooling — distinct from the risk-scoring ML model itself, which
  lives inside settlement-engine.
- Mobile E2E tests use Appium via its Python client against `mobile-app`.

## Infra

- **`infra/k8s`** — Kubernetes manifests for deploying each service.
- **`infra/terraform`** — cloud infrastructure provisioning.

## Stack
 
- **Go** (accounts-service, ledger-service, settlement-engine):
  - `net/http` + `chi` router — no heavier framework (Gin, etc.)
  - `golang-migrate` for schema migrations — plain `.sql` files, no
    code-gen tooling (e.g. no sqlc)
  - `testcontainers-go` for DB tests — real Postgres in tests, no
    `sqlmock`/mocked driver
  - `nats.go` (`jetstream` subpackage) for NATS JetStream event
    publishing/consuming — real JetStream in tests via
    `testcontainers-go/modules/nats`, no broker mock
  - Standard layout per service: `cmd/server/main.go` (entrypoint) +
    `internal/{api,db,<domain>}/` (unimportable from outside the module)
  - Module path: `github.com/phuoctmse/settleguard/<service-name>`
- **Python** (notification-service, all of `tests/`, AI tooling):
  - pytest for test suites
  - Appium (`Appium-Python-Client`) for mobile E2E against `mobile-app`
- **Postgres** — one instance/schema per service that needs persistence
  (ledger-service, accounts-service at minimum). No shared schema.
## AI Usage — Justification Required
 
Every AI-driven feature in this project must have a concrete business
reason, not just be added to demonstrate the skill. Before adding one, be
able to answer: "if this were removed, would the system still work
correctly, just with less signal?" If AI is load-bearing for correctness
(vs. a signal/convenience layer), reconsider the design.
 
Current justified uses:
- **In-product**: rule-based checks (velocity limits, mismatch thresholds,
  blocklists) combined with ML risk scoring inside settlement-engine,
  gating transactions in real time.
- **Tooling** (`tests/ai-tools`): generating test cases from each service's
  OpenAPI spec, and triaging CI failures (flaky vs. real regression vs.
  environment issue). Kept separate from the in-product risk-scoring model.

## Working Style
Mentor mode: when implementing a plan, explain each step/pattern first 
and let the user write the code. Do not write full implementations 
directly into files unless explicitly asked to. Review what the user 
writes; point out mistakes rather than rewriting silently.

- Prefer explaining **why** before **how** — e.g. explain the isolation
  level needed for a double-entry transaction before showing how to write
  the query, rather than handing over working SQL first.
- Plan documents (from the writing-plans skill) may contain illustrative
  code to make the plan concrete — that's fine as a plan artifact. It is
  NOT something to copy into actual service files. When executing a plan,
  treat its code as a reference/answer-key, not a source to paste from.
  Walk through each step and let the user type the real implementation.
- Simplicity over cleverness — avoid unnecessary abstraction layers,
  interfaces, or generics until there's a concrete second use case that
  needs them.
- Commit after each small complete step; `/clear` between unrelated
  services/modules to avoid context rot.

## Git Workflow

One branch per service or major repo-level step, merged to `main` once that
branch's tests pass (or, for scaffolding-only branches with no tests, once
reviewed and functional):

- Naming: `service/<name>` for a whole service (e.g. `service/ledger-service`),
  `step/<short-description>` for repo-level steps (e.g. `step/repo-scaffolding`)
- Work happens on the branch; commit freely as you go (mentor-mode sessions
  produce plenty of small/messy commits — typos, retries, WIP — and that's fine)
- Merge to `main` via **squash merge**, not a merge commit — this collapses
  the branch's messy commit history into one clean commit per service/step
  on `main`
- Delete the branch after merge
- `main` should always be in a working state — nothing half-finished merged in
