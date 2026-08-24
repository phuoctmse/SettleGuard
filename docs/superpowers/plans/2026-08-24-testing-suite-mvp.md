# tests/ suite MVP Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use
> superpowers:subagent-driven-development (recommended) or
> superpowers:executing-plans to implement this plan task-by-task. Steps
> use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build `tests/api`, `tests/security` (fraud-bypass only), and
`tests/perf`, per
`docs/superpowers/specs/2026-08-24-testing-suite-design.md`. These are the
three areas of the `tests/` layout described in `CLAUDE.md` that aren't
blocked by a missing prerequisite — `tests/ai-tools`, auth boundary
checks, and mobile Appium E2E all need something that doesn't exist yet
(a real OpenAPI spec, CI, or `testID`s in `mobile-app`) and are explicitly
out of scope here (see design spec §8).

**Branch:** `step/testing-suite` (repo-level step, not owned by one
service).

**Prerequisite for actually running these tests:** all 4 backend services
plus their Postgres instances and NATS must be running locally — there is
no single-command way to start the whole stack yet
(`infra/docker/docker-compose.yml` only has the datastores). See Task 1's
`conftest.py` for how tests behave when something's missing, and Task 7's
README for the exact multi-terminal startup sequence.

**Tech Stack:** Python 3.11+, `pytest`, `httpx` (HTTP client), `locust`
(load testing). No OpenAPI-driven codegen (spec is empty) — contract
tests are written directly against each service's documented `## API`
section.

## Global Constraints

- Layout: `tests/{api,security,perf}/`, shared `tests/pyproject.toml` +
  `tests/requirements.txt` at the `tests/` root (one Python environment
  for the whole suite, not per-subdirectory — these are tightly related
  and always run together).
- Env vars (all optional, default to each service's documented local
  port): `ACCOUNTS_API_URL` (default `http://localhost:8081`),
  `LEDGER_API_URL` (`http://localhost:8080`), `SETTLEMENT_API_URL`
  (`http://localhost:8082`), `NOTIFICATION_API_URL`
  (`http://localhost:8083`).
- `tests/security` imports `tests/api/clients` directly (both live under
  the same `tests/` pytest rootdir) — no duplicate HTTP client code.
- Run all commands below from inside `tests/`.

---

### Task 1: Scaffold + health-check conftest

**Files:**
- Create: `tests/requirements.txt`, `tests/pyproject.toml`,
  `tests/api/conftest.py`, `tests/api/clients/__init__.py`

**Interfaces:**
- Produces: session-scoped pytest fixtures `accounts_base_url`,
  `ledger_base_url`, `settlement_base_url`, `notification_base_url` (each
  reads its env var, GETs `/health`, `pytest.skip(...)` with a clear
  message if unreachable within 2s). Task 2's clients and every test file
  after depend on these.

- [ ] **Step 1: Scaffold files**

`tests/requirements.txt`:

```
pytest>=8.0,<9
httpx>=0.27,<1
locust>=2.25,<3
```

`tests/pyproject.toml`:

```toml
[tool.pytest.ini_options]
testpaths = ["api", "security"]
```

- [ ] **Step 2: Write `conftest.py`**

`tests/api/conftest.py`:

```python
import os

import httpx
import pytest

SERVICES = {
    "accounts": os.environ.get("ACCOUNTS_API_URL", "http://localhost:8081"),
    "ledger": os.environ.get("LEDGER_API_URL", "http://localhost:8080"),
    "settlement": os.environ.get("SETTLEMENT_API_URL", "http://localhost:8082"),
    "notification": os.environ.get("NOTIFICATION_API_URL", "http://localhost:8083"),
}


def _require_reachable(name: str, base_url: str) -> str:
    try:
        resp = httpx.get(f"{base_url}/health", timeout=2.0)
        resp.raise_for_status()
    except httpx.HTTPError as exc:
        pytest.skip(f"{name}-service not reachable at {base_url} ({exc}) -- start it first, see tests/README.md")
    return base_url


@pytest.fixture(scope="session")
def accounts_base_url():
    return _require_reachable("accounts", SERVICES["accounts"])


@pytest.fixture(scope="session")
def ledger_base_url():
    return _require_reachable("ledger", SERVICES["ledger"])


@pytest.fixture(scope="session")
def settlement_base_url():
    return _require_reachable("settlement", SERVICES["settlement"])


@pytest.fixture(scope="session")
def notification_base_url():
    return _require_reachable("notification", SERVICES["notification"])
```

