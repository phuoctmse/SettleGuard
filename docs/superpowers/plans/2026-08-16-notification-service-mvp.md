# notification-service MVP Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build `services/notification-service` (Python) so it consumes `settlement-engine`'s `transaction.risk-scored` (hold decisions only) and `settlement.finalized` events from NATS JetStream and persists them as `notifications` rows — the audit-trail/insertion-point MVP described in `docs/superpowers/specs/2026-08-16-notification-service-design.md`.

**Architecture:** A single Postgres table (`notifications`) doubles as both the business record and the idempotency guard via `UNIQUE (type, subject_id)`. Two durable JetStream push consumers (one per subject, mirroring the one-consumer-per-subject pattern already used by `ledger-service`/`accounts-service`/`settlement-engine`) each parse their event, apply the one business rule that matters (skip `transaction.risk-scored` unless `decision == "hold"`), and insert-or-ignore into `notifications`. A stdlib `http.server` `/health` endpoint runs alongside for k8s liveness/readiness probes. No outbox — this service publishes nothing.

**Tech Stack:** Python 3.11+, `psycopg` v3 (raw SQL, no ORM), `nats-py` (legacy `nats.js` JetStream API — `import nats; nc.jetstream()`), `migrate` CLI (golang-migrate, invoked via subprocess — same binary already required for the 3 Go services), `pytest` + `pytest-asyncio` + `testcontainers-python` (real Postgres + real NATS in tests, no mocks), stdlib `http.server`.

## Global Constraints

- Module/package layout: `services/notification-service/main.py` +
  `internal/{db,notifications,broker,consumer,api}/` — mirrors the Go
  services' `cmd/server/main.go` + `internal/{api,db,...}/` layout.
- No ORM, no code-gen migration tooling — plain `.sql` files under
  `internal/db/migrations/`, run via the `migrate` CLI.
- No mocks in tests — `testcontainers-python` spins up real Postgres and
  real NATS (JetStream enabled) for every test that touches either.
- Tests co-located with source as `<module>_test.py` (pytest discovers
  `test_*.py` and `*_test.py`; this repo's Python syntax reference
  confirms both are valid — co-location matches the Go `_test.go`
  convention already used in this repo).
- Env vars: `DATABASE_URL`, `NATS_URL` (required, fatal if missing),
  `LISTEN_ADDR` (optional, default `:8083` — next free port after
  ledger-service `:8080`, accounts-service `:8081`, settlement-engine
  `:8082`).
- Stream/subjects this service consumes (already owned and published by
  `settlement-engine`, do not redefine): `SETTLEMENT_EVENTS` stream,
  subjects `transaction.risk-scored` and `settlement.finalized`.
- Postgres DB name/user/password: `notification` (matches the
  `accounts`/`settlement` single-word convention in the other services'
  `.env` files).
- Run all commands below from inside `services/notification-service/`
  (matches how `go build`/`go test` are always run from inside their
  service directory in this repo).

---

### Task 1: Project scaffold + `internal/db` (connect + migrate)

**Files:**
- Create: `services/notification-service/requirements.txt`
- Create: `services/notification-service/pyproject.toml`
- Create: `services/notification-service/.env.example`
- Create: `services/notification-service/internal/__init__.py`
- Create: `services/notification-service/internal/db/__init__.py`
- Create: `services/notification-service/internal/db/connection.py`
- Create: `services/notification-service/internal/db/migrations/000001_create_notifications.up.sql`
- Create: `services/notification-service/internal/db/migrations/000001_create_notifications.down.sql`
- Test: `services/notification-service/internal/db/connection_test.py`

**Interfaces:**
- Produces: `internal.db.connection.connect(dsn: str) -> psycopg.Connection`
  and `internal.db.connection.migrate(dsn: str) -> None`. Every later task
  that needs a DB connection calls these two functions.

- [ ] **Step 1: Create the scaffold files**

`services/notification-service/requirements.txt`:

```
psycopg[binary]>=3.1,<4
nats-py>=2.6,<3
pytest>=8.0,<9
pytest-asyncio>=0.23,<1
testcontainers[postgres]>=4.0,<5
```

`services/notification-service/pyproject.toml`:

```toml
[tool.pytest.ini_options]
asyncio_mode = "auto"
```

`services/notification-service/.env.example`:

```
POSTGRES_USER=notification
POSTGRES_PASSWORD=notification
POSTGRES_DB=notification
NATS_URL=nats://localhost:4222
```

`services/notification-service/internal/__init__.py` — empty file.
`services/notification-service/internal/db/__init__.py` — empty file.

- [ ] **Step 2: Create a virtualenv and install dependencies**

