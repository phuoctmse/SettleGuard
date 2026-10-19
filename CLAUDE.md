# CLAUDE.md

SettleGuard: a B2B platform that tracks and settles financial obligations
between parties on top of external payment rails (it never moves real money),
guarding settlement with rule-based + ML risk scoring. v1 excludes moving
funds directly, multi-currency/FX, and on-prem. Charter:
`docs/PROJECT_CHARTER.md`; domain rules: `docs/BUSINESS_RULES.md` (money is
VND dong, MONEY-01); API contracts: `docs/openapi/*.yaml`; designs:
`docs/superpowers/specs/`.

## Commands

Each module builds from its own directory; there is no root build.
Package managers: **npm** (`mobile-app`, `package-lock.json`), **pip + venv**
(`requirements.txt`), **Go modules**. Other managers are blocked by the harness.

**Go** — `services/{ledger-service,accounts-service,settlement-engine,gateway}`
- Build `go build ./...` · Lint `go vet ./...` then `golangci-lint run ./...`
  (root `.golangci.yml`, v1 config; CI pins v1.64.8, built with `go install`).
  gofmt/goimports are enforced by the linter.
- Test `go test -count=1 -p 1 ./...`. Always `-p 1`: each package starts its
  own Postgres/NATS testcontainers, and parallel packages fight over Docker.
- Single test: `go test -count=1 -run '^TestName$' ./internal/<pkg>`

**notification-service** (Python, venv at `.venv`)
- Test `.venv/Scripts/python -m pytest` (needs Docker and the `migrate` CLI).
- Single test: `.venv/Scripts/python -m pytest internal/consumer/consumer_test.py::test_name`
  (tests are `*_test.py` beside the code). No linter is configured.

**mobile-app** (Expo/TypeScript)
- `npm ci` · Lint `npm run lint` · Format check `npm run format:check`
  (fix: `npm run format`) · Test `npm test` · Run `npm start` (port 8090).
- Single test: `npx jest src/components/Button.test.tsx` (add `-t 'name'`).

**tests/** (Python, venv at `tests/.venv`) — black-box suites against a live
local stack (`infra/docker/docker-compose.yml` + the services): `api`,
`security` (+ ZAP), `perf` (Locust), `e2e` (Playwright on the web build).
Setup and env vars: `tests/README.md`, `tests/e2e/README.md`.
- Single test: `.venv/Scripts/python -m pytest api/test_ledger_contract.py::test_name`

## Editing scope

- Change only the files the task names or directly needs. Modules that are
  working stay exactly as they are: refactors, renames and drive-by cleanups
  are separate tasks, proposed first.
- Simplicity over cleverness: add an interface, generic or layer only when a
  concrete second use case exists.
- Explain key design decisions (the why, e.g. the isolation level a
  double-entry write needs) before implementing, then code, run the tests and
  report the results.

## Protected files

- **Migrations** `services/*/internal/db/migrations/*.sql` are append-only.
  One on `origin/main` has already run against real databases: change the
  schema by adding the next numbered `NNNNNN_<name>.up.sql` + `.down.sql`
  pair. Every schema/data change goes through a migration, never ad-hoc
  DDL/DML via `psql` or `docker exec`.
- **Lockfiles** `mobile-app/package-lock.json`, `services/*/go.sum`: change
  only via the tool (`npm install <pkg>`, `go get` / `go mod tidy`), and
  only when adding or removing a dependency.
- **Secrets** `.env` files are off-limits; edit `.env.example` instead.
- **User-owned** `infra/**` (terraform, k8s, docker) and `.github/workflows/**`:
  the user owns devops/cloud/SRE; change these only when asked.
- **Contracts** `docs/openapi/*.yaml`, `docs/BUSINESS_RULES.md`: change them
  together with the code whose behaviour they describe.
- **Generated** `.expo/`, `.e2e-web/`, `tests/test-results/`, `node_modules/`,
  `.venv/`, `__pycache__/`: never edit.

## Harness (`.claude/`)

- `settings.json` denies foreign package managers, force-push,
  `reset --hard`, checking out main/master, deleting branches, `git clean -f`,
  and edits to lockfiles/`.env`; edits under `infra/` and workflows ask first.
- `hooks/guard.py` (PreToolUse) blocks the same git commands in any spelling,
  DDL/DML through `psql`, `migrate force/drop/down`, and edits to migrations
  already on `origin/main`. A block message names the safe route: take it.
- `hooks/stop_verify.py` (Stop) lints and unit-tests every module touched this
  turn (Go: build, vet, golangci-lint, `go test` on touched packages;
  mobile-app: eslint, prettier, `jest --findRelatedTests`; Python: compile +
  related `*_test.py`). On failure it sends you back once with the output:
  fix the cause and re-run the failing command.

## Architecture

Event-driven over **NATS JetStream**: each service owns its data and schema
(one Postgres per service, no cross-service DB access) and publishes through
a transactional outbox.

- **ledger-service** — append-only, double-entry source of truth; publishes
  `ledger.entry-recorded`.
- **accounts-service** — identity, status, `Account.balance` (Σcredit −
  Σdebit) from ledger events, idempotent via a processed-transaction table;
  publishes `account.updated` (`ACCOUNTS_EVENTS`; no consumer yet).
- **settlement-engine** — consumes ledger events (durable
  `settlement-engine-risk-scoring`), scores risk per transaction, batches
  cleared ones on a schedule; publishes `transaction.risk-scored`,
  `settlement.finalized` (`SETTLEMENT_EVENTS`).
- **notification-service** (Python) — consumes risk-hold/settlement events;
  never called synchronously.
- **gateway** — reverse proxy in front of the four: CORS, API-key auth, rate
  limits, request ids. Keys come from `cmd/adminctl`.
- **mobile-app** — read-oriented client with approve/reject on held
  transactions; never a source of truth.

Auth (decided 2026-09-09): API key for client businesses + JWT for human
users, both verified at the gateway; only the API-key half exists. The
backend ports 8080–8083 have no auth of their own, so direct calls bypass it
(AUTH-01..03). Authentication is not authorization: no service checks which
accounts a key may touch, and `GET /notifications` returns every client's
rows. Ownership enforcement is unbuilt: gateway design spec §10.1, §11.

## Stack conventions

- Go: `net/http` + `chi`; `golang-migrate` with plain `.sql` (no sqlc);
  real Postgres/NATS in tests via `testcontainers-go` (no sqlmock, no broker
  mock); layout `cmd/server/main.go` + `internal/{api,db,<domain>}`; module
  path `github.com/phuoctmse/settleguard/<service>`.
- Python: pytest everywhere; Appium Python client for mobile E2E (not written yet).
- AI features need a business reason and must stay a signal layer, never
  load-bearing for correctness. Justified today: ML scoring alongside rules in
  settlement-engine; planned `tests/ai-tools` (test generation from OpenAPI,
  CI-failure triage).

## Git workflow

- One branch per service (`service/<name>`) or repo-level step
  (`step/<desc>`). Every commit lands on the branch matching its scope.
- Start from fresh main: `git fetch && git switch -c <branch> origin/main`
  (local `main` lags the remote).
- Commit after each small complete step; messy WIP commits are fine.
- Merge to `main` by **squash merge** once the branch's tests pass; `main` is
  always working. **Keep every branch after merge.**
- Plan documents' illustrative code is the reference implementation to build from.
