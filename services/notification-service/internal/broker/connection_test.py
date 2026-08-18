import pytest
from testcontainers.core.container import DockerContainer
from testcontainers.core.wait_strategies import LogMessageWaitStrategy

from internal.broker.connection import connect, ensure_stream


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


async def test_ensure_stream_creates_then_is_idempotent(nats_url):
    nc, js = await connect(nats_url)
    try:
        await ensure_stream(js, "SETTLEMENT_EVENTS", ["settlement.>", "transaction.risk-scored"])
        await ensure_stream(js, "SETTLEMENT_EVENTS", ["settlement.>", "transaction.risk-scored"])  # must not raise

        info = await js.stream_info("SETTLEMENT_EVENTS")
        assert set(info.config.subjects) == {"settlement.>", "transaction.risk-scored"}
    finally:
        await nc.close()