`tests/api/clients/__init__.py` — empty file.

- [ ] **Step 3: Verify the skip behavior manually**

Run `pytest tests/api -v` with no services running (nothing else exists
yet, so this just confirms fixtures import cleanly — full skip behavior
gets exercised once Task 3 adds real tests that request these fixtures).

- [ ] **Step 4: Commit**

```bash
git add tests/requirements.txt tests/pyproject.toml tests/api/conftest.py tests/api/clients/__init__.py
git commit -m "chore(tests): scaffold tests/ + service-reachability conftest"
```

---

### Task 2: HTTP clients per service

**Files:**
- Create: `tests/api/clients/accounts.py`, `tests/api/clients/ledger.py`,
  `tests/api/clients/settlement.py`, `tests/api/clients/notifications.py`

**Interfaces:**
- Produces: one class per service, each a thin `httpx.Client` wrapper
  with one method per documented endpoint, returning
  `httpx.Response` directly (tests assert on `.status_code`/`.json()`
  themselves — no extra parsing layer, since these responses are small
  and test-local). Task 3/4/5 construct and use these.

- [ ] **Step 1: `tests/api/clients/accounts.py`**

```python
import httpx


class AccountsClient:
    def __init__(self, base_url: str):
        self._http = httpx.Client(base_url=base_url, timeout=5.0)

    def create_client(self, name: str) -> httpx.Response:
        return self._http.post("/clients", json={"name": name})

    def get_client(self, client_id: str) -> httpx.Response:
        return self._http.get(f"/clients/{client_id}")

    def update_client_status(self, client_id: str, status: str) -> httpx.Response:
        return self._http.patch(f"/clients/{client_id}/status", json={"status": status})

    def create_account(self, client_id: str, external_ref: str = "") -> httpx.Response:
        return self._http.post("/accounts", json={"client_id": client_id, "external_ref": external_ref})

    def get_account(self, account_id: str) -> httpx.Response:
        return self._http.get(f"/accounts/{account_id}")

    def list_accounts(self, client_id: str) -> httpx.Response:
        return self._http.get("/accounts", params={"client_id": client_id})

    def update_account_status(self, account_id: str, status: str) -> httpx.Response:
        return self._http.patch(f"/accounts/{account_id}/status", json={"status": status})
```

- [ ] **Step 2: `tests/api/clients/ledger.py`**

```python
import httpx


class LedgerClient:
    def __init__(self, base_url: str):
        self._http = httpx.Client(base_url=base_url, timeout=5.0)

    def post_transaction(self, entries: list[dict]) -> httpx.Response:
        return self._http.post("/transactions", json={"entries": entries})

    def list_entries(self, *, account_id: str | None = None, transaction_id: str | None = None) -> httpx.Response:
        params = {}
        if account_id:
            params["account_id"] = account_id
        if transaction_id:
            params["transaction_id"] = transaction_id
        return self._http.get("/entries", params=params)
```

- [ ] **Step 3: `tests/api/clients/settlement.py`**

```python
import httpx


class SettlementClient:
    def __init__(self, base_url: str):
        self._http = httpx.Client(base_url=base_url, timeout=5.0)

    def get_transaction(self, transaction_id: str) -> httpx.Response:
        return self._http.get(f"/transactions/{transaction_id}")

    def list_transactions(self, status: str) -> httpx.Response:
        return self._http.get("/transactions", params={"status": status})

    def approve(self, transaction_id: str) -> httpx.Response:
        return self._http.post(f"/transactions/{transaction_id}/approve")

    def reject(self, transaction_id: str) -> httpx.Response:
        return self._http.post(f"/transactions/{transaction_id}/reject")

    def list_settlements(self) -> httpx.Response:
        return self._http.get("/settlements")

    def get_settlement(self, settlement_id: str) -> httpx.Response:
        return self._http.get(f"/settlements/{settlement_id}")
```

