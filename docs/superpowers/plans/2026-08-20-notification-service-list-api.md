# notification-service: GET /notifications read API

> **For agentic workers:** REQUIRED SUB-SKILL: Use
> superpowers:subagent-driven-development (recommended) or
> superpowers:executing-plans to implement this plan task-by-task. Steps
> use checkbox (`- [ ]`) syntax for tracking.

**Goal:** notification-service's MVP spec
(`docs/superpowers/specs/2026-08-16-notification-service-design.md` §7)
explicitly deferred any HTTP surface beyond `GET /health`: *"chưa có use
case cụ thể yêu cầu ở v1"*. That use case now exists — `mobile-app`'s
Alerts screen (`docs/superpowers/specs/2026-08-19-mobile-app-design.md`
§3-4) needs to read back what's already sitting in the `notifications`
table. This plan adds exactly one endpoint, `GET /notifications`, with
optional `type`/`since` filters and a capped `limit` — nothing else (no
write endpoints, no auth, still publishes nothing — this service stays a
terminal consumer).

**Branch:** `service/notification-service` (extends the existing merged
MVP).

**Blocks:** `mobile-app` plan
(`docs/superpowers/plans/2026-08-19-mobile-app-mvp.md`) Task 7.

## Design decisions

1. **Still stdlib `http.server`, no framework.** One more route doesn't
   change the calculus from the original MVP spec (*"chỉ cần một endpoint
   `/health`, không cần FastAPI/Flask vì service này không bị service khác
   gọi synchronous"*) — a second read-only route is still well within what
   `http.server` handles cleanly. Adding FastAPI/Flask now for two routes
   total would be exactly the kind of premature abstraction the project
   avoids.
2. **The handler needs access to the repository, so `create_server` gains
   a `repo` parameter.** `BaseHTTPRequestHandler` is instantiated
   per-request by `HTTPServer` with no constructor args of its own; the
   standard stdlib pattern for handler-needs-shared-state is to stash it
   on the server instance (`self.server.repo` from inside the handler) via
   a small `HTTPServer` subclass — no new dependency, no global.
3. **`limit` is capped (default 50, max 200), `type`/`since` are optional
   filters, sort is always `created_at DESC`.** Mirrors why
   settlement-engine's new `GET /transactions` requires an explicit filter
   rather than dumping everything — the one caller (mobile-app's polling
   Alerts screen) always wants "most recent N", never a full table scan.
4. **No new business rule for `docs/BUSINESS_RULES.md`.** This is a pure
   read path with no invariant to violate — `NOTIFICATION-01` already
   covers the one rule that governs what ends up in the table in the first
   place.

---

### Task 1: `NotificationRepository.list()`

**Files:**
- Modify: `internal/notifications/repository.py`
- Test: `internal/notifications/repository_test.py`

**Interfaces:**
- Produces: `NotificationRepository.list(type_: str | None = None, since:
  datetime | None = None, limit: int = 50) -> list[dict]`, each dict
  shaped `{"id": str, "type": str, "subject_id": str, "payload": dict,
  "created_at": str}` (ISO 8601). Task 2's HTTP handler calls this
  directly and JSON-encodes the result as-is.

- [ ] **Step 1: Write the failing tests**

Append to `internal/notifications/repository_test.py`:

```python
from datetime import datetime, timedelta, timezone


def test_list_returns_most_recent_first():
    with PostgresContainer("postgres:18-alpine") as postgres:
        dsn = postgres.get_connection_url(driver=None)
        migrate(dsn)
        repo, conn = _repo(dsn)
        older_id, newer_id = uuid.uuid4(), uuid.uuid4()

        repo.record("risk_hold", older_id, {"decision": "hold"})
        repo.record("settlement_finalized", newer_id, {"transaction_count": 1})

        result = repo.list()

        assert [n["subject_id"] for n in result] == [str(newer_id), str(older_id)]
        conn.close()


def test_list_filters_by_type():
    with PostgresContainer("postgres:18-alpine") as postgres:
        dsn = postgres.get_connection_url(driver=None)
        migrate(dsn)
        repo, conn = _repo(dsn)
        hold_id, settlement_id = uuid.uuid4(), uuid.uuid4()
        repo.record("risk_hold", hold_id, {"decision": "hold"})
        repo.record("settlement_finalized", settlement_id, {"transaction_count": 1})

        result = repo.list(type_="risk_hold")

        assert [n["subject_id"] for n in result] == [str(hold_id)]
        conn.close()


def test_list_filters_by_since():
    with PostgresContainer("postgres:18-alpine") as postgres:
        dsn = postgres.get_connection_url(driver=None)
        migrate(dsn)
        repo, conn = _repo(dsn)
        repo.record("risk_hold", uuid.uuid4(), {"decision": "hold"})

        future_cutoff = datetime.now(timezone.utc) + timedelta(hours=1)
        result = repo.list(since=future_cutoff)

        assert result == []
        conn.close()


def test_list_respects_limit():
    with PostgresContainer("postgres:18-alpine") as postgres:
        dsn = postgres.get_connection_url(driver=None)
        migrate(dsn)
        repo, conn = _repo(dsn)
        for _ in range(3):
            repo.record("risk_hold", uuid.uuid4(), {"decision": "hold"})

        result = repo.list(limit=2)

        assert len(result) == 2
        conn.close()
```

- [ ] **Step 2: Run to verify it fails**

`pytest internal/notifications/repository_test.py -v` — `list` doesn't
exist yet.

- [ ] **Step 3: Implement**

```python
from datetime import datetime


class NotificationRepository:
    # ... existing __init__/record unchanged ...

    def list(self, type_: str | None = None, since: datetime | None = None, limit: int = 50) -> list[dict]:
        query = "SELECT id, type, subject_id, payload, created_at FROM notifications WHERE TRUE"
        params: list = []
        if type_ is not None:
            query += " AND type = %s"
            params.append(type_)
        if since is not None:
            query += " AND created_at >= %s"
            params.append(since)
        query += " ORDER BY created_at DESC LIMIT %s"
        params.append(limit)

        with self._conn.cursor() as cur:
            cur.execute(query, params)
            rows = cur.fetchall()

        return [
            {
                "id": str(row[0]),
                "type": row[1],
                "subject_id": str(row[2]),
                "payload": row[3],
                "created_at": row[4].isoformat(),
            }
            for row in rows
        ]
```

- [ ] **Step 4: Run to verify it passes**

`pytest internal/notifications/repository_test.py -v` — all pass.

- [ ] **Step 5: Commit**

```bash
git add internal/notifications/repository.py internal/notifications/repository_test.py
git commit -m "feat(notification-service): NotificationRepository.list with type/since/limit filters"
```

---

### Task 2: `/notifications` HTTP route

**Files:**
- Modify: `internal/api/health.py`
- Test: `internal/api/health_test.py`

**Interfaces:**
- Changes: `create_server(host: str, port: int) -> HTTPServer` becomes
  `create_server(host: str, port: int, repo: NotificationRepository) ->
  HTTPServer`. Task 3 (`main.py`) updates its one call site.
- Adds route `GET /notifications?type=&since=&limit=` → `200` JSON array
  (see Task 1 shape) or `400` on an invalid `since`/`limit`.

- [ ] **Step 1: Write the failing tests**

Append to `internal/api/health_test.py`:

```python
import json
import uuid
from unittest.mock import MagicMock

from internal.api.health import create_server


def test_notifications_endpoint_returns_repo_list():
    repo = MagicMock()
    repo.list.return_value = [
        {"id": str(uuid.uuid4()), "type": "risk_hold", "subject_id": str(uuid.uuid4()), "payload": {}, "created_at": "2026-08-20T00:00:00+00:00"}
    ]
    server = create_server("127.0.0.1", 0, repo)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        port = server.server_address[1]
        response = urllib.request.urlopen(f"http://127.0.0.1:{port}/notifications")
        body = json.loads(response.read())
        assert response.status == 200
        assert body == repo.list.return_value
    finally:
        server.shutdown()
        thread.join()


def test_notifications_endpoint_passes_query_params_to_repo():
    repo = MagicMock()
    repo.list.return_value = []
    server = create_server("127.0.0.1", 0, repo)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        port = server.server_address[1]
        urllib.request.urlopen(f"http://127.0.0.1:{port}/notifications?type=risk_hold&limit=10")
        repo.list.assert_called_once_with(type_="risk_hold", since=None, limit=10)
    finally:
        server.shutdown()
        thread.join()


def test_notifications_endpoint_rejects_invalid_limit():
    repo = MagicMock()
    server = create_server("127.0.0.1", 0, repo)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        port = server.server_address[1]
        try:
            urllib.request.urlopen(f"http://127.0.0.1:{port}/notifications?limit=not-a-number")
            assert False, "expected HTTPError"
        except urllib.error.HTTPError as e:
            assert e.code == 400
    finally:
        server.shutdown()
        thread.join()
```

Update the two existing tests (`test_health_endpoint_returns_200`,
`test_unknown_path_returns_404`) to pass a `MagicMock()` as the new third
`create_server` argument.

- [ ] **Step 2: Run to verify failure**

`pytest internal/api/health_test.py -v` — fails, `create_server` doesn't
accept a third argument yet, `/notifications` 404s.

- [ ] **Step 3: Implement `internal/api/health.py`**

```python
import json
from datetime import datetime
from http.server import BaseHTTPRequestHandler, HTTPServer
from urllib.parse import parse_qs, urlparse

MAX_LIMIT = 200
DEFAULT_LIMIT = 50


class _NotificationServer(HTTPServer):
    def __init__(self, address, repo):
        super().__init__(address, _Handler)
        self.repo = repo


class _Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        parsed = urlparse(self.path)
        if parsed.path == "/health":
            self._write(200, b"ok", content_type="text/plain")
        elif parsed.path == "/notifications":
            self._handle_notifications(parsed)
        else:
            self.send_response(404)
            self.end_headers()

    def _handle_notifications(self, parsed):
        params = parse_qs(parsed.query)
        type_ = params.get("type", [None])[0]
        since_raw = params.get("since", [None])[0]
        limit_raw = params.get("limit", [str(DEFAULT_LIMIT)])[0]

        since = None
        if since_raw is not None:
            try:
                since = datetime.fromisoformat(since_raw)
            except ValueError:
                self._write(400, b'{"error":"invalid since"}')
                return

        try:
            limit = min(int(limit_raw), MAX_LIMIT)
        except ValueError:
            self._write(400, b'{"error":"invalid limit"}')
            return

        notifications = self.server.repo.list(type_=type_, since=since, limit=limit)
        self._write(200, json.dumps(notifications).encode())

    def _write(self, status: int, body: bytes, content_type: str = "application/json"):
        self.send_response(status)
        self.send_header("Content-Type", content_type)
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, format, *args):
        pass


def create_server(host: str, port: int, repo) -> HTTPServer:
    return _NotificationServer((host, port), repo)
```

- [ ] **Step 4: Run to verify it passes**

`pytest internal/api/health_test.py -v`

- [ ] **Step 5: Commit**

```bash
git add internal/api/health.py internal/api/health_test.py
git commit -m "feat(notification-service): GET /notifications endpoint"
```

---

### Task 3: Wire `main.py`

**Files:**
- Modify: `main.py`

- [ ] **Step 1: Pass `repo` into `create_server`**

```python
server = create_server(host or "0.0.0.0", int(port), repo)
```

(`repo` is already constructed earlier in `main()` for the consumer — no
new wiring beyond passing the existing variable through.)

- [ ] **Step 2: Manual smoke test**

```bash
docker compose -f infra/docker/docker-compose.yml up -d notification-postgres nats
cd services/notification-service && source .venv/Scripts/activate
export DATABASE_URL="postgres://notification:notification@localhost:5436/notification?sslmode=disable"
export NATS_URL="nats://localhost:4222"
python main.py
```

In another terminal, publish a test event exactly as the notification-service
README's existing smoke-test snippet does, then:

```bash
curl "http://localhost:8083/notifications"
curl "http://localhost:8083/notifications?type=risk_hold&limit=5"
```

Expected: JSON array containing the notification recorded from the
published event.

- [ ] **Step 3: Commit**

```bash
git add main.py
git commit -m "feat(notification-service): wire repo into HTTP server"
```

---

### Task 4: README

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Update the `## API` section** — replace the current single
  bullet (`GET /health — ... there is no other HTTP surface`) with:

```markdown
## API

- `GET /health` — health check.
- `GET /notifications?type=&since=&limit=` — list notifications, most
  recent first. `type` (`risk_hold` | `settlement_finalized`) and `since`
  (ISO 8601 timestamp) are optional filters. `limit` defaults to 50,
  capped at 200. `400` if `since`/`limit` fail to parse.
```

- [ ] **Step 2: Commit**

```bash
git add README.md
git commit -m "docs(notification-service): document GET /notifications"
```
