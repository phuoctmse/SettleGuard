# accounts-service: NATS JetStream balance consumer

**Goal:** `accounts-service` starts actually owning "balances-of-obligation"
(claimed by root `CLAUDE.md` since the start but never implemented) by
consuming `ledger-service`'s `ledger.entry-recorded` events over NATS
JetStream and maintaining a running `balance` per `Account`. Closes the
other deferred TODO from `docs/superpowers/plans/2026-08-09-accounts-service-mvp.md`.

This is a **consumer**, mirroring the **publisher** already built in
`docs/superpowers/plans/2026-08-10-ledger-service-nats-outbox.md` — read
that doc for the event contract this depends on. `accounts-service` is a
separate Go module from `ledger-service`, so none of ledger-service's Go
packages can be imported directly; types/constants are re-declared locally
where needed.

## Design decisions

1. **Balance formula**: `balance = Σcredit − Σdebit` per `account_id`
   (user's explicit choice — nothing in the codebase defined this before).
   Computed by a pure function in a new `internal/ledgerevent` package.
2. **Idempotency**: JetStream is at-least-once. A `processed_ledger_transactions`
   table (`transaction_id UUID PRIMARY KEY`) dedups at the transaction
   level — `INSERT ... ON CONFLICT (transaction_id) DO NOTHING` inside the
   same DB transaction as the balance update. `jetstream.Consumer.Consume`
   invokes handlers sequentially within one process (verified against
   `nats.go` v1.52.0 source), so `READ COMMITTED` is sufficient; no special
   isolation level needed. The atomicity comes from the PK conflict, not
   from serializability.
3. **Unknown account_id** (an entry references an account this service has
   no row for — the two services are separate bounded contexts, no FK
   possible): not an error. The `UPDATE ... WHERE id = $1` just affects 0
   rows; log it, keep going, still mark the transaction processed.
4. **Ack/Nak/Term policy**: malformed JSON or unrecognized `direction` →
   `Term()` (permanent, will never succeed on redelivery). DB error
   applying the transaction → `Nak()` (transient, let JetStream redeliver
   — `MaxDeliver` stays unlimited, the right call for financial data).
   Success → `Ack()`.
5. **Consumer API**: durable JetStream consumer via `Consumer.Consume(handler)`
   (continuous callback, no manual polling loop), `AckExplicitPolicy`,
   `DeliverPolicy: DeliverAllPolicy` (also the zero-value default —
   explicit for documentation; means a first-ever start replays all
   history already on the stream, which is correct: nothing should be
   missed).
6. **`internal/broker` duplicated, not shared** — same reasoning as
   `ledgerevent`: separate Go modules, no shared package without
   extracting a third module (YAGNI). Copied in shape from ledger-service.
   `EnsureStream` called defensively here too (idempotent, removes a
   startup-ordering dependency between the two services in local dev).
7. **Branch**: continue on `service/accounts-service` (already has the
   unmerged MVP). That branch was missing the NATS docker-compose service
   (only existed on `service/ledger-service`) — resolved via
   `git cherry-pick a783f77` before this plan's Task 1.
8. **Expose it**: `balance` gets added to `Account`/`accountResponse` so
   it's visible through the existing `GET /accounts/{id}` — no new
   endpoint needed.

## Global constraints

Same as `docs/superpowers/plans/2026-08-09-accounts-service-mvp.md`
§Global Constraints (module path, `net/http`+chi, golang-migrate plain
SQL, testcontainers-go real deps, sentinel-error conventions) — unchanged.

---

## Task 1 — Dependencies

```bash
cd services/accounts-service
go get github.com/nats-io/nats.go@v1.52.0
go get github.com/testcontainers/testcontainers-go/modules/nats@v0.44.0
go mod tidy
```
`go mod tidy` also fixes accounts-service's pre-existing `chi` `// indirect`
mismarking (same fix ledger-service picked up incidentally).

- [x] Commit: `chore(accounts-service): add NATS client and testcontainers NATS module dependencies`

## Task 2 — `testutil.NewTestNATS`/`NewTestNATSURL`

`services/accounts-service/internal/testutil/nats.go` — copy of
`services/ledger-service/internal/testutil/nats.go` verbatim (module path
adjusted; file has no other internal imports). Remember: do **not** pass
`natstc.WithArgument("js", "")` — the module already defaults to `-js`;
passing that option breaks nats-server's arg parsing (real bug hit and
fixed while building ledger-service's version).

- [x] Commit: `test(accounts-service): add NATS testcontainers helper`

## Task 3 — `internal/broker`

`services/accounts-service/internal/broker/broker.go` — `Connect(url)
(*nats.Conn, jetstream.JetStream, error)`, `EnsureStream(ctx, js) error`
(stream `LEDGER_EVENTS`, subject `ledger.>`, `FileStorage`,
`CreateOrUpdateStream` — confirmed idempotent). Same shape as
ledger-service's. TDD: stream exists after `EnsureStream`; calling twice
doesn't error.

- [x] Commit: `feat(accounts-service): add NATS JetStream connection and stream setup`

## Task 4 — Migration 000003: `accounts.balance` + expose via API

`internal/db/migrations/000003_add_accounts_balance.up.sql`:
```sql
ALTER TABLE accounts ADD COLUMN balance BIGINT NOT NULL DEFAULT 0;
```
`.down.sql`: `ALTER TABLE accounts DROP COLUMN balance;`

Then: `Account` struct gains `Balance int64`; every `SELECT`/`RETURNING`/
`Scan` in `AccountRepository`'s `Create`/`Get`/`ListByClient`/
`UpdateStatus` gains `balance`/`&acc.Balance`; `accountResponse` gains
`Balance int64 \`json:"balance"\`` mapped in `toAccountResponse`. TDD:
extend existing `Create`/`Get` repository tests to assert `Balance == 0`
on a fresh account (red until migration + repo changes land).

- [x] Commit: `feat(accounts-service): add balance column to accounts and expose it via the API`

## Task 5 — Migration 000004: `processed_ledger_transactions`

```sql
CREATE TABLE processed_ledger_transactions (
    transaction_id UUID PRIMARY KEY,
    processed_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
```
`.down.sql`: `DROP TABLE IF EXISTS processed_ledger_transactions;`

No app code in this task — exercised by Task 7's repository test.

- [x] Commit: `feat(accounts-service): add processed_ledger_transactions table migration`

## Task 6 — `internal/ledgerevent`: payload types + `BalanceDeltas` (TDD, pure)

`services/accounts-service/internal/ledgerevent/payload.go`:
```go
const EventLedgerEntryRecorded = "ledger.entry-recorded"

type OutboxPayload struct {
    TransactionID uuid.UUID            `json:"transaction_id"`
    Entries       []OutboxPayloadEntry `json:"entries"`
}
type OutboxPayloadEntry struct {
    ID        uuid.UUID `json:"id"`
    AccountID uuid.UUID `json:"account_id"`
    Direction string    `json:"direction"`
    Amount    int64     `json:"amount"`
    Reason    string    `json:"reason"`
    CreatedAt time.Time `json:"created_at"`
}

// BalanceDeltas: credit +amount, debit -amount, summed per account_id.
// Unrecognized direction -> error (caller treats as permanent failure).
func BalanceDeltas(entries []OutboxPayloadEntry) (map[uuid.UUID]int64, error)
```
Table-driven test (mirrors `TestValidateBalanced`'s style): single
credit, single debit, multiple entries same account netting to one delta,
multiple accounts in one transaction, unknown direction → error.

- [x] Commit: `feat(accounts-service): add ledger event payload types and balance delta calculation`

## Task 7 — `AccountRepository.ApplyLedgerTransaction` (TDD, real Postgres)

```go
// ApplyLedgerTransaction idempotently applies net balance deltas from one
// ledger transaction. No-op if transactionID was already processed
// (dedup against JetStream's at-least-once redelivery). Deltas for an
// unknown account_id are skipped (logged, not an error).
func (r *AccountRepository) ApplyLedgerTransaction(ctx context.Context, transactionID uuid.UUID, deltas map[uuid.UUID]int64) error
```
Body: one `sql.Tx` — `INSERT INTO processed_ledger_transactions
(transaction_id) VALUES ($1) ON CONFLICT (transaction_id) DO NOTHING`;
check `RowsAffected()` — `0` means already processed, commit and return
early; otherwise loop deltas, `UPDATE accounts SET balance = balance + $1,
updated_at = now() WHERE id = $2`, log if `RowsAffected() == 0`, commit.

Tests: `TestApplyLedgerTransaction_UpdatesBalances` (two accounts, deltas
applied correctly), `TestApplyLedgerTransaction_IdempotentOnSameTransactionID`
(apply twice, balance changes once — **the deterministic idempotency
proof**, no NATS involved), `TestApplyLedgerTransaction_SkipsUnknownAccountGracefully`.

- [x] Commit: `feat(accounts-service): add idempotent ledger-transaction balance apply to account repository`

## Task 8 — `internal/consumer` (TDD, real Postgres + real NATS)

```go
const DurableName = "accounts-service-balance"

type Consumer struct{ accounts *account.AccountRepository }
func New(accounts *account.AccountRepository) *Consumer
func (c *Consumer) Start(ctx context.Context, js jetstream.JetStream) (jetstream.ConsumeContext, error)
func (c *Consumer) handleMessage(msg jetstream.Msg)
```
`Start`: `js.CreateOrUpdateConsumer(ctx, broker.LedgerEventsStream,
jetstream.ConsumerConfig{Durable: DurableName, FilterSubject:
ledgerevent.EventLedgerEntryRecorded, AckPolicy: AckExplicitPolicy,
DeliverPolicy: DeliverAllPolicy})`, then `cons.Consume(c.handleMessage)`.
`handleMessage`: decode → `BalanceDeltas` → `ApplyLedgerTransaction`
(bounded `context.WithTimeout`, not bare `Background()`) → `Ack()`; per
decision 4 above for error paths.

Tests: applies balance on publish (`require.Eventually`); idempotent
across redelivery — publish the **same transaction_id twice as two
separate publishes without `WithMsgID`** (proves *our* dedup table works,
not NATS's broker-side dedupe); skips unknown-account message without
wedging the consumer (publish a second valid message after, confirm it's
still processed); terminates malformed payload promptly (doesn't wait out
`MaxDeliver`).

- [x] Commit: `feat(accounts-service): add JetStream consumer applying ledger balance updates`

## Task 9 — Wire into `main.go`

Same pattern as `services/ledger-service/cmd/server/main.go`: fail-fast on
missing `DATABASE_URL`/`NATS_URL`, `db.Connect`+`db.Migrate`,
`broker.Connect`, `signal.NotifyContext` for shutdown, `broker.EnsureStream`,
construct `consumer.New(accounts)`, `Start(ctx, js)`, `defer consumeCtx.Stop()`,
then existing `api.NewHandlers`/`NewRouter`/`ListenAndServe` unchanged.
`.env.example` gains `NATS_URL=nats://localhost:4222` — **note:
`.env`/`.env.example` are permission-denied for Claude to read/edit; ask
the user to add this line themselves.**

Known gap inherited from ledger-service (not fixed here, flagged only):
`http.ListenAndServe` isn't itself gracefully shut down by the signal
context — only the consumer goroutine is.

- [x] Commit: `feat(accounts-service): wire up NATS balance consumer in main`

## Task 10 — Docs

`README.md`: `NATS_URL` in "Run locally", `balance` field in the API
section, a line noting balance is eventually-consistent (arrives async via
`ledger.entry-recorded`, not synchronously on account creation). Root
`CLAUDE.md`: confirm Stack/Architecture sections don't need further edit
(already NATS-aware from the ledger-service plan, once branches merge).

- [x] Commit: `docs(accounts-service): document NATS balance consumer and balance API field`

---

## Open questions / risks

1. **Consumer durable-name reconciliation** — `CreateOrUpdateConsumer`
   doesn't guarantee every `ConsumerConfig` field is mutable in place
   (e.g. `AckPolicy`). Not a concern now (fresh durable, nothing to
   reconcile), but a *future* change to `DurableName`/`FilterSubject`/
   `AckPolicy` needs an explicit `DeleteConsumer` step, not just a code
   change.
2. **Horizontal scaling** — `Consume()` is sequential within one process
   (verified), so single-instance correctness needs no locking. Multiple
   replicas sharing the durable consumer would deliver concurrently across
   processes; the `ON CONFLICT DO NOTHING` dedup is still atomic under
   `READ COMMITTED` for that case, but this is reasoned, not tested against
   real concurrent replicas.
3. **Branch divergence** — `service/accounts-service` and
   `service/ledger-service` are both long-lived and unmerged; only the one
   isolated docker-compose commit was cherry-picked. They'll need
   reconciling (merge to `main`) eventually.

## Verification

- `cd services/accounts-service && go build ./... && go vet ./... && go test ./...` green after every task.
- Manual: `docker compose -f infra/docker/docker-compose.yml up -d nats accounts-postgres ledger-postgres`, run both services, create a client+account, POST a balanced transaction on ledger-service referencing that account, `GET /accounts/{id}` on accounts-service and confirm `balance` updates within ~1s. Restart accounts-service and confirm balance doesn't double (durable consumer, already-acked messages aren't redelivered).
