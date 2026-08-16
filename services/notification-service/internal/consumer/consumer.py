import json
import logging
import uuid

from nats.aio.msg import Msg
from nats.js import JetStreamContext
from nats.js.api import AckPolicy, ConsumerConfig, DeliverPolicy

from internal.broker.connection import SETTLEMENT_EVENTS_STREAM
from internal.notifications.repository import NotificationRepository

logger = logging.getLogger(__name__)

RISK_HOLD_DURABLE = "notification-service-risk-hold"
SETTLEMENT_FINALIZED_DURABLE = "notification-service-settlement-finalized"


def decide_risk_scored(payload: dict) -> dict | None:
    if payload.get("decision") != "hold":
        return None
    return {
        "type": "risk_hold",
        "subject_id": uuid.UUID(payload["transaction_id"]),
        "payload": payload,
    }


def decide_settlement_finalized(payload: dict) -> dict:
    return {
        "type": "settlement_finalized",
        "subject_id": uuid.UUID(payload["settlement_id"]),
        "payload": payload,
    }


class Consumer:
    def __init__(self, repo: NotificationRepository):
        self._repo = repo

    async def start(self, js: JetStreamContext) -> None:
        await js.subscribe(
            "transaction.risk-scored",
            stream=SETTLEMENT_EVENTS_STREAM,
            durable=RISK_HOLD_DURABLE,
            cb=self._handle_risk_scored,
            manual_ack=True,
            config=ConsumerConfig(
                durable_name=RISK_HOLD_DURABLE,
                ack_policy=AckPolicy.EXPLICIT,
                deliver_policy=DeliverPolicy.ALL,
            ),
        )
        await js.subscribe(
            "settlement.finalized",
            stream=SETTLEMENT_EVENTS_STREAM,
            durable=SETTLEMENT_FINALIZED_DURABLE,
            cb=self._handle_settlement_finalized,
            manual_ack=True,
            config=ConsumerConfig(
                durable_name=SETTLEMENT_FINALIZED_DURABLE,
                ack_policy=AckPolicy.EXPLICIT,
                deliver_policy=DeliverPolicy.ALL,
            ),
        )

    async def _handle_risk_scored(self, msg: Msg) -> None:
        await self._handle(msg, decide_risk_scored)

    async def _handle_settlement_finalized(self, msg: Msg) -> None:
        await self._handle(msg, decide_settlement_finalized)

    async def _handle(self, msg: Msg, decide) -> None:
        try:
            payload = json.loads(msg.data)
        except json.JSONDecodeError:
            logger.error("consumer: malformed payload on %s, terminating message", msg.subject)
            await msg.term()
            return

        record = decide(payload)
        if record is not None:
            try:
                self._repo.record(record["type"], record["subject_id"], record["payload"])
            except Exception:
                logger.exception("consumer: failed to record notification for %s", msg.subject)
                await msg.nak()
                return

        await msg.ack()
