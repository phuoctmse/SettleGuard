import os
import uuid

import httpx
import pytest

from clients.accounts import AccountsClient
from clients.gateway_admin import IssuedKey, create_key
from clients.ledger import LedgerClient
from clients.notifications import NotificationsClient
from clients.settlement import SettlementClient

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
