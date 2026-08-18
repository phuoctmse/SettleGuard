import nats
from nats.aio.client import Client
from nats.js import JetStreamContext
from nats.js.errors import NotFoundError

SETTLEMENT_EVENTS_STREAM = "SETTLEMENT_EVENTS"
SETTLEMENT_EVENTS_SUBJECTS = ["settlement.>", "transaction.risk-scored"]


async def connect(url: str) -> tuple[Client, JetStreamContext]:
    nc = await nats.connect(url)
    js = nc.jetstream()
    return nc, js


async def ensure_stream(js: JetStreamContext, name: str, subjects: list[str]) -> None:
    try:
        await js.stream_info(name)
    except NotFoundError:
        await js.add_stream(name=name, subjects=subjects)
