# settlement-engine MVP

**Goal:** `settlement-engine` is the last piece of SettleGuard's real-time
path: it consumes `ledger.entry-recorded` (already published by
`ledger-service`), decides whether each transaction should be held or
allowed to settle, and periodically batches allowed transactions into a
settlement. It's not scaffolded at all yet — no directory exists on disk.

Two decisions were made with the user before this plan was written:

1. **Rule-based risk scoring only, no ML for this MVP.** The project
   charter explicitly defers "choice of ML risk-scoring model, training
   data, and serving approach," and there's no transaction history yet to
   train anything on. This also matches the project's AI Usage policy
   ("if removed, would the system still work correctly, just with less
   signal?") — rules must be the correctness backbone; ML would be a
   later, additive layer.
2. **`transaction.risk-scored` decides hold/pass per transaction; batches
   never carry a "held" state.** A batch only ever contains transactions
   that already passed, so it always publishes `settlement.finalized` —
   there's no `settlement.held-for-review` in this MVP. Simplest reading
   of an otherwise ambiguous charter passage.

**Coding mode for this plan — important, followed per-task below:** the
user wants to hand-write the interesting/core business logic themselves
(risk-scoring rules, the consumer's message-handling logic, batch-settlement
grouping) as a hands-on exercise, since this is the most important/core
service in the project. Everything else — NATS plumbing, DB
connection/migration boilerplate, the generic outbox relay, test helpers,
`main.go` wiring — is routine, already built twice this project
(`ledger-service`, `accounts-service`), and Claude codes it directly per
the default autonomous-coding convention. **Every task below is tagged
`Mode: Claude implements` or `Mode: Mentor` — follow the tag, don't default
to one style for everything.**

For `Mode: Mentor` tasks: Claude writes the failing tests first (TDD,
against real Postgres/NATS via testcontainers, no mocks — this project's
established convention), explains the pattern/why, then the user writes
the implementation, then Claude reviews the diff before moving on.

## Explicitly out of scope for this plan

ML scoring; `settlement.held-for-review`; held-transaction approve/reject
API; blocklist CRUD API (only manual `INSERT` for MVP); any HTTP surface
beyond `GET /health`; real payout / external payment processor
integration (settlement-engine's job ends at publishing
`settlement.finalized`); auth; Dockerfile/k8s manifest.

## Package layout

```
services/settlement-engine/
  cmd/server/main.go
  internal/
    api/          # Health only
    db/           # Connect/Migrate + migrations/ (copy-adapted)
    broker/       # Connect/EnsureStream + stream consts
    ledgerevent/  # mirrored ledger.entry-recorded payload + pure helpers
    risk/         # PURE domain: RiskScore, Scorer, 3 rules — no DB import
    settlement/   # DB-backed: Transaction + Settlement, both repos, Scheduler
    consumer/     # JetStream consumer for ledger.entry-recorded
    outbox/       # generic Relay (copy-adapted)
    testutil/     # NewTestDB, NewTestNATS(URL)
```

`internal/risk` stays DB-free (unit-testable with tiny in-memory fakes for
its two interfaces) by defining `VelocityLimiter`/`BlocklistChecker` as
interfaces there; `settlement.TransactionRepository` implements them
structurally (no import needed in `risk` — avoids a cycle). This mirrors
`account.AccountRepository` owning two related tables under one package
(same bounded context: a transaction's scoring→holding→batching lifecycle).

## Event contracts

**Consumed** (already live, don't change): `ledger.entry-recorded` on
stream `LEDGER_EVENTS` (`ledger.>`), payload
`{transaction_id, entries: [{id, account_id, direction, amount, reason, created_at}]}`.
Mirror locally in `internal/ledgerevent` exactly like
`services/accounts-service/internal/ledgerevent/payload.go` does.

**Owned** (new — first place these get defined): one stream
`SETTLEMENT_EVENTS` with an explicit multi-subject list (JetStream streams
accept a list, not just one wildcard):
```go
jetstream.StreamConfig{
    Name:     "SETTLEMENT_EVENTS",
    Subjects: []string{"settlement.>", "transaction.risk-scored"},
    Storage:  jetstream.FileStorage,
}
```

`transaction.risk-scored` payload (full snapshot, not a diff — matches
`account.updated`'s convention):
```go
type RiskScoredPayload struct {
    TransactionID  uuid.UUID   `json:"transaction_id"`
    AccountIDs     []uuid.UUID `json:"account_ids"`
    Amount         int64       `json:"amount"`
    Score          int         `json:"score"`
    Decision       string      `json:"decision"`        // "pass" | "hold"
    TriggeredRules []string    `json:"triggered_rules"`
    ScoredAt       time.Time   `json:"scored_at"`
}
```

`settlement.finalized` payload:
```go
type SettlementFinalizedPayload struct {
    SettlementID     uuid.UUID   `json:"settlement_id"`
    TransactionIDs   []uuid.UUID `json:"transaction_ids"`
    TransactionCount int         `json:"transaction_count"`
    TotalAmount      int64       `json:"total_amount"`
    FinalizedAt      time.Time   `json:"finalized_at"`
}
```

Both flow through the same generic `outbox.Relay` unmodified.

## Scoring design

`amount` = sum of debit-side entry amounts (== credit-side sum, per
`ledger.ValidateBalanced`'s invariant). `AccountIDs` = distinct accounts
touched across all entries; a rule triggering for *any* touched account
holds the whole transaction. **Decision is OR-based**: `Hold` iff any of
the three rules trigger (matches the charter's literal "more than N →
hold" wording). `Score` is a separate informational/audit field (fixed
weights, summed, capped at 100) — decision doesn't depend on it.

Three rules, MVP definitions (env-configurable, no hardcoding):

- **Velocity limit**: account has >N transactions in a rolling T-minute
  window → hold. (`SETTLEMENT_VELOCITY_LIMIT` default 5,
  `SETTLEMENT_VELOCITY_WINDOW_MINUTES` default 5)
- **Mismatch threshold**: transaction amount exceeds a fixed value →
  hold. (`SETTLEMENT_MISMATCH_THRESHOLD` — **placeholder default only,
  not a real business number**, needs input from whoever owns the
  product side)
- **Blocklist**: any touched account_id has a row in the `blocklist`
  table → hold. (Client-level blocking is schema-ready but inert in this
  MVP — `ledger.entry-recorded` never carries `client_id`, so there's
  nothing to match against without settlement-engine also consuming
  `account.updated`, which is out of scope here.)

Scoring reads DB state (`CountRecentTransactions`) *before* persisting
this transaction's own rows, so a transaction never counts itself.
Idempotency against JetStream redelivery comes from the same
`processed_ledger_transactions` dedup-insert pattern
`AccountRepository.ApplyLedgerTransaction` already uses — `Score` may
recompute harmlessly on redelivery, but the actual writes never double-run.

## Migrations (`internal/db/migrations/`, 7 pairs)

1. `blocklist` — `id, entity_type CHECK IN ('account','client'), entity_id, reason, created_at`, unique `(entity_type, entity_id)`.
2. `transactions` — `id, amount, score, decision CHECK IN ('pass','hold'), status CHECK IN ('pending_settlement','held','settled'), triggered_rules TEXT[], scored_at`; partial index on `status='pending_settlement'`. One row per ledger transaction — `id = transaction_id`, no separate risk-scores table needed.
3. `transaction_accounts` — `(transaction_id, account_id, created_at)` PK, indexed on `(account_id, created_at)` for the velocity query.
4. `processed_ledger_transactions` — exact mirror of accounts-service's (`transaction_id PK, processed_at`).
5. `settlements` — `id, transaction_count, total_amount, created_at`. No `status` column — a row's existence *is* "finalized" in this MVP (no batch-level held state).
6. `settlement_transactions` — `(settlement_id, transaction_id)` PK, both FKs.
7. `outbox_events` — exact mirror of accounts-service's.

## Tasks

Branch: `service/settlement-engine` (already created — never delete after merge, per current convention).

### Task 1 — Scaffold — **Mode: Claude implements**
`go mod init github.com/phuoctmse/settleguard/settlement-engine`
(go 1.26.2); deps = accounts-service's set (chi/v5, golang-migrate/v4,
google/uuid, jackc/pgx/v5, nats-io/nats.go@v1.52.0, testify,
testcontainers-go + its nats module); create the directory layout above;
`.gitignore` copied from accounts-service.

### Task 2 — Migrations — **Mode: Claude implements**
All 7 pairs above.

### Task 3 — `internal/db` + `internal/testutil/postgres.go` — **Mode: Claude implements**
Verbatim copy of `services/accounts-service/internal/db/db.go` +
`internal/testutil/postgres.go`, module path adjusted. Keep the
`postgres:18-alpine` + `wait.ForLog(...).WithOccurrence(2)` detail intact
(known real fix, not incidental).

### Task 4 — `internal/testutil/nats.go` — **Mode: Claude implements**
Verbatim copy of `services/accounts-service/internal/testutil/nats.go`.
**Do not** pass `WithArgument("js", "")` to the nats testcontainers module
— breaks nats-server's arg parsing (hit twice already this project).

### Task 5 — `internal/broker` — **Mode: Claude implements**
Verbatim copy of `Connect`/`EnsureStream` (generic, takes a
`jetstream.StreamConfig`) from `services/accounts-service/internal/broker/broker.go`,
plus this service's constants: `LedgerEventsStream = "LEDGER_EVENTS"`
(defensive-ensure only) and `SettlementEventsStream = "SETTLEMENT_EVENTS"` (owned).

### Task 6 — `internal/ledgerevent` — **Mode: Claude implements**
Mirrored `ledger.entry-recorded` payload (same shape as
`services/accounts-service/internal/ledgerevent/payload.go`), plus two
pure helpers: `TotalAmount(entries) int64` (sum of debit amounts),
`AccountIDs(entries) []uuid.UUID` (distinct). TDD, table-driven.

### Task 7 — `internal/risk` — **Mode: split (see below)**
**Claude implements** the skeleton (no rule logic): `Decision`,
`Config{VelocityLimit, VelocityWindow, MismatchThreshold}`,
`TransactionInput{ID, AccountIDs, Amount, OccurredAt}`,
`RuleOutcome{Rule, Triggered, Detail}`, `RiskScore{TransactionID, Score,
Decision, Outcomes}`, the `VelocityLimiter`/`BlocklistChecker` interfaces,
`Scorer{cfg, velocity, blocklist}` + `NewScorer(...)`.

**Claude writes failing tests first** (in-memory fakes for the two
interfaces — this doesn't violate the "no mocks" convention since nothing
here mocks Postgres/NATS, it's pure-function unit testing):
`TestScorer_Score_PassWhenNoRuleTriggers`,
`..._HoldOnVelocityLimit`/`..._HoldOnMismatchThreshold`/`..._HoldOnBlocklist`,
`..._MultipleRulesTrigger_ScoreAccumulatesCapped100`,
`..._PropagatesVelocityLimiterError`/`..._BlocklistCheckerError`.

**User implements** (the core mentor-mode payload of this whole plan):
`Score(ctx, tx) (RiskScore, error)`, `checkVelocityLimit`,
`checkMismatchThreshold`, `checkBlocklist`. Proposed weights (confirm/adjust
with the user before writing the tests, since the tests encode the exact
numbers): velocity=40, mismatch=30, blocklist=100 (hard block, ties
Decision=Hold alone).

Two commits: skeleton+tests (Claude), then implementation (user, after
Claude review).

### Task 8 — `settlement.Transaction` + `TransactionRepository` core — **Mode: Claude implements**
Routine — mirrors `AccountRepository.ApplyLedgerTransaction`'s dedup shape
and `account/outbox.go`'s insert-in-tx pattern almost exactly.
`RecordScore(ctx, tx, score)`: one `sql.Tx` — dedup insert into
`processed_ledger_transactions` (early-return on conflict), else insert
`transactions` row + one `transaction_accounts` row per account touched +
`transaction.risk-scored` outbox row, commit. `status` derived from
`score.Decision` (Hold→held, Pass→pending_settlement). TDD real Postgres:
persists passing/held transactions, idempotent on same transaction_id
(deterministic idempotency proof, no NATS), writes outbox event.

### Task 9 — `CountRecentTransactions` + `IsBlocked` — **Mode: split**
**Claude writes failing tests first** (real Postgres): counts within/
excludes-outside velocity window, excludes other accounts;
blocklist true/false/true-on-any-of-multiple.

**User implements** the two query bodies on `TransactionRepository` —
this is the DB-facing half of "the rule-checking logic."

### Task 10 — `Settlement` + `SettlementRepository.RunBatch` — **Mode: split**
**Claude writes failing tests first** (real Postgres): batches all
pending, excludes held, excludes already-settled, no-op (nil,nil) when
nothing pending, marks included transactions settled, writes
`settlement.finalized` outbox event.

**User implements** `Settlement{ID, TransactionIDs, TransactionCount,
TotalAmount, CreatedAt}` and `RunBatch(ctx) (*Settlement, error)`: select
pending transactions (row-locked defensively, not load-bearing until
horizontal scaling exists), insert `settlements` + `settlement_transactions`
rows, flip transactions to settled, write outbox row, commit — all one
`sql.Tx`. No time window: `pending_settlement` status *is* "not yet
batched," so nothing skips or double-includes even under concurrent
consumer inserts.

### Task 11 — `settlement.Scheduler` — **Mode: Claude implements**
Boilerplate ticker loop, mirrors `outbox.Relay.Run`'s exact shape
(`time.NewTicker`/`select`/`ctx.Done()`) but calls `RunBatch` instead of
`PublishBatch`. TDD: calls `RunBatch` on each tick (short interval +
`require.Eventually`, real Postgres).

### Task 12 — `internal/outbox.Relay` — **Mode: Claude implements**
Verbatim copy of `services/accounts-service/internal/outbox/relay.go`.

### Task 13 — `internal/consumer` — **Mode: split**
**Claude implements** `New`/`Start`/`DurableName = "settlement-engine-risk-scoring"`
— identical shape to `services/accounts-service/internal/consumer/consumer.go`
(`js.CreateOrUpdateConsumer` with `AckExplicitPolicy`/`DeliverAllPolicy` on
`LedgerEventsStream` filtered to `ledger.entry-recorded`, then
`cons.Consume(handleMessage)`).

**Claude writes failing tests first** (real Postgres + real NATS): scores
and persists on publish; idempotent across redelivery (same
`transaction_id` published twice **without** `WithMsgID`, to prove *our*
dedup table works, not NATS's broker-side dedupe); terminates malformed
payload promptly; a held transaction still gets `Ack()`'d (holding isn't
an error).

**User implements** `handleMessage(msg jetstream.Msg)`: decode → build
`risk.TransactionInput` → score → `RecordScore` → Ack/Nak/Term per the
established policy (malformed JSON → `Term()`; scoring/DB error → `Nak()`;
success → `Ack()`) — Claude explains this pattern (from
`accounts-service`'s consumer) before the user writes it.

### Task 14 — `cmd/server/main.go` — **Mode: Claude implements**
Mirrors `services/accounts-service/cmd/server/main.go` exactly in shape:
fail-fast on missing `DATABASE_URL`/`NATS_URL` → `db.Connect`+`db.Migrate`
→ `broker.Connect` → `signal.NotifyContext` → `EnsureStream` for both
`LedgerEventsStream` (defensive) and `SettlementEventsStream` (owned) →
construct repos → `risk.NewScorer(cfg, transactions, transactions)` (one
repo satisfies both interfaces) → `consumer.New(...).Start(ctx, js)` +
`defer consumeCtx.Stop()` → `settlement.NewScheduler(...)` in a goroutine
→ `outbox.NewRelay(...)` in a goroutine → minimal `api` router (health
only) → `http.ListenAndServe`. `LISTEN_ADDR` default **`:8082`** (sequential
after ledger `:8080`, accounts `:8081` — confirmed from their `main.go`
files; could not verify root `.env.example`'s port reservations since that
file is permission-denied for Claude to read). Env-driven config:
`SETTLEMENT_VELOCITY_LIMIT` (5), `SETTLEMENT_VELOCITY_WINDOW_MINUTES` (5),
`SETTLEMENT_MISMATCH_THRESHOLD` (placeholder), `SETTLEMENT_BATCH_INTERVAL_SECONDS` (60).

### Task 15 — Lint pass — **Mode: Claude implements**
`golangci-lint run ./...` clean across all files, including the
user-written bodies from Tasks 7/9/10/13 — this is the concrete part of
"Claude reviews the result" for those tasks.

### Task 16 — Docs + local-dev wiring — **Mode: Claude implements**
`services/settlement-engine/README.md` (same shape as accounts-service's:
run locally, event contracts, risk rules + env knobs, scheduler behavior,
build/lint/test, note held transactions have no resolution path yet).
`infra/docker/docker-compose.yml`: add `settlement-postgres` block
(`postgres:18-alpine`, port `5435:5432`, matching the existing two Postgres
blocks). `.env`/`.env.example`: **permission-denied for Claude** — ask the
user to add `DATABASE_URL`, `NATS_URL`, and the four `SETTLEMENT_*` vars
themselves. Update root `CLAUDE.md`'s Project Status line once merged.

## Open questions to resolve during implementation (not blocking this plan)

1. Client-level blocklist is schema-ready but inert (no `client_id` data
   available without also consuming `account.updated` — out of scope).
2. No blocklist CRUD API in this MVP — manual `INSERT` only; a minimal
   admin endpoint would be a cheap fast-follow if wanted.
3. `SETTLEMENT_MISMATCH_THRESHOLD`'s default is a placeholder, not a real
   number — needs real input before Task 7's tests lock in a meaningful value.
4. `transaction_accounts` has no retention policy (unbounded growth) —
   fine for MVP, flag for later.
5. Consumer durable-name (`settlement-engine-risk-scoring`) reconciliation:
   changing `FilterSubject`/`AckPolicy` later needs an explicit
   `DeleteConsumer` step, not just a code change.
6. `RunBatch`'s `FOR UPDATE` locking is defensive, not load-bearing until
   settlement-engine itself runs with >1 instance (not currently planned).

## Verification

- `cd services/settlement-engine && go build ./... && go vet ./... && go test ./...` green after every task (Docker required).
- `golangci-lint run ./...` clean.
- Manual: `docker compose -f infra/docker/docker-compose.yml up -d nats settlement-postgres ledger-postgres`, run `ledger-service` + `settlement-engine`, `POST` a balanced transaction on ledger-service. Confirm a `transactions` row appears with expected `score`/`decision`/`status` within ~1s and `transaction.risk-scored` publishes. Post enough transactions on one account within the velocity window to force a hold; confirm `status='held'` and exclusion from the next batch. Wait one scheduler tick; confirm `settlements`+`settlement_transactions` rows and a `settlement.finalized` publish for all `pending_settlement` transactions, held one untouched.

## Critical files for implementation

- `services/settlement-engine/internal/risk/scorer.go` (new — pure scoring domain, core mentor-mode logic)
- `services/settlement-engine/internal/settlement/transaction_repository.go` (new — dedup, persistence, velocity/blocklist queries)
- `services/settlement-engine/internal/settlement/settlement_repository.go` (new — batch grouping/finalization)
- `services/settlement-engine/internal/consumer/consumer.go` (new — ties consume→score→persist→outbox together)
- `services/settlement-engine/cmd/server/main.go` (new — wiring)
- `services/accounts-service/internal/account/account_repository.go` (reference — exact dedup/outbox-in-tx pattern to mirror)
- `services/accounts-service/internal/outbox/relay.go` (reference — copied near-verbatim)
- `services/accounts-service/internal/consumer/consumer.go` (reference — Ack/Nak/Term pattern to explain to the user before Task 13)
