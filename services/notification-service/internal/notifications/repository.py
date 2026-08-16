import uuid

import psycopg
from psycopg.types.json import Jsonb


class NotificationRepository:
    def __init__(self, conn: psycopg.Connection):
        self._conn = conn

    def record(self, type_: str, subject_id: uuid.UUID, payload: dict) -> bool:
        with self._conn.cursor() as cur:
            cur.execute(
                """
                INSERT INTO notifications (id, type, subject_id, payload)
                VALUES (%s, %s, %s, %s)
                ON CONFLICT (type, subject_id) DO NOTHING
                """,
                (uuid.uuid4(), type_, subject_id, Jsonb(payload)),
            )
            inserted = cur.rowcount > 0
        self._conn.commit()
        return inserted
