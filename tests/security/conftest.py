import os
import sys
import uuid
from pathlib import Path

import psycopg2
import pytest

sys.path.insert(0, str(Path(__file__).parent.parent / "api"))

pytest_plugins = ["conftest"]

SETTLEMENT_DATABASE_URL = os.environ.get(
    "SETTLEMENT_DATABASE_URL",
    "postgres://settlement:settlement@localhost:5435/settlement?sslmode=disable",
)


@pytest.fixture
def seed_blocklist():
    """Yields a function that inserts a `blocklist` row for a given
    (already-existing, e.g. from accounts-service) account id. Rows
    inserted during the test are deleted afterward. Direct-DB is a
    deliberate exception here -- settlement-engine has no HTTP endpoint to
    manage the blocklist (see plan Global Constraints)."""
    conn = psycopg2.connect(SETTLEMENT_DATABASE_URL)
    inserted_ids = []

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
                "DELETE FROM blocklist WHERE entity_id = %s",
                [(aid,) for aid in inserted_ids],
            )
    conn.close()