- [ ] **Step 4: `tests/api/clients/notifications.py`**

```python
import httpx


class NotificationsClient:
    def __init__(self, base_url: str):
        self._http = httpx.Client(base_url=base_url, timeout=5.0)

    def list_notifications(self, *, type_: str | None = None, limit: int | None = None) -> httpx.Response:
        params = {}
        if type_:
            params["type"] = type_
        if limit:
            params["limit"] = limit
        return self._http.get("/notifications", params=params)
```

- [ ] **Step 5: Add fixtures to `conftest.py` that construct these**

Append to `tests/api/conftest.py`:

```python
from clients.accounts import AccountsClient
from clients.ledger import LedgerClient
from clients.notifications import NotificationsClient
from clients.settlement import SettlementClient


@pytest.fixture
def accounts(accounts_base_url):
    return AccountsClient(accounts_base_url)


@pytest.fixture
def ledger(ledger_base_url):
    return LedgerClient(ledger_base_url)


@pytest.fixture
def settlement(settlement_base_url):
    return SettlementClient(settlement_base_url)


@pytest.fixture
def notifications(notification_base_url):
    return NotificationsClient(notification_base_url)
```

(Requires `tests/api` on `sys.path` for the bare `from clients...`
import to resolve — add `pythonpath = ["api"]` under
`[tool.pytest.ini_options]` in `tests/pyproject.toml`, pytest's built-in
mechanism for this, no `conftest.py` sys.path hacking needed.)

- [ ] **Step 6: Commit**

```bash
git add tests/api/clients/ tests/api/conftest.py tests/pyproject.toml
git commit -m "feat(tests): HTTP clients for all 4 backend services"
```

---

### Task 3: Per-service contract tests

**Files:**
- Create: `tests/api/test_accounts_contract.py`,
  `tests/api/test_ledger_contract.py`,
  `tests/api/test_settlement_contract.py`,
  `tests/api/test_notification_contract.py`

**Interfaces:** Consumes Task 2's fixtures. No new interfaces produced.

- [ ] **Step 1: `test_accounts_contract.py`**

```python
def test_create_client_rejects_empty_name(accounts):
    resp = accounts.create_client("")
    assert resp.status_code == 400


def test_create_and_get_client(accounts):
    created = accounts.create_client("Test Client Co").json()
    assert created["status"] == "active"

    fetched = accounts.get_client(created["id"])
    assert fetched.status_code == 200
    assert fetched.json()["name"] == "Test Client Co"


def test_get_client_not_found(accounts):
    resp = accounts.get_client("00000000-0000-0000-0000-000000000000")
    assert resp.status_code == 404


def test_create_account_rejects_unknown_client(accounts):
    resp = accounts.create_account("00000000-0000-0000-0000-000000000000")
    assert resp.status_code == 404


def test_create_account_rejects_suspended_client(accounts):
    client = accounts.create_client("Suspend Me Co").json()
    accounts.update_client_status(client["id"], "suspended")

    resp = accounts.create_account(client["id"])

    assert resp.status_code == 422


def test_list_accounts_by_client(accounts):
    client = accounts.create_client("List Accounts Co").json()
    created = accounts.create_account(client["id"], external_ref="ext-1").json()

    resp = accounts.list_accounts(client["id"])

    assert resp.status_code == 200
    assert created["id"] in [a["id"] for a in resp.json()]
```

- [ ] **Step 2: `test_ledger_contract.py`**

