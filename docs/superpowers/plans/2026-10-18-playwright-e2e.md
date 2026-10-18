# Python Playwright E2E (dự án con 3) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Bộ test E2E bằng Python Playwright chạy trên bản web build của `mobile-app`, đi qua gateway thật, và job CI `e2e-web` upload report, trace, video — theo §7 và §9 của `docs/superpowers/specs/2026-10-18-pentest-and-playwright-design.md`.

**Architecture:** `npx expo export --platform web` với `EXPO_PUBLIC_*_API_URL` trỏ vào gateway (`http://localhost:8000/<prefix>`), serve thư mục build bằng `python -m http.server 8090`. `pytest-playwright` mở Chromium; context được đặt `extra_http_headers={"Authorization": "Bearer <key>"}` nên app không cần sửa và key không nằm trong bundle. Dữ liệu được seed qua API (client HTTP có sẵn trong `tests/api/clients`, cổng trực tiếp); UI chỉ dùng để quan sát và thao tác. Mọi selector là `testID` (render thành `data-testid` trên web), riêng trạng thái lỗi được tìm theo text vì `EmptyState` không có testID.

**Tech Stack:** Python 3.11+ (CI) / 3.14 (local), pytest 8, `pytest-playwright` 0.7–0.10 (Playwright 1.64), `pytest-html` 4, Chromium, Expo 57 web export, GitHub Actions.

## Global Constraints

- Branch: `step/playwright-e2e`, tạo từ `origin/main`. Chạy `git branch --show-current` ngay trước **mỗi** commit.
- Commit message **không** có dòng `Co-Authored-By` hay "Generated with Claude".
- Không sửa `services/*` và `mobile-app/*`. Nếu một test phát hiện lỗi app, báo cáo lại — không sửa app trên branch này.
- Chỉ Chromium. Không Firefox/WebKit, không visual regression, không Appium (spec §1).
- Selector: chỉ dùng `testID` đã có (`client-id-input`, `view-accounts-button`, `account-card-<id>`, `account-balance`, `held-tx-<id>`, `approve-<id>`, `reject-<id>`, `settlement-card-<id>`, `alert-card-<id>`, `tab-accounts`, `tab-held`, `tab-settlements`, `tab-alerts`) qua `page.get_by_test_id(...)`. Ngoại lệ duy nhất: trạng thái lỗi tìm bằng text `"Failed to load accounts."`.
- Key luôn đi qua `extra_http_headers` của context, không bao giờ qua bundle hay URL.
- Web build: `EXPO_PUBLIC_ACCOUNTS_API_URL=http://localhost:8000/accounts`, `EXPO_PUBLIC_LEDGER_API_URL=http://localhost:8000/ledger`, `EXPO_PUBLIC_SETTLEMENT_API_URL=http://localhost:8000/settlement`, `EXPO_PUBLIC_NOTIFICATION_API_URL=http://localhost:8000/notifications`. Biến môi trường của tiến trình được ưu tiên hơn `mobile-app/.env` (đã kiểm chứng: bundle không còn `localhost:8081`).
- Serve trên `http://localhost:8090` — đúng origin gateway cho phép (`CORS_ALLOWED_ORIGINS=http://localhost:8090`).
- `tests/pyproject.toml` giữ `testpaths = ["api", "security"]`: E2E chạy bằng `pytest e2e` tường minh.
- README của suite mới viết **tiếng Việt**; code, tên test, comment: tiếng Anh. `tests/README.md` giữ tiếng Anh.

## Đã kiểm chứng trước khi viết plan (spike 2026-10-18, stack local thật)

1. `extra_http_headers` đi qua CORS + auth của gateway: danh sách account render trong 0,1 s, không request nào lỗi.
2. `fetch` từ `http://localhost:8090` tới gateway kèm key: tới được upstream (không bị CORS chặn).
3. `fetch` y hệt từ `http://127.0.0.1:8090` (cùng server, khác origin): trình duyệt chặn, `TypeError: Failed to fetch`.
4. Key đã revoke: UI hiện `"Failed to load accounts."` sau ~7,5 s (react-query retry 3 lần) — timeout cho trạng thái lỗi phải là 20 s.

## Những điều phải biết (từ final review của dự án con 2)

