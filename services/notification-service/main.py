import asyncio
import os
import threading

from internal.api.health import create_server
from internal.broker.connection import (
    SETTLEMENT_EVENTS_STREAM,
    SETTLEMENT_EVENTS_SUBJECTS,
    connect as nats_connect,
    ensure_stream,
)
from internal.consumer.consumer import Consumer
from internal.db.connection import connect as db_connect, migrate
from internal.notifications.repository import NotificationRepository


async def main() -> None:
    dsn = os.environ.get("DATABASE_URL")
    if not dsn:
        raise SystemExit("DATABASE_URL environment variable is required")
    nats_url = os.environ.get("NATS_URL")
    if not nats_url:
        raise SystemExit("NATS_URL environment variable is required")

    migrate(dsn)
    conn = db_connect(dsn)
    repo = NotificationRepository(conn)

    nc, js = await nats_connect(nats_url)
    await ensure_stream(js, SETTLEMENT_EVENTS_STREAM, SETTLEMENT_EVENTS_SUBJECTS)

    consumer = Consumer(repo)
    await consumer.start(js)

    addr = os.environ.get("LISTEN_ADDR", ":8083")
    host, port = addr.rsplit(":", 1)
    server = create_server(host or "0.0.0.0", int(port), repo)
    threading.Thread(target=server.serve_forever, daemon=True).start()
    print(f"notification-service listening on {addr}")

    try:
        await asyncio.Event().wait()
    finally:
        server.shutdown()
        await nc.close()
        conn.close()


if __name__ == "__main__":
    asyncio.run(main())