```python
import uuid


def _account_id(accounts):
    client = accounts.create_client(f"Ledger Test {uuid.uuid4()}").json()
    return accounts.create_account(client["id"]).json()["id"]


def test_post_transaction_rejects_unbalanced_entries(ledger, accounts):
    acc = _account_id(accounts)

    resp = ledger.post_transaction(
        [{"account_id": acc, "direction": "debit", "amount": 100, "reason": "test"}]
    )

    assert resp.status_code == 422


def test_post_balanced_transaction_and_list_entries(ledger, accounts):
    acc1, acc2 = _account_id(accounts), _account_id(accounts)

    resp = ledger.post_transaction(
        [
            {"account_id": acc1, "direction": "debit", "amount": 500, "reason": "test"},
            {"account_id": acc2, "direction": "credit", "amount": 500, "reason": "test"},
        ]
    )
    assert resp.status_code in (200, 201)

    entries = ledger.list_entries(account_id=acc1)
    assert entries.status_code == 200
    assert len(entries.json()) >= 1
```

- [ ] **Step 3: `test_settlement_contract.py`**

```python
def test_list_transactions_requires_status(settlement):
    resp = settlement.list_transactions("")
    assert resp.status_code == 400


def test_get_transaction_not_found(settlement):
    resp = settlement.get_transaction("00000000-0000-0000-0000-000000000000")
    assert resp.status_code == 404


def test_approve_not_found(settlement):
    resp = settlement.approve("00000000-0000-0000-0000-000000000000")
    assert resp.status_code == 404


def test_reject_not_found(settlement):
    resp = settlement.reject("00000000-0000-0000-0000-000000000000")
    assert resp.status_code == 404

    # The 409-on-not-held path needs a real held transaction to exercise
    # meaningfully -- covered by tests/security/test_fraud_bypass.py
    # (test_double_approve_second_call_returns_409), not duplicated here.


def test_list_settlements(settlement):
    resp = settlement.list_settlements()
    assert resp.status_code == 200
    assert isinstance(resp.json(), list)
```

- [ ] **Step 4: `test_notification_contract.py`**

```python
def test_list_notifications_defaults(notifications):
    resp = notifications.list_notifications()
    assert resp.status_code == 200
    assert isinstance(resp.json(), list)


def test_list_notifications_rejects_bad_limit(notification_base_url):
    import httpx

    resp = httpx.get(f"{notification_base_url}/notifications", params={"limit": "not-a-number"})
    assert resp.status_code == 400
```

- [ ] **Step 5: Run against the real stack, verify pass**

With all 4 services + Postgres + NATS running (see Task 7's README for
the exact commands):

```bash
pytest tests/api -v
```

Expected: all pass. Run with one service stopped to confirm its tests
`SKIPPED` with a clear reason instead of erroring.

- [ ] **Step 6: Commit**

```bash
git add tests/api/test_*.py
git commit -m "feat(tests): per-service contract tests"
```

---

### Task 4: End-to-end cross-service flow test

**Files:**
- Create: `tests/api/test_end_to_end_flow.py`

**Interfaces:** Consumes Task 2's fixtures. No new interfaces produced.

- [ ] **Step 1: Write the test**