Run:

```bash
cd services/notification-service
python -m venv .venv
source .venv/Scripts/activate
pip install -r requirements.txt
```

Expected: installs cleanly, no errors. Every later `pytest`/`python` command
in this plan assumes this virtualenv is activated.

- [ ] **Step 3: Write the migration SQL**

`services/notification-service/internal/db/migrations/000001_create_notifications.up.sql`:

```sql
CREATE TABLE notifications (
    id UUID PRIMARY KEY,
    type TEXT NOT NULL CHECK (type IN ('risk_hold', 'settlement_finalized')),
    subject_id UUID NOT NULL,
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (type, subject_id)
);
```

`services/notification-service/internal/db/migrations/000001_create_notifications.down.sql`:

```sql
DROP TABLE notifications;
```

- [ ] **Step 4: Write the failing test**

`services/notification-service/internal/db/connection_test.py`:

```python
from testcontainers.postgres import PostgresContainer

from internal.db.connection import connect, migrate


def test_migrate_creates_notifications_table():
    with PostgresContainer("postgres:18-alpine") as postgres:
        dsn = postgres.get_connection_url(driver=None)

        migrate(dsn)

        conn = connect(dsn)
        try:
            with conn.cursor() as cur:
                cur.execute(
                    """
                    SELECT column_name FROM information_schema.columns
                    WHERE table_name = 'notifications'
                    ORDER BY column_name
                    """
                )
                columns = {row[0] for row in cur.fetchall()}
        finally:
            conn.close()

        assert columns == {"id", "type", "subject_id", "payload", "created_at"}


def test_migrate_is_idempotent():
    with PostgresContainer("postgres:18-alpine") as postgres:
        dsn = postgres.get_connection_url(driver=None)

        migrate(dsn)
        migrate(dsn)  # must not raise
```

- [ ] **Step 5: Run the test to verify it fails**

