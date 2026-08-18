import uuid

import pytest
from testcontainers.postgres import PostgresContainer

from internal.db.connection import connect, migrate
from internal.notifications.repository import NotificationRepository


def _repo(dsn):
    conn = connect(dsn)
    return NotificationRepository(conn), conn


def test_record_inserts_new_notification():
    with PostgresContainer("postgres:18-alpine") as postgres:
        dsn = postgres.get_connection_url(driver=None) + "?sslmode=disable"
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
        dsn = postgres.get_connection_url(driver=None) + "?sslmode=disable"
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


def test_record_rolls_back_on_error_so_connection_stays_usable():
    with PostgresContainer("postgres:18-alpine") as postgres:
        dsn = postgres.get_connection_url(driver=None) + "?sslmode=disable"
        migrate(dsn)
        repo, conn = _repo(dsn)
        tx_id = uuid.uuid4()

        # "not_a_valid_type" violates the CHECK constraint on notifications.type,
        # which aborts the transaction on a non-autocommit connection.
        with pytest.raises(Exception):
            repo.record("not_a_valid_type", tx_id, {"decision": "hold"})

        # If record() had not rolled back, this call would raise
        # InFailedSqlTransaction instead of succeeding.
        other_tx_id = uuid.uuid4()
        inserted = repo.record("risk_hold", other_tx_id, {"decision": "hold"})

        assert inserted is True
        conn.close()