```python
import time
import uuid

MISMATCH_THRESHOLD_AMOUNT = 20_000_000  # over SETTLEMENT_MISMATCH_THRESHOLD default (10_000_000) -- deterministically triggers a hold
NORMAL_AMOUNT = 1_000


def _wait_until(predicate, timeout=15.0, interval=0.5):
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        result = predicate()
        if result is not None:
            return result
        time.sleep(interval)
    raise AssertionError(f"condition not met within {timeout}s")


def _new_account(accounts):
    client = accounts.create_client(f"E2E {uuid.uuid4()}").json()
    return accounts.create_account(client["id"]).json()["id"]


def test_passing_transaction_flows_to_settlement_and_notification(accounts, ledger, settlement, notifications):
    acc1, acc2 = _new_account(accounts), _new_account(accounts)

    tx = ledger.post_transaction(
        [
            {"account_id": acc1, "direction": "debit", "amount": NORMAL_AMOUNT, "reason": "e2e"},
            {"account_id": acc2, "direction": "credit", "amount": NORMAL_AMOUNT, "reason": "e2e"},
        ]
    ).json()
    transaction_id = tx["transaction_id"]

    def scored():
        resp = settlement.get_transaction(transaction_id)
        return resp.json() if resp.status_code == 200 else None

    scored_tx = _wait_until(scored)
    assert scored_tx["status"] == "pending_settlement"

    # settlement-engine batches on a schedule (SETTLEMENT_BATCH_INTERVAL_SECONDS,
    # default 60s) -- run it with SETTLEMENT_BATCH_INTERVAL_SECONDS=5 for this
    # test to complete in reasonable time, see tests/README.md.
    def settled():
        resp = settlement.get_transaction(transaction_id)
        body = resp.json()
        return body if body["status"] == "settled" else None

    _wait_until(settled, timeout=30.0)

    def notified():
        resp = notifications.list_notifications(type_="settlement_finalized")
        matches = [n for n in resp.json() if transaction_id in str(n["payload"])]
        return matches if matches else None

    _wait_until(notified, timeout=15.0)


def test_held_transaction_can_be_approved_and_then_settles(accounts, ledger, settlement, notifications):
    acc1, acc2 = _new_account(accounts), _new_account(accounts)

    tx = ledger.post_transaction(
        [
            {"account_id": acc1, "direction": "debit", "amount": MISMATCH_THRESHOLD_AMOUNT, "reason": "e2e-hold"},
            {"account_id": acc2, "direction": "credit", "amount": MISMATCH_THRESHOLD_AMOUNT, "reason": "e2e-hold"},
        ]
    ).json()
    transaction_id = tx["transaction_id"]

    def held():
        resp = settlement.get_transaction(transaction_id)
        body = resp.json() if resp.status_code == 200 else None
        return body if body and body["status"] == "held" else None

    _wait_until(held)

    def notified_hold():
        resp = notifications.list_notifications(type_="risk_hold")
        matches = [n for n in resp.json() if transaction_id in str(n["payload"])]
        return matches if matches else None

    _wait_until(notified_hold)

    approve_resp = settlement.approve(transaction_id)
    assert approve_resp.status_code == 200
    assert approve_resp.json()["status"] == "pending_settlement"

    def settled():
        resp = settlement.get_transaction(transaction_id)
        body = resp.json()
        return body if body["status"] == "settled" else None

    _wait_until(settled, timeout=30.0)
```

- [ ] **Step 2: Run against the real stack, verify pass**

```bash
export SETTLEMENT_BATCH_INTERVAL_SECONDS=5  # set before starting settlement-engine
pytest tests/api/test_end_to_end_flow.py -v
```

Expected: both tests pass. This is the first test in the repo that
exercises all 4 services together over real NATS -- if it's flaky,
suspect the batch interval (too short a `_wait_until` timeout relative to
`SETTLEMENT_BATCH_INTERVAL_SECONDS`) before suspecting the services
themselves.

- [ ] **Step 3: Commit**

```bash
git add tests/api/test_end_to_end_flow.py
git commit -m "feat(tests): end-to-end cross-service flow test (charter v1 success criterion)"
```

---

### Task 5: `tests/security` — fraud-bypass tests

**Files:**
- Create: `tests/security/conftest.py`, `tests/security/test_fraud_bypass.py`

**Interfaces:** Reuses Task 2's `tests/api/clients/*` and Task 1's
fixtures. No new interfaces produced.

- [ ] **Step 1: `tests/security/conftest.py`**

```python
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent.parent / "api"))

pytest_plugins = ["conftest"]
```

(Reuses `tests/api/conftest.py`'s fixtures wholesale via `pytest_plugins`
-- the standard pytest mechanism for sharing fixtures across directories,
avoids copy-pasting the health-check logic.)

- [ ] **Step 2: Write the tests**

