# accounts-service: publish `account.updated`

**Goal:** Close the last deferred event-publishing TODO from
`docs/superpowers/plans/2026-08-09-accounts-service-mvp.md` — publish
`account.updated` whenever an Account's state changes (creation, status
update, or balance update from the ledger consumer), mirroring the
transactional outbox pattern already built for `ledger-service`
(`docs/superpowers/plans/2026-08-10-ledger-service-nats-outbox.md`) and
for this service's own consumer side
(`docs/superpowers/plans/2026-08-10-accounts-service-balance-consumer.md`).

## Design decisions

1. **Three trigger points**, all in `AccountRepository`: `Create`,
   `UpdateStatus`, and `ApplyLedgerTransaction` (once per account whose
   balance actually changed). Each writes one `outbox_events` row, in the
   same DB transaction as the mutation, containing the account's full
   post-write state — a snapshot, not a diff, so downstream consumers
   don't need to reconstruct state from partial updates.
2. **New stream `ACCOUNTS_EVENTS`** on subject `account.>`.
   accounts-service *owns* this stream (unlike `LEDGER_EVENTS`, which it
   only defensively ensures as a consumer). `broker.EnsureStream` is
   generalized to accept a `jetstream.StreamConfig` parameter instead of
   being hardcoded to one stream — this is the second concrete stream it
   needs to ensure, so the generalization is justified now, not premature.
3. **Relay**: same shape as `ledger-service/internal/outbox` — a goroutine
   polling unpublished `outbox_events` rows, `js.Publish` with
   `jetstream.WithMsgID(row.id.String())` for dedupe, mark `published_at`
   on ack. accounts-service is a separate Go module, so this package is
   copied, not shared.
4. **No new idempotency concern on the publish side** — that's already
   handled by the outbox pattern itself (each row published once,
   dedup'd by JetStream on the msg-id). The *consumer* side (whoever
   subscribes to `account.updated` later — notification-service,
   mobile-app) will need its own idempotency handling when built, same as
   `settlement-engine` will for `ledger.entry-recorded`.

## Task 1 — Generalize `broker.EnsureStream`

Change signature: `EnsureStream(ctx context.Context, js jetstream.JetStream, cfg jetstream.StreamConfig) error` —
body unchanged (`js.CreateOrUpdateStream(ctx, cfg)`), just takes the config
as a parameter instead of building `LEDGER_EVENTS`'s config internally. Add
`const AccountsEventsStream = "ACCOUNTS_EVENTS"` alongside the existing
`LedgerEventsStream` const. Update `broker_test.go` (both tests now pass a
config explicitly) and the one call site in `main.go`.

- [x] Commit: `refactor(accounts-service): generalize broker.EnsureStream to accept a StreamConfig`

## Task 2 — `outbox_events` migration

`internal/db/migrations/000005_create_outbox_events.{up,down}.sql` — same
shape as ledger-service's:
```sql
CREATE TABLE outbox_events (
    id UUID PRIMARY KEY,
    event_type TEXT NOT NULL,
    subject TEXT NOT NULL,
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ
);
CREATE INDEX idx_outbox_events_unpublished ON outbox_events (created_at) WHERE published_at IS NULL;
```

- [x] Commit: `feat(accounts-service): add outbox_events table migration`

## Task 3 — `internal/account/outbox.go`

```go
const EventAccountUpdated = "account.updated"

type OutboxPayload struct {
    AccountID   uuid.UUID `json:"account_id"`
    ClientID    uuid.UUID `json:"client_id"`
    ExternalRef string    `json:"external_ref"`
    Status      string    `json:"status"`
    Balance     int64     `json:"balance"`
    UpdatedAt   time.Time `json:"updated_at"`
}

func newOutboxPayload(a Account) OutboxPayload
```

- [x] Commit: `feat(accounts-service): add account.updated outbox payload type`

## Task 4 — Write outbox row at each mutation point (TDD)

Extend `Create`, `UpdateStatus`, `ApplyLedgerTransaction` in
`account_repository.go` to each insert one `outbox_events` row (same
`INSERT INTO outbox_events (id, event_type, subject, payload) VALUES
($1, 'account.updated', 'account.updated', $2)`, JSON-marshaled
`newOutboxPayload(acc)`) inside their existing transaction/statement.
`ApplyLedgerTransaction`'s balance-update `UPDATE` needs a `RETURNING`
clause added (currently discards the row) so the full post-update Account
is available to build the payload — only for accounts actually touched
(`RowsAffected() > 0`), not for unknown account_ids.

TDD: extend each method's existing tests (or add new ones) asserting an
`outbox_events` row exists with the right payload after each call.

- [x] Commit: `feat(accounts-service): write account.updated outbox event on every account mutation`

## Task 5 — `internal/outbox.Relay`

Copy of `ledger-service/internal/outbox/relay.go` verbatim (module path
adjusted) — `NewRelay(db, js)`, `Run(ctx) error`, `PublishBatch(ctx) (int, error)`.

- [x] Commit: `feat(accounts-service): add outbox relay publishing to JetStream`

## Task 6 — Wire into `main.go`

`broker.EnsureStream(ctx, js, jetstream.StreamConfig{Name: broker.LedgerEventsStream, Subjects: []string{"ledger.>"}, Storage: jetstream.FileStorage})`
(defensive, unchanged behavior) **and**
`broker.EnsureStream(ctx, js, jetstream.StreamConfig{Name: broker.AccountsEventsStream, Subjects: []string{"account.>"}, Storage: jetstream.FileStorage})`
(owned). Construct `outbox.NewRelay(conn, js)`, launch `go relay.Run(ctx)`
alongside the existing balance consumer goroutine.

- [x] Commit: `feat(accounts-service): wire up account.updated outbox relay in main`

## Task 7 — Docs

README: document `account.updated`, the outbox table, and the relay —
same shape as the "Balance tracking" section already there.

- [x] Commit: `docs(accounts-service): document account.updated event publishing`

## Verification

- `go build ./... && go vet ./... && go test ./...` green after every task.
- Manual: boot the service, create an account, confirm an `outbox_events`
  row appears and gets `published_at` set within ~1s; update its status
  and confirm a second row/publish happens.
