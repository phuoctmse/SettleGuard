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
