import pytest
from testcontainers.core.container import DockerContainer
from testcontainers.core.waiting_utils import WaitStrategy

from internal.broker.connection import connect, ensure_stream


class LogWaitStrategy(WaitStrategy):
    """Custom wait strategy to wait for a log message in container output."""

    def __init__(self, log_text: str):
        self.log_text = log_text

    def wait_until_ready(self, target):
        """Wait until log_text appears in container logs."""
        import time

        start_time = time.time()
        timeout = 120  # 120 seconds default timeout

        while time.time() - start_time < timeout:
            try:
                logs_tuple = target.get_logs()
                # get_logs() returns a tuple of (stdout, stderr) or bytes
                if isinstance(logs_tuple, tuple):
                    stdout, stderr = logs_tuple
                    if isinstance(stdout, bytes):
                        stdout = stdout.decode("utf-8", errors="ignore")
                    if isinstance(stderr, bytes):
                        stderr = stderr.decode("utf-8", errors="ignore")
                    logs = stdout + stderr
                elif isinstance(logs_tuple, bytes):
                    logs = logs_tuple.decode("utf-8", errors="ignore")
                else:
                    logs = str(logs_tuple)

                if self.log_text in logs:
                    return
            except Exception as e:
                # Continue waiting even if there's an exception
                pass
            time.sleep(0.5)

        raise TimeoutError(f"Timeout waiting for '{self.log_text}' in container logs")


@pytest.fixture
def nats_url():
    container = (
        DockerContainer("nats:2.10-alpine")
        .with_command(["-js"])
        .with_exposed_ports(4222)
        .waiting_for(LogWaitStrategy("Server is ready"))
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