```python
import time
import uuid


def _wait_until(predicate, timeout=15.0, interval=0.5):
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        result = predicate()
        if result is not None:
            return result
        time.sleep(interval)
    raise AssertionError(f"condition not met within {timeout}s")


def _new_account(accounts):
    client = accounts.create_client(f"Security {uuid.uuid4()}").json()
    return accounts.create_account(client["id"]).json()["id"]


def test_velocity_limit_holds_transaction_over_threshold(accounts, ledger, settlement):
    # SETTLEMENT-01/02: more than SETTLEMENT_VELOCITY_LIMIT (default 5)
    # transactions for the same account inside the trailing window holds
    # the transaction that crosses the limit.
    acc1, acc2 = _new_account(accounts), _new_account(accounts)

    last_transaction_id = None
    for _ in range(7):
        tx = ledger.post_transaction(
            [
                {"account_id": acc1, "direction": "debit", "amount": 100, "reason": "velocity"},
                {"account_id": acc2, "direction": "credit", "amount": 100, "reason": "velocity"},
            ]
        ).json()
        last_transaction_id = tx["transaction_id"]

    def held():
        resp = settlement.get_transaction(last_transaction_id)
        body = resp.json() if resp.status_code == 200 else None
        return body if body and body["status"] == "held" else None

    result = _wait_until(held)
    assert "velocity_limit" in result["triggered_rules"]


def test_mismatch_threshold_holds_oversized_transaction(accounts, ledger, settlement):
    # SETTLEMENT-01: amount over SETTLEMENT_MISMATCH_THRESHOLD holds, regardless of velocity.
    acc1, acc2 = _new_account(accounts), _new_account(accounts)

    tx = ledger.post_transaction(
        [
            {"account_id": acc1, "direction": "debit", "amount": 20_000_000, "reason": "mismatch"},
            {"account_id": acc2, "direction": "credit", "amount": 20_000_000, "reason": "mismatch"},
        ]
    ).json()

    def held():
        resp = settlement.get_transaction(tx["transaction_id"])
        body = resp.json() if resp.status_code == 200 else None
        return body if body and body["status"] == "held" else None

    result = _wait_until(held)
    assert "mismatch_threshold" in result["triggered_rules"]


def test_double_approve_second_call_returns_409(accounts, ledger, settlement):
    # SETTLEMENT-05: approve/reject only valid from `held`; a second
    # resolve attempt on an already-resolved transaction must not succeed.
    acc1, acc2 = _new_account(accounts), _new_account(accounts)
    tx = ledger.post_transaction(
        [
            {"account_id": acc1, "direction": "debit", "amount": 20_000_000, "reason": "double-approve"},
            {"account_id": acc2, "direction": "credit", "amount": 20_000_000, "reason": "double-approve"},
        ]
    ).json()

    def held():
        resp = settlement.get_transaction(tx["transaction_id"])
        body = resp.json() if resp.status_code == 200 else None
        return body if body and body["status"] == "held" else None

    _wait_until(held)

    first = settlement.approve(tx["transaction_id"])
    assert first.status_code == 200

    second = settlement.approve(tx["transaction_id"])
    assert second.status_code == 409


def test_held_transaction_excluded_from_settlements_until_approved(accounts, ledger, settlement):
    # SETTLEMENT-04: a held transaction must never appear in a settlement
    # batch while still held.
    acc1, acc2 = _new_account(accounts), _new_account(accounts)
    tx = ledger.post_transaction(
        [
            {"account_id": acc1, "direction": "debit", "amount": 20_000_000, "reason": "held-exclusion"},
            {"account_id": acc2, "direction": "credit", "amount": 20_000_000, "reason": "held-exclusion"},
        ]
    ).json()

    def held():
        resp = settlement.get_transaction(tx["transaction_id"])
        body = resp.json() if resp.status_code == 200 else None
        return body if body and body["status"] == "held" else None

    _wait_until(held)

    # give a batch cycle a chance to run and confirm it was skipped
    time.sleep(6)

    for s in settlement.list_settlements().json():
        detail = settlement.get_settlement(s["id"]).json()
        assert tx["transaction_id"] not in detail["transaction_ids"]
```

- [ ] **Step 3: Run against the real stack, verify pass**

```bash
export SETTLEMENT_BATCH_INTERVAL_SECONDS=5
pytest tests/security -v
```