- Tab Held và Settlements **không poll**; Alerts poll 15 s; screen của tab đã mở thì giữ nguyên. Vì vậy mỗi test **seed dữ liệu trước**, rồi mới mở trang và bấm vào tab lần đầu — lần mở đầu tiên luôn fetch mới. Mỗi test có `page` riêng (context mới) nên không dính trạng thái của test khác.
- `GET /settlements` trả `transaction_ids` cho mỗi batch (sửa ở PR #34), nên tìm được settlement chứa một giao dịch ngay từ danh sách.
- Danh sách Held/Settlements/Notifications là của mọi client, mới nhất trước; bản ghi vừa seed nằm đầu danh sách.
- Approve có thể được settle ngay trong chu kỳ batch 5 s, nên sau approve trạng thái API là `pending_settlement` **hoặc** `settled`.

## File Structure

| File | Trách nhiệm |
|---|---|
| `tests/conftest.py` (sửa) | Thêm fixture gateway dùng chung: `gateway_base_url`, `rate_limited_gateway_url`, `api_key_factory` (chuyển từ `tests/security/conftest.py`) |
| `tests/security/conftest.py` (sửa) | Chỉ còn `seed_blocklist` |
| `tests/e2e/conftest.py` (tạo) | `web_app_url`, `e2e_key`, `browser_context_args`, `wait_until`, `held_tx` |
| `tests/e2e/test_app_shell.py` (tạo) | App tải qua gateway, đủ 4 tab |
| `tests/e2e/test_accounts_flow.py` (tạo) | Client → danh sách account → số dư |
| `tests/e2e/test_held_review.py` (tạo) | Approve và Reject trên UI |
| `tests/e2e/test_settlements_flow.py` (tạo) | Settlement mới hiện trên tab Settlements |
| `tests/e2e/test_alerts_flow.py` (tạo) | Alert của giao dịch bị hold hiện trên tab Alerts |
| `tests/e2e/test_ui_auth.py` (tạo) | Key đã revoke → trạng thái lỗi, không có dữ liệu |
| `tests/e2e/test_browser_cors.py` (tạo) | Trình duyệt thật: origin được phép đi qua, origin lạ bị chặn |
| `tests/requirements.txt` (sửa) | `pytest-playwright`, `pytest-html` |
| `.github/workflows/ci.yml` (sửa) | Job `e2e-web` |
| `tests/e2e/README.md` (tạo), `tests/README.md` (sửa) | Cách chạy |

---

### Task 1: Fixture gateway dùng chung, khung E2E và test app shell

**Files:**
- Modify: `tests/conftest.py`, `tests/security/conftest.py`, `tests/requirements.txt`
- Create: `tests/e2e/conftest.py`, `tests/e2e/test_app_shell.py`

**Interfaces:**
- Produces (dùng ở mọi task sau):
  - `tests/conftest.py`: `gateway_base_url` (session, `GATEWAY_URL`, mặc định `http://localhost:8000`), `rate_limited_gateway_url` (session), `api_key_factory()` → `IssuedKey(client_id, key_id, raw_key)` (function).
  - `tests/e2e/conftest.py`: `web_app_url` (session, `WEB_APP_URL`, mặc định `http://localhost:8090`; skip nếu không tới được), `e2e_key` (session `IssuedKey`), `browser_context_args` (session; thêm `base_url=web_app_url` và header `Authorization`), `wait_until(predicate, timeout=20.0, interval=0.5)` (fixture trả về hàm; hàm trả về giá trị khác `None` đầu tiên của `predicate`, quá hạn thì `pytest.fail`).
  - Vì `browser_context_args` có `base_url`, test dùng `page.goto("/")`.

- [ ] **Step 1: Tạo branch**

```bash
cd D:/SettleGuard
git fetch origin
git checkout -b step/playwright-e2e origin/main
git branch --show-current   # step/playwright-e2e
```

- [ ] **Step 2: Chuyển fixture gateway lên `tests/conftest.py`**

Lý do: `tests/e2e` cần đúng các fixture này (`api_key_factory` cho test key bị revoke, kiểm tra gateway sống), và conftest của một thư mục con không dùng được ở thư mục anh em.

Thêm vào cuối `tests/conftest.py` (giữ nguyên mọi thứ đang có; thêm `import uuid` và hai import client lên đầu file nếu chưa có):

```python
import uuid

from clients.gateway_admin import IssuedKey, create_key

GATEWAY_URL = os.environ.get("GATEWAY_URL", "http://localhost:8000")
GATEWAY_RATE_LIMIT_URL = os.environ.get("GATEWAY_RATE_LIMIT_URL", "http://localhost:8001")


def _require_gateway(base_url: str) -> str:
    try:
        httpx.get(f"{base_url}/health", timeout=2.0).raise_for_status()
    except httpx.HTTPError as exc:
        pytest.skip(f"gateway not reachable at {base_url} ({exc}) -- start it first, see tests/security/README.md")
    return base_url


@pytest.fixture(scope="session")
def gateway_base_url():
    return _require_gateway(GATEWAY_URL)


@pytest.fixture(scope="session")
def rate_limited_gateway_url():
    return _require_gateway(GATEWAY_RATE_LIMIT_URL)


@pytest.fixture
def api_key_factory(accounts_base_url):
    """Each call creates a fresh client business (accounts-service, direct
    port -- setup only) and issues a key for it with the real adminctl."""
    accounts = AccountsClient(accounts_base_url)

    def _issue() -> IssuedKey:
        client_id = accounts.create_client(f"Pentest {uuid.uuid4()}").json()["id"]
        return create_key(client_id, "pentest")

    return _issue
```

Xoá khỏi `tests/security/conftest.py` đúng các khối đã chuyển: `GATEWAY_URL`, `GATEWAY_RATE_LIMIT_URL`, `_require_gateway`, `gateway_base_url`, `rate_limited_gateway_url`, `api_key_factory`, và các import chỉ chúng dùng (`httpx`, `AccountsClient`, `IssuedKey`, `create_key`). Giữ nguyên `seed_blocklist` cùng import của nó (`os`, `uuid`, `psycopg2`, `pytest`, `SETTLEMENT_DATABASE_URL`).

- [ ] **Step 3: Xác nhận suite security không đổi hành vi**

Stack phải đang chạy (4 service + 2 gateway, xem `tests/security/README.md`).

```bash
cd D:/SettleGuard/tests
export GATEWAY_ADMINCTL=<đường dẫn adminctl đã build, nếu có>
python -m pytest security -q -rxX
```

Expected: cùng kết quả như trước khi chuyển — `76 passed, 1 deselected, 5 xfailed` (SG-PT-01/02/03/06/08). Nếu `test_gateway_rate_limit.py` fail vì bucket `:8001` đã cạn, chờ 60 s rồi chạy lại riêng file đó.

- [ ] **Step 4: Thêm dependency và cài Chromium**

`tests/requirements.txt` thêm hai dòng:

```
pytest-playwright>=0.7,<0.11
pytest-html>=4.1,<5
```

```bash
pip install -r tests/requirements.txt
python -m playwright install chromium
```

- [ ] **Step 5: Build và serve bản web (local)**

Terminal riêng, từ repo root:

```bash
cd mobile-app
EXPO_PUBLIC_ACCOUNTS_API_URL=http://localhost:8000/accounts \
EXPO_PUBLIC_LEDGER_API_URL=http://localhost:8000/ledger \
EXPO_PUBLIC_SETTLEMENT_API_URL=http://localhost:8000/settlement \
EXPO_PUBLIC_NOTIFICATION_API_URL=http://localhost:8000/notifications \
  npx expo export --platform web --output-dir ../.e2e-web
cd ..
python -m http.server 8090 -d .e2e-web
```

Thêm `.e2e-web/` vào `.gitignore` (dòng mới cuối file). Kiểm tra bundle trỏ vào gateway: `grep -l "localhost:8000/accounts" .e2e-web/_expo/static/js/web/*.js` có kết quả.

- [ ] **Step 6: Viết test thất bại**

`tests/e2e/test_app_shell.py`:

```python
"""The web build loads through the gateway with the injected key."""
from playwright.sync_api import Page, expect

TABS = ["tab-accounts", "tab-held", "tab-settlements", "tab-alerts"]


def test_app_shell_renders_every_tab(page: Page):
    page.goto("/")
    for tab in TABS:
        expect(page.get_by_test_id(tab)).to_be_visible()
    expect(page.get_by_test_id("client-id-input")).to_be_visible()
```

Run: `cd tests && python -m pytest e2e -v`
Expected: FAIL vì chưa có `base_url` — `page.goto("/")` không điều hướng được tới một URL tương đối (lỗi của Playwright về URL không hợp lệ). Ghi nguyên văn thông báo lỗi vào báo cáo.

- [ ] **Step 7: Viết `tests/e2e/conftest.py`**

```python
"""Fixtures for the Playwright suite. The app under test is the Expo web
build served on WEB_APP_URL, pointed at the gateway; every browser context
carries an API key as an extra header, so no key ever ships in the bundle
(design spec §7.1)."""
import os
import time
import uuid

import httpx
import pytest

from clients.accounts import AccountsClient
from clients.gateway_admin import IssuedKey, create_key

WEB_APP_URL = os.environ.get("WEB_APP_URL", "http://localhost:8090")


@pytest.fixture(scope="session")
def web_app_url(gateway_base_url):
    try:
        httpx.get(WEB_APP_URL, timeout=2.0).raise_for_status()
    except httpx.HTTPError as exc:
        pytest.skip(f"web build not served at {WEB_APP_URL} ({exc}) -- see tests/e2e/README.md")
    return WEB_APP_URL


@pytest.fixture(scope="session")
def e2e_key(accounts_base_url) -> IssuedKey:
    client_id = AccountsClient(accounts_base_url).create_client(f"E2E {uuid.uuid4()}").json()["id"]
    return create_key(client_id, "e2e")


@pytest.fixture(scope="session")
def browser_context_args(browser_context_args, web_app_url, e2e_key):
    return {
        **browser_context_args,
        "base_url": web_app_url,
        "extra_http_headers": {"Authorization": f"Bearer {e2e_key.raw_key}"},
    }


def _wait_until(predicate, timeout=20.0, interval=0.5):
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        result = predicate()
        if result is not None:
            return result
        time.sleep(interval)
    pytest.fail(f"condition not met within {timeout}s")


@pytest.fixture
def wait_until():
    return _wait_until
```

- [ ] **Step 8: Chạy lại**

Run: `cd tests && python -m pytest e2e -v --browser chromium`
Expected: `test_app_shell_renders_every_tab PASSED`.

Thử nhánh skip: dừng `http.server` rồi chạy lại → `SKIPPED (web build not served at ...)`. Khởi động lại nó.

- [ ] **Step 9: Commit**

```bash
git branch --show-current   # step/playwright-e2e
git add tests/conftest.py tests/security/conftest.py tests/requirements.txt tests/e2e/conftest.py tests/e2e/test_app_shell.py .gitignore
git commit -m "tests(e2e): Playwright scaffold on the web build, shared gateway fixtures"
```

---

### Task 2: Luồng Accounts

**Files:**
- Create: `tests/e2e/test_accounts_flow.py`

**Interfaces:**
- Consumes: `page` (pytest-playwright, đã có `base_url` + key), `accounts`, `ledger` (`tests/conftest.py`), `wait_until` (Task 1).

Số dư được accounts-service cập nhật **bất đồng bộ** từ event của ledger, nên test chờ qua API cho tới khi số dư đúng rồi mới mở UI — nếu không, UI có thể hiện số dư cũ và test chập chờn.

- [ ] **Step 1: Viết test**

```python
"""Client lookup -> account list -> account detail, through the gateway."""
import uuid

from playwright.sync_api import Page, expect


def test_account_list_and_balance_match_the_api(page: Page, accounts, ledger, wait_until):
    client_id = accounts.create_client(f"E2E accounts {uuid.uuid4()}").json()["id"]
    debit = accounts.create_account(client_id).json()["id"]
    credit = accounts.create_account(client_id).json()["id"]
    resp = ledger.post_transaction([
        {"account_id": debit, "direction": "debit", "amount": 100, "reason": "e2e"},
        {"account_id": credit, "direction": "credit", "amount": 100, "reason": "e2e"},
    ])
    assert resp.status_code == 201, resp.text
    # Balances are projected asynchronously from ledger events.
    wait_until(lambda: True if accounts.get_account(debit).json()["balance"] == -100 else None)

    page.goto("/")
    page.get_by_test_id("client-id-input").fill(client_id)
    page.get_by_test_id("view-accounts-button").click()

    expect(page.get_by_test_id(f"account-card-{credit}")).to_be_visible()
    page.get_by_test_id(f"account-card-{debit}").click()
    expect(page.get_by_test_id("account-balance")).to_have_text("Balance: -100")
```

- [ ] **Step 2: Chạy và xác nhận nó kiểm tra thật**

Run: `cd tests && python -m pytest e2e/test_accounts_flow.py -v --browser chromium`
Expected: PASS.

Chứng minh test không pass suông: tạm đổi `"Balance: -100"` thành `"Balance: -101"`, chạy lại → FAIL với `Expected string: "Balance: -101" Received string: "Balance: -100"`. Hoàn tác thay đổi.

- [ ] **Step 3: Commit**

```bash
git branch --show-current   # step/playwright-e2e
git add tests/e2e/test_accounts_flow.py
git commit -m "tests(e2e): accounts flow -- lookup, list, balance"
```

---

### Task 3: Duyệt giao dịch bị hold (approve, reject)

**Files:**
- Modify: `tests/e2e/conftest.py` (thêm fixture `held_tx`)
- Create: `tests/e2e/test_held_review.py`

**Interfaces:**
- Consumes: `accounts`, `ledger`, `settlement` (`tests/conftest.py`), `wait_until` (Task 1).
- Produces: fixture `held_tx()` → `str` (transaction id của một giao dịch mới đang ở `held`). Task 4 dùng nó.

20.000.000 vượt `SETTLEMENT_MISMATCH_THRESHOLD` mặc định (10.000.000), nên giao dịch luôn bị hold.

- [ ] **Step 1: Thêm `held_tx` vào `tests/e2e/conftest.py`**

```python
HOLD_AMOUNT = 20_000_000  # over SETTLEMENT_MISMATCH_THRESHOLD (default 10,000,000)


@pytest.fixture
def held_tx(accounts, ledger, settlement, wait_until):
    """Returns a factory: each call posts a fresh over-threshold transaction
    and waits until settlement-engine has scored it as held."""

    def _make() -> str:
        client_id = accounts.create_client(f"E2E held {uuid.uuid4()}").json()["id"]
        debit = accounts.create_account(client_id).json()["id"]
        credit = accounts.create_account(client_id).json()["id"]
        resp = ledger.post_transaction([
            {"account_id": debit, "direction": "debit", "amount": HOLD_AMOUNT, "reason": "e2e-hold"},
            {"account_id": credit, "direction": "credit", "amount": HOLD_AMOUNT, "reason": "e2e-hold"},
        ])
        if resp.status_code != 201:
            pytest.fail(f"seeding the held transaction failed: {resp.status_code} {resp.text}")
        tx_id = resp.json()[0]["transaction_id"]

        def is_held():
            r = settlement.get_transaction(tx_id)
            return True if r.status_code == 200 and r.json()["status"] == "held" else None

        wait_until(is_held)
        return tx_id

    return _make
```

- [ ] **Step 2: Viết test**

`tests/e2e/test_held_review.py`:

```python
"""Approve and reject a held transaction from the Held tab (SETTLEMENT-05)."""
from playwright.sync_api import Page, expect


def _open_held_tab(page: Page, tx_id: str):
    page.goto("/")  # seeded before the first visit, so the tab fetches it
    page.get_by_test_id("tab-held").click()
    expect(page.get_by_test_id(f"held-tx-{tx_id}")).to_be_visible()


def test_approve_removes_the_row_and_releases_the_transaction(page: Page, held_tx, settlement):
    tx_id = held_tx()
    _open_held_tab(page, tx_id)

    page.get_by_test_id(f"approve-{tx_id}").click()

    expect(page.get_by_test_id(f"held-tx-{tx_id}")).to_have_count(0)
    # The 5 s batch may already have settled it.
    assert settlement.get_transaction(tx_id).json()["status"] in ("pending_settlement", "settled")


def test_reject_removes_the_row_and_marks_the_transaction_rejected(page: Page, held_tx, settlement):
    tx_id = held_tx()
    _open_held_tab(page, tx_id)

    page.get_by_test_id(f"reject-{tx_id}").click()

    expect(page.get_by_test_id(f"held-tx-{tx_id}")).to_have_count(0)
    assert settlement.get_transaction(tx_id).json()["status"] == "rejected"
```

- [ ] **Step 3: Chạy**

Run: `cd tests && python -m pytest e2e/test_held_review.py -v --browser chromium`
Expected: 2 PASS. Settlement-engine phải chạy với `SETTLEMENT_BATCH_INTERVAL_SECONDS=5`.

Chứng minh test reject không pass suông: tạm đổi `"rejected"` thành `"held"`, chạy lại → FAIL ở assert trạng thái. Hoàn tác.

- [ ] **Step 4: Commit**

```bash
git branch --show-current   # step/playwright-e2e
git add tests/e2e/conftest.py tests/e2e/test_held_review.py
git commit -m "tests(e2e): approve and reject a held transaction from the UI"
```

---

### Task 4: Settlements và Alerts

**Files:**
- Create: `tests/e2e/test_settlements_flow.py`, `tests/e2e/test_alerts_flow.py`

**Interfaces:**
- Consumes: `held_tx()` (Task 3), `settlement`, `notifications` (`tests/conftest.py`), `wait_until` (Task 1).

Thao tác approve ở đây làm qua API (UI đã được test ở Task 3); test này kiểm tra **hiển thị** của settlement và alert.

- [ ] **Step 1: Viết test settlement**

`tests/e2e/test_settlements_flow.py`:

```python
"""An approved transaction lands in a settlement that the Settlements tab shows."""
from playwright.sync_api import Page, expect


def test_settlement_containing_the_transaction_is_listed(page: Page, held_tx, settlement, wait_until):
    tx_id = held_tx()
    resp = settlement.approve(tx_id)
    assert resp.status_code == 200, resp.text

    def find_settlement():
        for s in settlement.list_settlements().json():
            if tx_id in s["transaction_ids"]:
                return s["id"]
        return None

    settlement_id = wait_until(find_settlement)  # one 5 s batch cycle

    page.goto("/")
    page.get_by_test_id("tab-settlements").click()  # first visit: fresh fetch
    expect(page.get_by_test_id(f"settlement-card-{settlement_id}")).to_be_visible()
```

- [ ] **Step 2: Viết test alert**

`tests/e2e/test_alerts_flow.py`:

```python
"""A held transaction raises a risk_hold alert that the Alerts tab shows."""
from playwright.sync_api import Page, expect


def test_risk_hold_alert_is_listed(page: Page, held_tx, notifications, wait_until):
    tx_id = held_tx()

    def find_notification():
        for n in notifications.list_notifications(type_="risk_hold", limit=200).json():
            if n["subject_id"] == tx_id:
                return n["id"]
        return None

    notification_id = wait_until(find_notification)

    page.goto("/")
    page.get_by_test_id("tab-alerts").click()  # first visit: fresh fetch
    expect(page.get_by_test_id(f"alert-card-{notification_id}")).to_be_visible()
```

- [ ] **Step 3: Chạy**

Run: `cd tests && python -m pytest e2e/test_settlements_flow.py e2e/test_alerts_flow.py -v --browser chromium`
Expected: 2 PASS.

Chứng minh không pass suông: tạm đổi id trong `expect` của test alert thành `f"alert-card-{notification_id}x"`, chạy lại → FAIL (`element(s) not found`). Hoàn tác.

Nếu `/notifications` không trả lời (curl `http://localhost:8083/notifications?limit=1` khác 200): notification-service có thể đã bị test `destructive` (SG-PT-07) làm hỏng — khởi động lại service rồi chạy lại. Không coi đó là lỗi của test.

- [ ] **Step 4: Commit**

```bash
git branch --show-current   # step/playwright-e2e
git add tests/e2e/test_settlements_flow.py tests/e2e/test_alerts_flow.py
git commit -m "tests(e2e): settlements and alerts tabs show what the backend produced"
```

---

### Task 5: Key bị revoke và CORS trong trình duyệt thật

**Files:**
- Create: `tests/e2e/test_ui_auth.py`, `tests/e2e/test_browser_cors.py`

**Interfaces:**
- Consumes: `browser` (pytest-playwright), `web_app_url`, `e2e_key` (Task 1), `api_key_factory` (`tests/conftest.py`, Task 1), `clients.gateway_admin.revoke_key`.

**Lệch khỏi spec §7.2, có lý do:** origin lạ là `http://127.0.0.1:8090` (cùng bản build, khác origin) thay vì một trang `http://evil.test` dựng bằng `page.route`. Origin `127.0.0.1` là một trang thật được serve, nên không có ống nối giả, và cả hai origin đều là địa chỉ nội bộ nên chính sách Private Network Access của Chrome không thể là lý do bị chặn — chỉ còn CORS. Spike đã xác nhận: `TypeError: Failed to fetch`.

- [ ] **Step 1: Viết test key bị revoke**

`tests/e2e/test_ui_auth.py`:

```python
"""A revoked key shows the error state and no data -- never a crash, never rows."""
import re

from playwright.sync_api import Browser, expect

from clients.gateway_admin import revoke_key


def test_revoked_key_shows_the_error_state(browser: Browser, web_app_url, api_key_factory):
    key = api_key_factory()
    revoke_key(key.key_id)
    context = browser.new_context(
        base_url=web_app_url,
        extra_http_headers={"Authorization": f"Bearer {key.raw_key}"},
    )
    page = context.new_page()
    try:
        page.goto("/")
        page.get_by_test_id("client-id-input").fill(key.client_id)
        page.get_by_test_id("view-accounts-button").click()

        # react-query retries three times before showing the error (~7.5 s).
        expect(page.get_by_text("Failed to load accounts.")).to_be_visible(timeout=20_000)
        expect(page.get_by_test_id(re.compile(r"^account-card-"))).to_have_count(0)
    finally:
        context.close()
```

- [ ] **Step 2: Viết test CORS**

`tests/e2e/test_browser_cors.py`:

```python
"""A real browser enforces the gateway's CORS allowlist (spec §7.2)."""
from playwright.sync_api import Browser, Page

FETCH = """async ([url, key]) => {
  try {
    const r = await fetch(url, { headers: { Authorization: 'Bearer ' + key } });
    return 'status ' + r.status;
  } catch (e) {
    return 'blocked: ' + e.name;
  }
}"""


def _client_url(e2e_key) -> str:
    return f"http://localhost:8000/accounts/clients/{e2e_key.client_id}"


def test_allowed_origin_can_call_the_gateway(page: Page, e2e_key):
    page.goto("/")  # http://localhost:8090 -- the allowlisted origin
    assert page.evaluate(FETCH, [_client_url(e2e_key), e2e_key.raw_key]) == "status 200"


def test_foreign_origin_is_blocked_by_the_browser(browser: Browser, e2e_key, web_app_url):
    # Same build, different origin string: 127.0.0.1 is not in CORS_ALLOWED_ORIGINS.
    foreign = web_app_url.replace("localhost", "127.0.0.1")
    context = browser.new_context()
    page = context.new_page()
    try:
        page.goto(foreign)
        assert page.evaluate(FETCH, [_client_url(e2e_key), e2e_key.raw_key]) == "blocked: TypeError"
    finally:
        context.close()
```

- [ ] **Step 3: Chạy**

Run: `cd tests && python -m pytest e2e/test_ui_auth.py e2e/test_browser_cors.py -v --browser chromium`
Expected: 3 PASS.

Chứng minh test CORS phụ thuộc đúng cấu hình: dừng gateway `:8000`, khởi động lại với `CORS_ALLOWED_ORIGINS=http://localhost:8090,http://127.0.0.1:8090`, chạy lại `test_foreign_origin_is_blocked_by_the_browser` → FAIL (`'status 200' == 'blocked: TypeError'`). Khởi động lại gateway với cấu hình gốc (`CORS_ALLOWED_ORIGINS=http://localhost:8090`) và xác nhận test PASS trở lại.

- [ ] **Step 4: Chạy cả suite**

Run: `cd tests && python -m pytest e2e -v --browser chromium`
Expected: 9 PASS (app shell 1, accounts 1, held 2, settlements 1, alerts 1, ui_auth 1, cors 2).

- [ ] **Step 5: Commit**

```bash
git branch --show-current   # step/playwright-e2e
git add tests/e2e/test_ui_auth.py tests/e2e/test_browser_cors.py
git commit -m "tests(e2e): revoked key shows the error state; the browser enforces CORS"
```

---

### Task 6: Job CI `e2e-web`

**Files:**
- Modify: `.github/workflows/ci.yml`

**Interfaces:**
- Consumes: composite action `./.github/actions/start-stack` (đã có: dựng 4 service + 2 gateway với `CORS_ALLOWED_ORIGINS=http://localhost:8090`, cài `tests/requirements.txt`, export `GATEWAY_ADMINCTL`).

Job riêng, không nằm sau bước `destructive` của job `security` (spec §9): bước đó làm hỏng `/notifications`, mà test Alerts cần nó.

- [ ] **Step 1: Thêm job vào cuối `.github/workflows/ci.yml`**

```yaml
  e2e-web:
    name: e2e-web (Playwright on the web build)
    runs-on: ubuntu-latest
    timeout-minutes: 30
    env:
      NATS_URL: nats://localhost:4222
      LOG_DIR: ${{ github.workspace }}/e2e-logs
    steps:
      - uses: actions/checkout@v5
      - uses: ./.github/actions/start-stack
      - uses: actions/setup-node@v5
        with:
          node-version: "24"
          cache: npm
          cache-dependency-path: mobile-app/package-lock.json
      - name: Build the web app against the gateway
        working-directory: mobile-app
        env:
          EXPO_PUBLIC_ACCOUNTS_API_URL: http://localhost:8000/accounts
          EXPO_PUBLIC_LEDGER_API_URL: http://localhost:8000/ledger
          EXPO_PUBLIC_SETTLEMENT_API_URL: http://localhost:8000/settlement
          EXPO_PUBLIC_NOTIFICATION_API_URL: http://localhost:8000/notifications
        run: |
          npm ci
          npx expo export --platform web --output-dir "$RUNNER_TEMP/web"
      - name: Serve the web app on :8090
        run: |
          mkdir -p "$LOG_DIR"
          nohup python -m http.server 8090 -d "$RUNNER_TEMP/web" > "$LOG_DIR/web.log" 2>&1 &
          for i in $(seq 1 30); do
            curl -fsS -o /dev/null http://localhost:8090/ && exit 0
            sleep 1
          done
          echo "web app never served"; exit 1
      - name: Install Chromium
        run: python -m playwright install --with-deps chromium
      - name: pytest tests/e2e
        working-directory: tests
        run: >
          python -m pytest e2e -v --browser chromium
          --tracing retain-on-failure --video retain-on-failure --screenshot only-on-failure
          --output test-results --html=e2e-report.html --self-contained-html
      - name: Upload E2E report, traces and videos
        if: always()
        uses: actions/upload-artifact@v5
        with:
          name: e2e-web-report
          path: |
            tests/e2e-report.html
            tests/test-results/
          if-no-files-found: warn
      - name: Collect infra logs
        if: failure()
        run: |
          mkdir -p "$LOG_DIR"
          docker compose -f infra/docker/docker-compose.yml logs --no-color > "$LOG_DIR/docker-compose.log" 2>&1 || true
      - name: Upload service logs
        if: failure()
        uses: actions/upload-artifact@v5
        with:
          name: e2e-web-service-logs
          path: e2e-logs/
          if-no-files-found: warn
```

- [ ] **Step 2: Kiểm tra cú pháp và lệnh pytest local**

```bash
python -c "import yaml; yaml.safe_load(open('.github/workflows/ci.yml')); print('ok')"
cd tests && python -m pytest e2e -v --browser chromium --tracing retain-on-failure --video retain-on-failure --screenshot only-on-failure --output test-results --html=e2e-report.html --self-contained-html
```

Expected: `ok`; toàn bộ test PASS; `tests/e2e-report.html` được tạo.

Chứng minh trace/video được giữ khi fail (không commit thay đổi này): tạm đổi `"Balance: -100"` thành `"Balance: -101"` trong `test_accounts_flow.py`, chạy lại đúng lệnh trên chỉ với file đó → FAIL, và `tests/test-results/<tên test>/` có `trace.zip` và một file `.webm`. Hoàn tác, xoá `tests/test-results/` và `tests/e2e-report.html`.

Thêm vào `.gitignore`: `tests/test-results/` và `tests/e2e-report.html`.

- [ ] **Step 3: Commit**

```bash
git branch --show-current   # step/playwright-e2e
git add .github/workflows/ci.yml .gitignore
git commit -m "ci: e2e-web job runs the Playwright suite on the web build"
```

---

### Task 7: Tài liệu

**Files:**
- Create: `tests/e2e/README.md`
- Modify: `tests/README.md`

- [ ] **Step 1: Viết `tests/e2e/README.md`** (tiếng Việt)

```markdown
# tests/e2e

Bộ E2E bằng Python Playwright chạy trên **bản web build** của `mobile-app`,
đi qua gateway thật. Thiết kế: `docs/superpowers/specs/2026-10-18-pentest-and-playwright-design.md` §7.

## Cách nó hoạt động

- Bản web được build với các `EXPO_PUBLIC_*_API_URL` trỏ vào gateway
  (`http://localhost:8000/<prefix>`) và serve ở `http://localhost:8090` —
  đúng origin mà gateway cho phép CORS.
- App không gửi API key. Playwright gắn key vào mọi request bằng
  `extra_http_headers` của browser context, nên key không bao giờ nằm trong
  bundle. Key được cấp bằng `adminctl` thật lúc bắt đầu session.
- Dữ liệu được seed qua API (cổng trực tiếp của service); UI chỉ dùng để xem
  và thao tác. Selector là `testID` (trên web là `data-testid`), xem bảng
  trong `mobile-app/README.md`.

## Chạy local

Cần stack như `tests/README.md` (4 service, NATS, Postgres) và gateway `:8000`
khởi động với `CORS_ALLOWED_ORIGINS=http://localhost:8090` — để trống thì
gateway chặn mọi origin và mọi test sẽ thấy trạng thái lỗi.

    pip install -r tests/requirements.txt
    python -m playwright install chromium

Build và serve bản web (từ repo root, terminal riêng):

    cd mobile-app
    EXPO_PUBLIC_ACCOUNTS_API_URL=http://localhost:8000/accounts \
    EXPO_PUBLIC_LEDGER_API_URL=http://localhost:8000/ledger \
    EXPO_PUBLIC_SETTLEMENT_API_URL=http://localhost:8000/settlement \
    EXPO_PUBLIC_NOTIFICATION_API_URL=http://localhost:8000/notifications \
      npx expo export --platform web --output-dir ../.e2e-web
    cd .. && python -m http.server 8090 -d .e2e-web

Biến môi trường ở trên ghi đè `mobile-app/.env`. Build lại mỗi khi code app đổi.

Chạy test (từ `tests/`):

    python -m pytest e2e -v --browser chromium
    python -m pytest e2e -v --browser chromium --headed      # xem trình duyệt
    python -m pytest e2e --tracing on                        # luôn giữ trace

Xem trace: `python -m playwright show-trace test-results/<tên test>/trace.zip`.

## Đọc kết quả

- **SKIPPED**: gateway hoặc bản web không chạy.
- Trạng thái lỗi của app (`"Failed to load …"`) chỉ hiện sau ~7,5 s vì
  react-query thử lại 3 lần; test về key bị revoke chờ tới 20 s.
- Tab Held và Settlements không tự tải lại: mỗi test seed dữ liệu **trước**
  rồi mới mở tab lần đầu. Nếu tự viết test mới, giữ đúng thứ tự đó hoặc
  `page.reload()`.
- `/notifications` không trả lời → có thể test `destructive` của
  `tests/security` (SG-PT-07) vừa chạy; khởi động lại notification-service.

## CI

Job `e2e-web` trong `.github/workflows/ci.yml` dựng stack, build và serve bản
web, chạy suite, rồi upload artifact `e2e-web-report`: `e2e-report.html`
(pytest-html) và `test-results/` (trace, video, screenshot của test fail). Mở
trace đã tải về bằng `python -m playwright show-trace <file>` hoặc
https://trace.playwright.dev.

Playwright cho Python không có HTML reporter như bản Node; `pytest-html` cùng
trace là phần thay thế.
```

- [ ] **Step 2: Sửa `tests/README.md`**

- Mục Scope: thêm gạch đầu dòng `tests/e2e` — "Python Playwright E2E against the `mobile-app` web build, through the gateway — see `tests/e2e/README.md`."
- Mục "Running the suites": câu về CI thêm "and `tests/e2e` in the `e2e-web` job".
- Mục "Explicitly deferred", gạch đầu dòng **Mobile Appium E2E**: thay lý do bằng "`testID`s now exist (`mobile-app/README.md`), so this is no longer blocked — just not written yet. On Android, Appium locates them by `id` (`resource-id`), on iOS by `accessibility id`."
- Mục Prerequisites: thêm "Node 24 + npm (to build the web app) and Chromium via `python -m playwright install chromium`".

- [ ] **Step 3: Commit**

```bash
git branch --show-current   # step/playwright-e2e
git add tests/e2e/README.md tests/README.md
git commit -m "docs(tests): how to run the Playwright E2E suite"
```

---

## Self-Review

**Spec coverage (§7, §8(3), §9):**

| Yêu cầu | Task |
|---|---|
| §7.1 web build trỏ gateway, key qua `extra_http_headers` | 1 (đã kiểm chứng bằng spike) |
| §7.2 `test_accounts_flow` | 2 |
| §7.2 `test_held_review` (approve + reject) | 3 |
| §7.2 `test_settlements_flow`, `test_alerts_flow` | 4 |
| §7.2 `test_ui_auth`, `test_browser_cors` | 5 |
| §7.3 CI: export, `http.server`, `--tracing/--video retain-on-failure`, `pytest-html`, artifact | 6 |
| §7.3 `testpaths` giữ nguyên, `pytest e2e` tường minh | Global Constraints |
| §7.3 requirements thêm `pytest-playwright`, `pytest-html` | 1 |
| §8(3) cố ý làm fail để xác nhận trace/video | 6 Step 2 |
| §8 `tests/README.md` thêm e2e, đổi lý do hoãn Appium | 7 |
| §9 job riêng, không sau bước destructive; dùng lại helper key và composite action | 1, 6 |

**Lệch khỏi spec, có lý do:**
- `test_browser_cors` dùng origin `127.0.0.1:8090` thay vì `page.route` dựng `evil.test` (lý do ở Task 5).
- Trạng thái lỗi được tìm theo text (`EmptyState` không có testID; thêm testID là sửa `mobile-app`, ngoài phạm vi branch này).
- Có thêm `test_app_shell.py`: là test đầu tiên chứng minh build + serve + key chạy, trước khi viết các luồng.
- Fixture gateway chuyển từ `tests/security/conftest.py` lên `tests/conftest.py` để `tests/e2e` dùng chung.
