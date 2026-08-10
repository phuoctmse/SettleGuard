# ledger-service: NATS JetStream event publishing (outbox pattern)

**Goal:** Publish `ledger.entry-recorded` from `ledger-service` via NATS
JetStream, using the transactional outbox pattern, closing the gap
`docs/superpowers/plans/2026-07-29-ledger-service-mvp.md` deliberately left
open ("no event publishing until the broker is chosen"). This is the first
slice of the event-broker rollout — `accounts-service`'s `account.updated`
publisher and the `settlement-engine` consumer are separate follow-up
plans, not covered here.

**Broker decision:** NATS JetStream. Reasoning (full writeup in
`docs/WORDING.md` if terms are unfamiliar): durability + per-subject
ordering for ledger correctness, replay for risk-model re-runs/fraud
audits, independent consumer groups (settlement-engine and
notification-service each read independently), much lighter ops than
Kafka for a solo developer, pure-Go client (`nats.go`, no cgo — matters on
Windows).

**Working style for this plan:** per updated root `CLAUDE.md`, Claude
writes the code directly (no mentor-mode hand-typing requirement). This
doc still follows the project's Task/Step convention for traceability, but
code shown inline is illustrative of the shape, not necessarily copied
verbatim — the actual files are the source of truth once written.

## Design decisions