- [ ] **Step 4: Commit**

```bash
git add tests/security/
git commit -m "feat(tests): fraud-bypass security tests (SETTLEMENT-01/02/04/05)"
```

---

### Task 6: `tests/perf` — Locust load test

**Files:**
- Create: `tests/perf/locustfile_settlement_scoring.py`

**Interfaces:** None — standalone Locust file, not imported by pytest.

- [ ] **Step 1: Write the locustfile**

```python
import os
import uuid

from locust import HttpUser, between, task

ACCOUNTS_API_URL = os.environ.get("ACCOUNTS_API_URL", "http://localhost:8081")


class TransactionScoringUser(HttpUser):
    """
    Load-tests settlement-engine's real-time scoring path indirectly, by
    posting transactions to ledger-service (the actual entry point --
    settlement-engine has no HTTP endpoint that triggers scoring directly,
    it only consumes ledger.entry-recorded off NATS). host should be
    ledger-service's base URL, e.g.:

        locust -f locustfile_settlement_scoring.py --host=http://localhost:8080

    Note: batch settlement throughput is NOT covered here -- RunBatch is
    schedule-driven (SETTLEMENT_BATCH_INTERVAL_SECONDS), not
    HTTP-triggered, so there's no request to load-test for that path.
    Measuring it would mean sampling settlement-engine's DB/logs over a
    fixed wall-clock window instead of a request-based Locust task --
    left as a manual follow-up, not scripted here.
    """

    wait_time = between(0.1, 0.5)

    def on_start(self):
        import httpx

        client = httpx.post(f"{ACCOUNTS_API_URL}/clients", json={"name": f"perf-{uuid.uuid4()}"}).json()
        self.account_1 = httpx.post(f"{ACCOUNTS_API_URL}/accounts", json={"client_id": client["id"]}).json()["id"]
        self.account_2 = httpx.post(f"{ACCOUNTS_API_URL}/accounts", json={"client_id": client["id"]}).json()["id"]

    @task
    def post_transaction(self):
        self.client.post(
            "/transactions",
            json={
                "entries": [
                    {"account_id": self.account_1, "direction": "debit", "amount": 100, "reason": "perf"},
                    {"account_id": self.account_2, "direction": "credit", "amount": 100, "reason": "perf"},
                ]
            },
        )
```

- [ ] **Step 2: Run against the real stack, verify it works**

```bash
cd tests/perf
locust -f locustfile_settlement_scoring.py --host=http://localhost:8080 --headless -u 20 -r 5 -t 30s
```

Expected: runs for 30s, prints a summary table with request counts and
latency percentiles, no crashes. Eyeball the failure rate — should be 0%
against a healthy stack.

- [ ] **Step 3: Commit**

```bash
git add tests/perf/
git commit -m "feat(tests): Locust load test for settlement-engine's scoring path"
```

---

### Task 7: `tests/README.md`

**Files:**
- Create: `tests/README.md`

- [ ] **Step 1: Write the README**

Cover: what's in scope here (`api`, `security`, `perf`) vs. explicitly
deferred (`ai-tools`, auth boundary checks, mobile Appium E2E — link to
design spec §8 for why); the exact multi-terminal startup sequence
(`docker compose -f infra/docker/docker-compose.yml up -d` for all 4
Postgres + NATS, then `go run ./cmd/server` in each of
`ledger-service`/`accounts-service`/`settlement-engine`, then
`python main.py` in `notification-service`, each in its own terminal,
setting `SETTLEMENT_BATCH_INTERVAL_SECONDS=5` before starting
settlement-engine so `tests/api/test_end_to_end_flow.py` and
`tests/security` don't wait a full 60s per batch); how to install
(`pip install -r requirements.txt`) and run each suite
(`pytest tests/api`, `pytest tests/security`,
`locust -f tests/perf/locustfile_settlement_scoring.py --host=...`).

- [ ] **Step 2: Commit**

```bash
git add tests/README.md
git commit -m "docs(tests): add README"
```
