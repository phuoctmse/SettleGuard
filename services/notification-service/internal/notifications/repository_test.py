import uuid
from datetime import datetime, timedelta, timezone

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


def test_list_returns_most_recent_first():
    with PostgresContainer("postgres:18-alpine") as postgres:
        dsn = postgres.get_connection_url(driver=None) + "?sslmode=disable"
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
        dsn = postgres.get_connection_url(driver=None) + "?sslmode=disable"
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
        dsn = postgres.get_connection_url(driver=None) + "?sslmode=disable"
        migrate(dsn)
        repo, conn = _repo(dsn)
        repo.record("risk_hold", uuid.uuid4(), {"decision": "hold"})

        future_cutoff = datetime.now(timezone.utc) + timedelta(hours=1)
        result = repo.list(since=future_cutoff)

        assert result == []
        conn.close()


def test_list_respects_limit():
    with PostgresContainer("postgres:18-alpine") as postgres:
        dsn = postgres.get_connection_url(driver=None) + "?sslmode=disable"
        migrate(dsn)
        repo, conn = _repo(dsn)
        for _ in range(3):
            repo.record("risk_hold", uuid.uuid4(), {"decision": "hold"})

        result = repo.list(limit=2)

        assert len(result) == 2
        conn.close()
