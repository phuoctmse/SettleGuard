import uuid

from internal.consumer.consumer import decide_risk_scored, decide_settlement_finalized


def test_decide_risk_scored_returns_none_when_pass():
    payload = {"transaction_id": str(uuid.uuid4()), "decision": "pass"}

    assert decide_risk_scored(payload) is None


def test_decide_risk_scored_returns_record_when_hold():
    tx_id = str(uuid.uuid4())
    payload = {"transaction_id": tx_id, "decision": "hold", "score": 40}

    record = decide_risk_scored(payload)

    assert record == {"type": "risk_hold", "subject_id": uuid.UUID(tx_id), "payload": payload}


def test_decide_settlement_finalized_always_returns_record():
    settlement_id = str(uuid.uuid4())
    payload = {"settlement_id": settlement_id, "transaction_count": 3}

    record = decide_settlement_finalized(payload)

    assert record == {"type": "settlement_finalized", "subject_id": uuid.UUID(settlement_id), "payload": payload}


class _FakeMsg:
    def __init__(self, data: bytes, subject: str = "transaction.risk-scored"):
        self.data = data
        self.subject = subject
        self.acked = False
        self.naked = False
        self.termed = False

    async def ack(self):
        self.acked = True

    async def nak(self):
        self.naked = True

    async def term(self):
        self.termed = True


class _FakeRepo:
    def __init__(self):
        self.calls = []

    def record(self, type_, subject_id, payload):
        self.calls.append((type_, subject_id, payload))
        return True


async def test_handle_terminates_message_on_schema_malformed_payload():
    import json

    from internal.consumer.consumer import Consumer

    repo = _FakeRepo()
    consumer = Consumer(repo)
    # valid JSON, but missing "transaction_id" -> KeyError inside decide_risk_scored
    msg = _FakeMsg(json.dumps({"decision": "hold", "score": 40}).encode())

    await consumer._handle_risk_scored(msg)

    assert msg.termed is True
    assert msg.acked is False
    assert msg.naked is False
    assert repo.calls == []


async def test_handle_terminates_message_on_non_uuid_id_field():
    import json

    from internal.consumer.consumer import Consumer

    repo = _FakeRepo()
    consumer = Consumer(repo)
    # valid JSON, well-formed keys, but "settlement_id" is not a valid UUID -> ValueError
    msg = _FakeMsg(
        json.dumps({"settlement_id": "not-a-uuid", "transaction_count": 3}).encode(),
        subject="settlement.finalized",
    )

    await consumer._handle_settlement_finalized(msg)

    assert msg.termed is True
    assert msg.acked is False
    assert msg.naked is False
    assert repo.calls == []


import asyncio
import json

import pytest
from testcontainers.core.container import DockerContainer
from testcontainers.core.wait_strategies import LogMessageWaitStrategy
from testcontainers.postgres import PostgresContainer

from internal.broker.connection import connect as nats_connect, ensure_stream
from internal.consumer.consumer import Consumer
from internal.db.connection import connect as db_connect, migrate
from internal.notifications.repository import NotificationRepository


@pytest.fixture
def nats_url():
    container = (
        DockerContainer("nats:2.10-alpine")
        .with_command(["-js"])
        .with_exposed_ports(4222)
        .waiting_for(LogMessageWaitStrategy("Server is ready"))
    )
    container.start()
    try:
        host = container.get_container_host_ip()
        port = container.get_exposed_port(4222)
        yield f"nats://{host}:{port}"
    finally:
        container.stop()


async def _wait_until(predicate, timeout=5.0, interval=0.1):
    elapsed = 0.0
    while elapsed < timeout:
        if predicate():
            return True
        await asyncio.sleep(interval)
        elapsed += interval
    return False


async def test_consumer_records_hold_skips_pass_and_dedupes(nats_url):
    with PostgresContainer("postgres:18-alpine") as postgres:
        dsn = postgres.get_connection_url(driver=None)
        migrate(dsn)
        conn = db_connect(dsn)
        repo = NotificationRepository(conn)

        nc, js = await nats_connect(nats_url)
        await ensure_stream(js, "SETTLEMENT_EVENTS", ["settlement.>", "transaction.risk-scored"])

        consumer = Consumer(repo)
        await consumer.start(js)

        held_tx_id = "11111111-1111-1111-1111-111111111111"
        passed_tx_id = "22222222-2222-2222-2222-222222222222"
        settlement_id = "33333333-3333-3333-3333-333333333333"

        await js.publish(
            "transaction.risk-scored",
            json.dumps({"transaction_id": held_tx_id, "decision": "hold", "score": 40}).encode(),
        )
        await js.publish(
            "transaction.risk-scored",
            json.dumps({"transaction_id": passed_tx_id, "decision": "pass", "score": 0}).encode(),
        )
        # redeliver the same held event again -- must not create a second row
        await js.publish(
            "transaction.risk-scored",
            json.dumps({"transaction_id": held_tx_id, "decision": "hold", "score": 40}).encode(),
        )
        await js.publish(
            "settlement.finalized",
            json.dumps({"settlement_id": settlement_id, "transaction_count": 3}).encode(),
        )

        def rows():
            with conn.cursor() as cur:
                cur.execute("SELECT type, subject_id FROM notifications ORDER BY type")
                return cur.fetchall()

        found = await _wait_until(lambda: len(rows()) == 2)
        assert found, f"expected 2 rows, got {rows()}"

        result = {(t, str(sid)) for t, sid in rows()}
        assert result == {("risk_hold", held_tx_id), ("settlement_finalized", settlement_id)}

        await nc.close()
        conn.close()