Run: `pytest internal/db/connection_test.py -v`
Expected: FAIL with `ModuleNotFoundError: No module named 'internal.db.connection'`
(or similar — `connection.py` doesn't exist yet).

- [ ] **Step 6: Implement `internal/db/connection.py`**

```python
import subprocess
from pathlib import Path

import psycopg

MIGRATIONS_DIR = Path(__file__).parent / "migrations"


def connect(dsn: str) -> psycopg.Connection:
    return psycopg.connect(dsn)


def migrate(dsn: str) -> None:
    subprocess.run(
        ["migrate", "-path", str(MIGRATIONS_DIR), "-database", dsn, "up"],
        check=True,
    )
```

This requires the `migrate` CLI binary on PATH — the same binary already
required for local dev of `ledger-service`/`accounts-service`/
`settlement-engine` (they use golang-migrate as a Go library instead, so
this is the one place in the repo that needs the CLI form specifically).

- [ ] **Step 7: Run the test to verify it passes**

Run: `pytest internal/db/connection_test.py -v`
Expected: 2 passed. Requires Docker running and `migrate` on PATH.

- [ ] **Step 8: Commit**

```bash
git add services/notification-service/requirements.txt \
        services/notification-service/pyproject.toml \
        services/notification-service/.env.example \
        services/notification-service/internal/__init__.py \
        services/notification-service/internal/db/
git commit -m "feat(notification-service): scaffold + db connect/migrate"
```

---

### Task 2: `internal/notifications` repository

**Files:**
- Create: `services/notification-service/internal/notifications/__init__.py`
- Create: `services/notification-service/internal/notifications/repository.py`
- Test: `services/notification-service/internal/notifications/repository_test.py`

**Interfaces:**
- Consumes: `internal.db.connection.connect`, `internal.db.connection.migrate`
  (Task 1).
- Produces: `internal.notifications.repository.NotificationRepository(conn)`
  with method `.record(type_: str, subject_id: uuid.UUID, payload: dict) -> bool`
  (returns `True` if a new row was inserted, `False` if it was a no-op
  duplicate). The consumer (Task 4) calls this directly.

- [ ] **Step 1: Write the failing test**

`services/notification-service/internal/notifications/repository_test.py`:

```python
import uuid

from testcontainers.postgres import PostgresContainer

from internal.db.connection import connect, migrate
from internal.notifications.repository import NotificationRepository


def _repo(dsn):
    conn = connect(dsn)
    return NotificationRepository(conn), conn


def test_record_inserts_new_notification():
    with PostgresContainer("postgres:18-alpine") as postgres:
        dsn = postgres.get_connection_url(driver=None)
        migrate(dsn)
        repo, conn = _repo(dsn)
        tx_id = uuid.uuid4()

        inserted = repo.record("risk_hold", tx_id, {"transaction_id": str(tx_id), "decision": "hold"})

        assert inserted is True
        with conn.cursor() as cur:
            cur.execute("SELECT type, subject_id FROM notifications WHERE subject_id = %s", (tx_id,))
            row = cur.fetchone()
        assert row == ("risk_hold", tx_id)
        conn.close()


def test_record_is_idempotent_on_type_and_subject_id():
    with PostgresContainer("postgres:18-alpine") as postgres:
        dsn = postgres.get_connection_url(driver=None)
        migrate(dsn)
        repo, conn = _repo(dsn)
        tx_id = uuid.uuid4()

        first = repo.record("risk_hold", tx_id, {"decision": "hold"})
        second = repo.record("risk_hold", tx_id, {"decision": "hold"})

        assert first is True
        assert second is False
        with conn.cursor() as cur:
            cur.execute("SELECT count(*) FROM notifications WHERE subject_id = %s", (tx_id,))
            count = cur.fetchone()[0]
        assert count == 1
        conn.close()
```

- [ ] **Step 2: Run test to verify it fails**

Run: `pytest internal/notifications/repository_test.py -v`
Expected: FAIL — `internal.notifications.repository` doesn't exist yet.

- [ ] **Step 3: Implement `internal/notifications/repository.py`**

```python
import uuid

import psycopg
from psycopg.types.json import Jsonb


class NotificationRepository:
    def __init__(self, conn: psycopg.Connection):
        self._conn = conn

    def record(self, type_: str, subject_id: uuid.UUID, payload: dict) -> bool:
        with self._conn.cursor() as cur:
            cur.execute(
                """
                INSERT INTO notifications (id, type, subject_id, payload)
                VALUES (%s, %s, %s, %s)
                ON CONFLICT (type, subject_id) DO NOTHING
                """,
                (uuid.uuid4(), type_, subject_id, Jsonb(payload)),
            )
            inserted = cur.rowcount > 0
        self._conn.commit()
        return inserted
```

`services/notification-service/internal/notifications/__init__.py` — empty file.

- [ ] **Step 4: Run test to verify it passes**

Run: `pytest internal/notifications/repository_test.py -v`
Expected: 2 passed.

- [ ] **Step 5: Commit**

```bash
git add services/notification-service/internal/notifications/
git commit -m "feat(notification-service): notifications repository"
```

---

### Task 3: `internal/broker` — NATS connect + ensure_stream

**Files:**
- Create: `services/notification-service/internal/broker/__init__.py`
- Create: `services/notification-service/internal/broker/connection.py`
- Test: `services/notification-service/internal/broker/connection_test.py`

**Interfaces:**
- Produces: `internal.broker.connection.connect(url: str) -> tuple[nats.aio.client.Client, nats.js.JetStreamContext]`
  and `internal.broker.connection.ensure_stream(js, name: str, subjects: list[str]) -> None`
  (idempotent — safe to call every startup, same reasoning as
  `settlement-engine`'s `broker.EnsureStream`). Task 6 (`main.py`) calls
  both; Task 4's tests call `ensure_stream` to set up `SETTLEMENT_EVENTS`
  before publishing test events.
- Constants: `SETTLEMENT_EVENTS_STREAM = "SETTLEMENT_EVENTS"`,
  `SETTLEMENT_EVENTS_SUBJECTS = ["settlement.>", "transaction.risk-scored"]`
  defined in this file (mirrors the subjects `settlement-engine` itself
  registers the stream with in `cmd/server/main.go`).

- [ ] **Step 1: Write the failing test**

`services/notification-service/internal/broker/connection_test.py`:

```python
import pytest
from testcontainers.core.container import DockerContainer
from testcontainers.core.waiting_utils import wait_for_logs

from internal.broker.connection import connect, ensure_stream


@pytest.fixture
def nats_url():
    container = (
        DockerContainer("nats:2.10-alpine")
        .with_command(["-js"])
        .with_exposed_ports(4222)
        .waiting_for(wait_for_logs("Server is ready"))
    )
    container.start()
    try:
        host = container.get_container_host_ip()
        port = container.get_exposed_port(4222)
        yield f"nats://{host}:{port}"
    finally:
        container.stop()


async def test_ensure_stream_creates_then_is_idempotent(nats_url):
    nc, js = await connect(nats_url)
    try:
        await ensure_stream(js, "SETTLEMENT_EVENTS", ["settlement.>", "transaction.risk-scored"])
        await ensure_stream(js, "SETTLEMENT_EVENTS", ["settlement.>", "transaction.risk-scored"])  # must not raise

        info = await js.stream_info("SETTLEMENT_EVENTS")
        assert set(info.config.subjects) == {"settlement.>", "transaction.risk-scored"}
    finally:
        await nc.close()
```

- [ ] **Step 2: Run test to verify it fails**

Run: `pytest internal/broker/connection_test.py -v`
Expected: FAIL — `internal.broker.connection` doesn't exist yet.

- [ ] **Step 3: Implement `internal/broker/connection.py`**

```python
import nats
from nats.aio.client import Client
from nats.js import JetStreamContext
from nats.js.errors import NotFoundError

SETTLEMENT_EVENTS_STREAM = "SETTLEMENT_EVENTS"
SETTLEMENT_EVENTS_SUBJECTS = ["settlement.>", "transaction.risk-scored"]


async def connect(url: str) -> tuple[Client, JetStreamContext]:
    nc = await nats.connect(url)
    js = nc.jetstream()
    return nc, js


async def ensure_stream(js: JetStreamContext, name: str, subjects: list[str]) -> None:
    try:
        await js.stream_info(name)
    except NotFoundError:
        await js.add_stream(name=name, subjects=subjects)
```

`services/notification-service/internal/broker/__init__.py` — empty file.

- [ ] **Step 4: Run test to verify it passes**

Run: `pytest internal/broker/connection_test.py -v`
Expected: 1 passed. Requires Docker running.

- [ ] **Step 5: Commit**

```bash
git add services/notification-service/internal/broker/
git commit -m "feat(notification-service): nats broker connect + ensure_stream"
```

---

### Task 4: `internal/consumer` — risk-hold + settlement-finalized handling

**Files:**
- Create: `services/notification-service/internal/consumer/__init__.py`
- Create: `services/notification-service/internal/consumer/consumer.py`
- Test: `services/notification-service/internal/consumer/consumer_test.py`
- Modify: `docs/BUSINESS_RULES.md` (add `NOTIFICATION-01`)

**Interfaces:**
- Consumes: `internal.notifications.repository.NotificationRepository` (Task
  2), `internal.broker.connection.connect`/`ensure_stream`/
  `SETTLEMENT_EVENTS_STREAM`/`SETTLEMENT_EVENTS_SUBJECTS` (Task 3).
- Produces: `internal.consumer.consumer.decide_risk_scored(payload: dict) -> dict | None`,
  `internal.consumer.consumer.decide_settlement_finalized(payload: dict) -> dict`
  (pure functions, no I/O — each returns `{"type": str, "subject_id":
  uuid.UUID, "payload": dict}` or `None` to signal "skip, do not
  notify"), and `internal.consumer.consumer.Consumer(repo)` with
  `async def start(self, js) -> None`. Task 6 (`main.py`) constructs
  `Consumer(repo)` and calls `.start(js)` once at boot.

- [ ] **Step 1: Write the failing pure-logic tests (no containers needed)**

`services/notification-service/internal/consumer/consumer_test.py` (part 1 —
append parts 2 and 3 in later steps of this same task):

```python
import uuid

from internal.consumer.consumer import decide_risk_scored, decide_settlement_finalized


def test_decide_risk_scored_returns_none_when_pass():
    payload = {"transaction_id": str(uuid.uuid4()), "decision": "pass"}

    assert decide_risk_scored(payload) is None


def test_decide_risk_scored_returns_record_when_hold():
    tx_id = str(uuid.uuid4())
    payload = {"transaction_id": tx_id, "decision": "hold", "score": 40}

    record = decide_risk_scored(payload)

    assert record == {"type": "risk_hold", "subject_id": uuid.UUID(tx_id), "payload": payload}


def test_decide_settlement_finalized_always_returns_record():
    settlement_id = str(uuid.uuid4())
    payload = {"settlement_id": settlement_id, "transaction_count": 3}

    record = decide_settlement_finalized(payload)

    assert record == {"type": "settlement_finalized", "subject_id": uuid.UUID(settlement_id), "payload": payload}
```

- [ ] **Step 2: Run to verify it fails**

Run: `pytest internal/consumer/consumer_test.py -v`
Expected: FAIL — `internal.consumer.consumer` doesn't exist yet.

- [ ] **Step 3: Implement the pure decision functions and the `Consumer` class**

`services/notification-service/internal/consumer/consumer.py`:

```python
import json
import logging
import uuid

from nats.aio.msg import Msg
from nats.js import JetStreamContext
from nats.js.api import AckPolicy, ConsumerConfig, DeliverPolicy

from internal.broker.connection import SETTLEMENT_EVENTS_STREAM
from internal.notifications.repository import NotificationRepository

logger = logging.getLogger(__name__)

RISK_HOLD_DURABLE = "notification-service-risk-hold"
SETTLEMENT_FINALIZED_DURABLE = "notification-service-settlement-finalized"


def decide_risk_scored(payload: dict) -> dict | None:
    if payload.get("decision") != "hold":
        return None
    return {
        "type": "risk_hold",
        "subject_id": uuid.UUID(payload["transaction_id"]),
        "payload": payload,
    }


def decide_settlement_finalized(payload: dict) -> dict:
    return {
        "type": "settlement_finalized",
        "subject_id": uuid.UUID(payload["settlement_id"]),
        "payload": payload,
    }


class Consumer:
    def __init__(self, repo: NotificationRepository):
        self._repo = repo

    async def start(self, js: JetStreamContext) -> None:
        await js.subscribe(
            "transaction.risk-scored",
            stream=SETTLEMENT_EVENTS_STREAM,
            durable=RISK_HOLD_DURABLE,
            cb=self._handle_risk_scored,
            manual_ack=True,
            config=ConsumerConfig(
                durable_name=RISK_HOLD_DURABLE,
                ack_policy=AckPolicy.EXPLICIT,
                deliver_policy=DeliverPolicy.ALL,
            ),
        )
        await js.subscribe(
            "settlement.finalized",
            stream=SETTLEMENT_EVENTS_STREAM,
            durable=SETTLEMENT_FINALIZED_DURABLE,
            cb=self._handle_settlement_finalized,
            manual_ack=True,
            config=ConsumerConfig(
                durable_name=SETTLEMENT_FINALIZED_DURABLE,
                ack_policy=AckPolicy.EXPLICIT,
                deliver_policy=DeliverPolicy.ALL,
            ),
        )

    async def _handle_risk_scored(self, msg: Msg) -> None:
        await self._handle(msg, decide_risk_scored)

    async def _handle_settlement_finalized(self, msg: Msg) -> None:
        await self._handle(msg, decide_settlement_finalized)

    async def _handle(self, msg: Msg, decide) -> None:
        try:
            payload = json.loads(msg.data)
        except json.JSONDecodeError:
            logger.error("consumer: malformed payload on %s, terminating message", msg.subject)
            await msg.term()
            return

        record = decide(payload)
        if record is not None:
            try:
                self._repo.record(record["type"], record["subject_id"], record["payload"])
            except Exception:
                logger.exception("consumer: failed to record notification for %s", msg.subject)
                await msg.nak()
                return

        await msg.ack()
```

`services/notification-service/internal/consumer/__init__.py` — empty file.

- [ ] **Step 4: Run to verify the pure-logic tests pass**

Run: `pytest internal/consumer/consumer_test.py -v`
Expected: 3 passed.

- [ ] **Step 5: Add the end-to-end integration test (real NATS + real Postgres)**

Append to `services/notification-service/internal/consumer/consumer_test.py`:

```python
import asyncio
import json

import pytest
from testcontainers.core.container import DockerContainer
from testcontainers.core.waiting_utils import wait_for_logs
from testcontainers.postgres import PostgresContainer

from internal.broker.connection import connect as nats_connect, ensure_stream
from internal.consumer.consumer import Consumer
from internal.db.connection import connect as db_connect, migrate
from internal.notifications.repository import NotificationRepository


@pytest.fixture
def nats_url():
    container = (
        DockerContainer("nats:2.10-alpine")
        .with_command(["-js"])
        .with_exposed_ports(4222)
        .waiting_for(wait_for_logs("Server is ready"))
    )
    container.start()
    try:
        host = container.get_container_host_ip()
        port = container.get_exposed_port(4222)
        yield f"nats://{host}:{port}"
    finally:
        container.stop()


async def _wait_until(predicate, timeout=5.0, interval=0.1):
    elapsed = 0.0
    while elapsed < timeout:
        if predicate():
            return True
        await asyncio.sleep(interval)
        elapsed += interval
    return False


async def test_consumer_records_hold_skips_pass_and_dedupes(nats_url):
    with PostgresContainer("postgres:18-alpine") as postgres:
        dsn = postgres.get_connection_url(driver=None)
        migrate(dsn)
        conn = db_connect(dsn)
        repo = NotificationRepository(conn)

        nc, js = await nats_connect(nats_url)
        await ensure_stream(js, "SETTLEMENT_EVENTS", ["settlement.>", "transaction.risk-scored"])

        consumer = Consumer(repo)
        await consumer.start(js)

        held_tx_id = "11111111-1111-1111-1111-111111111111"
        passed_tx_id = "22222222-2222-2222-2222-222222222222"
        settlement_id = "33333333-3333-3333-3333-333333333333"

        await js.publish(
            "transaction.risk-scored",
            json.dumps({"transaction_id": held_tx_id, "decision": "hold", "score": 40}).encode(),
        )
        await js.publish(
            "transaction.risk-scored",
            json.dumps({"transaction_id": passed_tx_id, "decision": "pass", "score": 0}).encode(),
        )
        # redeliver the same held event again -- must not create a second row
        await js.publish(
            "transaction.risk-scored",
            json.dumps({"transaction_id": held_tx_id, "decision": "hold", "score": 40}).encode(),
        )
        await js.publish(
            "settlement.finalized",
            json.dumps({"settlement_id": settlement_id, "transaction_count": 3}).encode(),
        )

        def rows():
            with conn.cursor() as cur:
                cur.execute("SELECT type, subject_id FROM notifications ORDER BY type")
                return cur.fetchall()

        found = await _wait_until(lambda: len(rows()) == 2)
        assert found, f"expected 2 rows, got {rows()}"

        result = {(t, str(sid)) for t, sid in rows()}
        assert result == {("risk_hold", held_tx_id), ("settlement_finalized", settlement_id)}

        await nc.close()
        conn.close()
```

- [ ] **Step 6: Run to verify it fails**

Run: `pytest internal/consumer/consumer_test.py -v`
Expected: this specific test fails at this point only if `Consumer`/wiring
has a bug — since `Consumer` was already implemented in Step 3, this test
should actually PASS immediately. Run it anyway to confirm: if it fails,
debug the consumer wiring (common cause: `stream=` mismatch or the
durable consumer not yet propagating before `publish` — the `_wait_until`
poll already accounts for async delivery lag).

- [ ] **Step 7: Run full test file to verify everything passes**

Run: `pytest internal/consumer/consumer_test.py -v`
Expected: 4 passed. Requires Docker running.

- [ ] **Step 8: Document the new business invariant**

Add to `docs/BUSINESS_RULES.md`, in the `## settlement-engine` section
(new subsection right after it, before `## Quy tắc xuyên suốt`):

```markdown
## notification-service

- **NOTIFICATION-01** — Chỉ tạo notification cho `transaction.risk-scored`
  khi `decision == "hold"`; bỏ qua (Ack, không ghi) khi `decision ==
  "pass"`. `settlement.finalized` luôn tạo notification.
  **Vì sao:** charter chỉ yêu cầu cảnh báo cho "risk holds" và settlement
  đã hoàn tất, không phải mọi giao dịch đã chấm điểm — ghi cả `pass` sẽ
  làm bảng `notifications` phình lên với dữ liệu không phải cảnh báo thật.
  **Ở đâu:** `services/notification-service/internal/consumer`
  (`decide_risk_scored`).
```

- [ ] **Step 9: Commit**

```bash
git add services/notification-service/internal/consumer/ docs/BUSINESS_RULES.md
git commit -m "feat(notification-service): risk-hold + settlement-finalized consumer"
```

---

### Task 5: `internal/api` — health endpoint

**Files:**
- Create: `services/notification-service/internal/api/__init__.py`
- Create: `services/notification-service/internal/api/health.py`
- Test: `services/notification-service/internal/api/health_test.py`

**Interfaces:**
- Produces: `internal.api.health.create_server(host: str, port: int) -> http.server.HTTPServer`.
  Task 6 (`main.py`) calls this, then runs `server.serve_forever()` in a
  background thread.

- [ ] **Step 1: Write the failing test**

`services/notification-service/internal/api/health_test.py`:

```python
import threading
import urllib.request

from internal.api.health import create_server


def test_health_endpoint_returns_200():
    server = create_server("127.0.0.1", 0)  # port 0 = OS picks a free port
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        port = server.server_address[1]
        response = urllib.request.urlopen(f"http://127.0.0.1:{port}/health")
        assert response.status == 200
    finally:
        server.shutdown()
        thread.join()


def test_unknown_path_returns_404():
    server = create_server("127.0.0.1", 0)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        port = server.server_address[1]
        try:
            urllib.request.urlopen(f"http://127.0.0.1:{port}/nope")
            assert False, "expected HTTPError"
        except urllib.error.HTTPError as e:
            assert e.code == 404
    finally:
        server.shutdown()
        thread.join()
```

- [ ] **Step 2: Run to verify it fails**

Run: `pytest internal/api/health_test.py -v`
Expected: FAIL — `internal.api.health` doesn't exist yet.

- [ ] **Step 3: Implement `internal/api/health.py`**

```python
from http.server import BaseHTTPRequestHandler, HTTPServer


class _HealthHandler(BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path == "/health":
            self.send_response(200)
            self.end_headers()
            self.wfile.write(b"ok")
        else:
            self.send_response(404)
            self.end_headers()

    def log_message(self, format, *args):
        pass


def create_server(host: str, port: int) -> HTTPServer:
    return HTTPServer((host, port), _HealthHandler)
```

`services/notification-service/internal/api/__init__.py` — empty file.

- [ ] **Step 4: Run to verify it passes**

Run: `pytest internal/api/health_test.py -v`
Expected: 2 passed.

- [ ] **Step 5: Commit**

```bash
git add services/notification-service/internal/api/
git commit -m "feat(notification-service): health endpoint"
```

---

### Task 6: `main.py` — wire everything together

**Files:**
- Create: `services/notification-service/main.py`

**Interfaces:**
- Consumes everything produced by Tasks 1-5: `internal.db.connection.{connect,migrate}`,
  `internal.notifications.repository.NotificationRepository`,
  `internal.broker.connection.{connect,ensure_stream,SETTLEMENT_EVENTS_STREAM,SETTLEMENT_EVENTS_SUBJECTS}`,
  `internal.consumer.consumer.Consumer`, `internal.api.health.create_server`.
- No automated test for this file — mirrors `cmd/server/main.go` in the Go
  services, which is wiring-only and verified by manual/e2e run instead.

- [ ] **Step 1: Implement `main.py`**

```python
import asyncio
import os
import threading

from internal.api.health import create_server
from internal.broker.connection import (
    SETTLEMENT_EVENTS_STREAM,
    SETTLEMENT_EVENTS_SUBJECTS,
    connect as nats_connect,
    ensure_stream,
)
from internal.consumer.consumer import Consumer
from internal.db.connection import connect as db_connect, migrate
from internal.notifications.repository import NotificationRepository


async def main() -> None:
    dsn = os.environ.get("DATABASE_URL")
    if not dsn:
        raise SystemExit("DATABASE_URL environment variable is required")
    nats_url = os.environ.get("NATS_URL")
    if not nats_url:
        raise SystemExit("NATS_URL environment variable is required")

    migrate(dsn)
    conn = db_connect(dsn)
    repo = NotificationRepository(conn)

    nc, js = await nats_connect(nats_url)
    await ensure_stream(js, SETTLEMENT_EVENTS_STREAM, SETTLEMENT_EVENTS_SUBJECTS)

    consumer = Consumer(repo)
    await consumer.start(js)

    addr = os.environ.get("LISTEN_ADDR", ":8083")
    host, port = addr.rsplit(":", 1)
    server = create_server(host or "0.0.0.0", int(port))
    threading.Thread(target=server.serve_forever, daemon=True).start()
    print(f"notification-service listening on {addr}")

    try:
        await asyncio.Event().wait()
    finally:
        server.shutdown()
        await nc.close()
        conn.close()


if __name__ == "__main__":
    asyncio.run(main())
```

- [ ] **Step 2: Verify it starts against real infra**

Requires `settlement-postgres`... no — requires only this service's own
Postgres + a shared NATS. From `infra/docker/docker-compose.yml` (after
Task 7 adds `notification-postgres`, see below):

```bash
docker compose -f infra/docker/docker-compose.yml up -d notification-postgres nats
```

Then, from `services/notification-service/` with the Task 1 virtualenv
activated:

```bash
export DATABASE_URL="postgres://notification:notification@localhost:5436/notification?sslmode=disable"
export NATS_URL="nats://localhost:4222"
python main.py
```

Expected console output: `notification-service listening on :8083`, no
tracebacks. In a second terminal:

```bash
curl http://localhost:8083/health
```

Expected: `ok` with HTTP 200.

- [ ] **Step 3: Manually verify the consume path end-to-end**

With `main.py` still running, publish a test event directly (no need for
`settlement-engine` to be running for this smoke check — that full
cross-service path gets exercised for real once
`settlement-engine`/`ledger-service` are also up, which is worth doing
once but isn't required for every future change to this service):

```bash
python -c "
import asyncio, json, nats

async def main():
    nc = await nats.connect('nats://localhost:4222')
    js = nc.jetstream()
    await js.publish('transaction.risk-scored', json.dumps({
        'transaction_id': '44444444-4444-4444-4444-444444444444',
        'decision': 'hold',
        'score': 40,
    }).encode())
    await nc.close()

asyncio.run(main())
"
```

Then check the row landed:

```bash
docker exec -it settleguard-notification-dev psql -U notification -d notification \
  -c "SELECT type, subject_id FROM notifications;"
```

Expected: one row, `risk_hold | 44444444-4444-4444-4444-444444444444`.

- [ ] **Step 4: Commit**

```bash
git add services/notification-service/main.py
git commit -m "feat(notification-service): wire main.py"
```

---

### Task 7: docker-compose — add `notification-postgres`

**Files:**
- Modify: `infra/docker/docker-compose.yml`

**Interfaces:** None — infra config only, no code interface.

- [ ] **Step 1: Add the service block and volume**

In `infra/docker/docker-compose.yml`, add after the `settlement-postgres`
block (before `nats:`):

```yaml
  notification-postgres:
    image: postgres:18-alpine
    container_name: settleguard-notification-dev
    env_file:
      - ../../services/notification-service/.env
    ports:
      - "5436:5432"
    volumes:
      - notification_dev_data:/var/lib/postgresql
```

And add to the `volumes:` block at the bottom:

```yaml
  notification_dev_data:
```

Note: this references `services/notification-service/.env`, which is
`.gitignore`d like the other services' `.env` files — copy
`.env.example` (Task 1) to `.env` locally before running compose:

```bash
cp services/notification-service/.env.example services/notification-service/.env
```

- [ ] **Step 2: Verify the compose file is valid and the container starts**

Run:

```bash
docker compose -f infra/docker/docker-compose.yml config --quiet
docker compose -f infra/docker/docker-compose.yml up -d notification-postgres
docker compose -f infra/docker/docker-compose.yml ps notification-postgres
```

Expected: `config --quiet` prints nothing (valid YAML); `ps` shows the
container `Up`/`healthy`.

- [ ] **Step 3: Commit**

```bash
git add infra/docker/docker-compose.yml
git commit -m "chore(notification-service): add notification-postgres compose block"
```

---

### Task 8: README

**Files:**
- Create: `services/notification-service/README.md`

**Interfaces:** None — documentation only.

- [ ] **Step 1: Write the README**

`services/notification-service/README.md`:

```markdown
# notification-service

Consumes `settlement-engine`'s `transaction.risk-scored` (hold decisions
only) and `settlement.finalized` events from NATS JetStream and persists
each as a row in `notifications` — an audit trail and insertion point for
a real delivery channel (email/push), neither of which exists yet. See
`docs/superpowers/specs/2026-08-16-notification-service-design.md` for
the full design rationale and `docs/PROJECT_CHARTER.md` for system-wide
context.

## Run locally

Requires a reachable Postgres instance, a NATS server with JetStream
enabled, and the `migrate` CLI (golang-migrate) on PATH:

```bash
docker compose -f infra/docker/docker-compose.yml up -d notification-postgres nats
cp services/notification-service/.env.example services/notification-service/.env  # first time only
```

```bash
cd services/notification-service
python -m venv .venv && source .venv/Scripts/activate  # if not already created
pip install -r requirements.txt
export DATABASE_URL="postgres://notification:notification@localhost:5436/notification?sslmode=disable"
export NATS_URL="nats://localhost:4222"
python main.py
```

Migrations run automatically on startup (via the `migrate` CLI). Listens
on `:8083` by default (override with `LISTEN_ADDR`) so it can run
alongside `ledger-service` (`:8080`), `accounts-service` (`:8081`), and
`settlement-engine` (`:8082`) locally.

## What gets notified

- **`transaction.risk-scored`** — only when `decision == "hold"`. Events
  with `decision == "pass"` are acknowledged and dropped (see
  `NOTIFICATION-01` in `docs/BUSINESS_RULES.md`).
- **`settlement.finalized`** — always.

Both are idempotent on `(type, subject_id)`: redelivery of the same event
(NATS JetStream is at-least-once) never creates a duplicate row.

## Event contracts

- **Consumes** `transaction.risk-scored` and `settlement.finalized`
  (stream `SETTLEMENT_EVENTS`, owned by `settlement-engine`), on durable
  consumers `notification-service-risk-hold` and
  `notification-service-settlement-finalized` respectively.
- **Publishes** nothing — this service is a terminal consumer in v1, so
  there is no outbox here.

## Test

Requires Docker (tests use `testcontainers-python` to run against real
Postgres and real NATS — no mocks):

```bash
pytest
```

Run a single test:

```bash
pytest internal/consumer/consumer_test.py::test_consumer_records_hold_skips_pass_and_dedupes -v
```

## API

- `GET /health` — health check. This service is event-driven end-to-end;
  there is no other HTTP surface.
```

- [ ] **Step 2: Commit**

```bash
git add services/notification-service/README.md
git commit -m "docs(notification-service): add README"
```
