import os
import uuid

import httpx
import psycopg2
import pytest

from clients.accounts import AccountsClient
from clients.gateway_admin import IssuedKey, create_key

SETTLEMENT_DATABASE_URL = os.environ.get(
    "SETTLEMENT_DATABASE_URL",
    "postgres://settlement:settlement@localhost:5435/settlement?sslmode=disable",
)


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


@pytest.fixture
def seed_blocklist():
    """Yields a function that inserts a `blocklist` row for a given
    (already-existing, e.g. from accounts-service) account id. Rows
    inserted during the test are deleted afterward. Direct-DB is a
    deliberate exception here -- settlement-engine has no HTTP endpoint to
    manage the blocklist (see plan Global Constraints)."""
    conn = psycopg2.connect(SETTLEMENT_DATABASE_URL)
    inserted_ids = []

    try:
        def _seed(account_id: str):
            with conn:
                with conn.cursor() as cur:
                    cur.execute(
                        "INSERT INTO blocklist (id, entity_type, entity_id, reason) VALUES (%s, 'account', %s, %s)",
                        (str(uuid.uuid4()), account_id, "fraud-bypass test"),
                    )
            inserted_ids.append(account_id)

        yield _seed

        with conn:
            with conn.cursor() as cur:
                cur.executemany(
                    "DELETE FROM blocklist WHERE entity_id = %s AND entity_type = 'account'",
                    [(aid,) for aid in inserted_ids],
                )
    finally:
        conn.close()