1. **Relay = goroutine inside `ledger-service`**, not a separate binary.
   Single Go module, no second consumer of the outbox table yet — a
   separate deploy unit would be pure overhead (CLAUDE.md: "simplicity
   over cleverness... until there's a concrete second use case").
2. **Outbox row written in the same `sql.Tx`** as the ledger entries insert
   in `Repository.InsertTransaction` — the standard way to avoid the
   dual-write problem (DB commit succeeds, event publish fails, or vice
   versa).
3. **At-least-once delivery.** Relay marks a row published only after a
   successful JetStream ack. `Nats-Msg-Id` header = the outbox row's UUID,
   so JetStream dedupes broker-side within its dedupe window. Downstream
   consumers (future settlement-engine) must still be idempotent — noted
   for that future plan, not designed here.
4. **One event per transaction**, not per entry. `InsertTransaction` is
   the natural transactional boundary, and risk scoring cares about the
   whole balanced transaction, not an isolated debit/credit leg. Payload
   carries the transaction ID and all its entries.
5. **Subject = event name verbatim**: `ledger.entry-recorded`. Stream
   `LEDGER_EVENTS` on wildcard `ledger.>`, so future ledger-service events
   don't need a stream migration.
6. **No retry/dead-letter handling in this slice** (deliberate technical
   debt, YAGNI per CLAUDE.md) — a row that fails to publish just retries
   every poll tick. Flagged here so it isn't forgotten if it turns out
   unacceptable for financial events.
7. **Local dev networking**: Go services run via `go run` on the host, not
   dockerized, so `NATS_URL=nats://localhost:4222` — same pattern as
   Postgres today. Dockerizing services is devops/SRE territory, out of
   scope.

## Global constraints

Same as `docs/superpowers/plans/2026-07-29-ledger-service-mvp.md` §Global
Constraints (module path, `net/http`+chi, golang-migrate plain SQL,
testcontainers-go real deps, `docs/CODING_STANDARDS.md` error/naming
conventions) — this plan doesn't change any of those, it only adds a new
dependency (`nats.go`) and a new package (`internal/broker`,
`internal/outbox`).

---

## Task 1 — NATS in docker-compose

`infra/docker/docker-compose.yml`, new service:
```yaml
  nats:
    image: nats:2.10-alpine
    container_name: settleguard-nats-dev
    command: ["-js", "-sd", "/data"]
    ports:
      - "4222:4222"
      - "8222:8222"
    volumes:
      - nats_dev_data:/data
    healthcheck:
      test: ["CMD", "wget", "--spider", "-q", "http://localhost:8222/healthz"]
      interval: 5s
      timeout: 3s
      retries: 5
```
Add `nats_dev_data:` under `volumes:`. `-js` enables JetStream; `-sd /data`
persists it to the mounted volume.

- [x] Commit: `chore(infra): add NATS JetStream service to docker-compose`

## Task 2 — Dependencies

```bash
cd services/ledger-service
go get github.com/nats-io/nats.go
go get github.com/testcontainers/testcontainers-go/modules/nats
```
Use the `jetstream` subpackage (`github.com/nats-io/nats.go/jetstream`) —
the current, non-deprecated JetStream API.

- [x] Commit: `chore(ledger-service): add NATS client and testcontainers NATS module dependencies`

## Task 3 — Outbox migration

`services/ledger-service/internal/db/migrations/000002_create_outbox_events.up.sql`:
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
`.down.sql`: `DROP TABLE IF EXISTS outbox_events;`

`event_type` and `subject` are equal today but kept separate in case
subject naming ever needs a transform independent of the logical event
type. Partial index keeps the relay's poll query cheap as published rows
accumulate.

- [x] Commit: `feat(ledger-service): add outbox_events table migration`

## Task 4 — `testutil.NewTestNATS`

`services/ledger-service/internal/testutil/nats.go`, mirroring
`testutil/postgres.go`'s shape: start `nats:2.10-alpine` via
testcontainers with the `-js` flag, connect, return `(*nats.Conn,
jetstream.JetStream)`, register `t.Cleanup`. If the official
`testcontainers-go/modules/nats` module's connection-string accessor turns
out awkward, fall back to the same generic-`ContainerRequest` pattern
`postgres.go` already uses (expose `4222/tcp`, wait on the port, build the
URL manually) — precedent-consistent fallback.

- [x] Commit: `test(ledger-service): add NATS testcontainers helper`

## Task 5 — `internal/broker` (connection + stream setup)

`services/ledger-service/internal/broker/broker.go`:
- `Connect(url string) (*nats.Conn, jetstream.JetStream, error)`
- `EnsureStream(ctx context.Context, js jetstream.JetStream) error` —
  idempotent `CreateStream`/`CreateOrUpdateStream` with `Name:
  "LEDGER_EVENTS"`, `Subjects: []string{"ledger.>"}`, `Storage:
  jetstream.FileStorage`.

TDD against `testutil.NewTestNATS`: assert stream exists after
`EnsureStream`, and that calling it twice doesn't error.

- [x] Commit: `feat(ledger-service): add NATS JetStream connection and stream setup`

## Task 6 — Outbox write path

Extend `Repository.InsertTransaction` (`internal/ledger/repository.go`):
within the existing `tx`, after inserting entries and before commit,
marshal `{transaction_id, entries: [...], created_at}` and insert into
`outbox_events` with `event_type = subject = 'ledger.entry-recorded'`.

TDD: after `InsertTransaction`, query `outbox_events` and assert a row
exists with `published_at IS NULL` and payload matching the entries just
written.

- [x] Commit: `feat(ledger-service): write outbox event atomically with ledger entry insert`

## Task 7 — `internal/outbox.Relay`

`services/ledger-service/internal/outbox/relay.go`:
- `type Relay struct { db *sql.DB; js jetstream.JetStream }`, unexported
  `pollInterval = 500ms`, `batchSize = 20` constants (placeholders, no
  load data to size them yet).
- `Run(ctx context.Context) error` — ticker loop, calls `publishBatch`
  each tick, exits on `ctx.Done()`.
- `publishBatch(ctx) error` — `SELECT id, subject, payload FROM
  outbox_events WHERE published_at IS NULL ORDER BY created_at LIMIT
  $1`; for each row, `js.Publish(ctx, subject, payload,
  jetstream.WithMsgID(id.String()))`, then `UPDATE outbox_events SET
  published_at = now() WHERE id = $1` on ack success. Publish errors: log,
  leave unpublished, picked up next tick (no retry/backoff logic — Task 3
  design note 6).

TDD: insert an outbox row, run one `publishBatch`, assert (a)
`published_at` now set, (b) a JetStream consumer on `ledger.entry-recorded`
receives the message with matching payload.

- [x] Commit: `feat(ledger-service): add outbox relay publishing to JetStream`

## Task 8 — Wire into `main.go`

- Read `NATS_URL` (fail fast if unset, matching `DATABASE_URL`'s existing
  pattern).
- `broker.Connect` → `broker.EnsureStream`.
- Construct `outbox.NewRelay(db, js)`, launch `go relay.Run(ctx)`.
- Add `signal.NotifyContext(context.Background(), os.Interrupt,
  syscall.SIGTERM)` for graceful shutdown — `main.go` currently has no
  signal handling; this is now needed so the relay goroutine has a
  cancellable context to stop cleanly. Small, in-scope addition, not scope
  creep.
- `.env.example` gains `NATS_URL=nats://localhost:4222`.
- Full `go build ./...`, `go vet ./...`, `go test ./...` pass.

- [x] Commit: `feat(ledger-service): wire up NATS publishing and outbox relay in main`

## Task 9 — Docs

- `services/ledger-service/README.md`: note `NATS_URL` requirement and the
  compose `nats` service; briefly document `outbox_events` + the relay.
- Root `CLAUDE.md` Stack section: add NATS JetStream (`nats.go`) as the
  Go event-broker client.

- [x] Commit: `docs(ledger-service): document event publishing via NATS JetStream outbox`

---

## Explicitly deferred (follow-up plans)

- `accounts-service` publishing `account.updated` (same outbox pattern,
  separate plan).
- `settlement-engine`'s consumer of `ledger.entry-recorded` (needs the
  idempotency handling noted in design decision 3).
- Outbox retry/backoff/dead-letter handling (design decision 6).
- Dockerizing Go services / production NATS config — devops/SRE, owned by
  the user, not planned here.

## Verification

- `cd services/ledger-service && go build ./... && go vet ./... && go test ./...` green after every task.
- Manual: `docker compose -f infra/docker/docker-compose.yml up -d nats ledger-postgres`, run the service, insert a transaction via the existing HTTP API, confirm (a) an `outbox_events` row appears then gets `published_at` set within ~1s, (b) `nats stream view LEDGER_EVENTS` (or the monitoring UI at `localhost:8222`) shows the message.
