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
