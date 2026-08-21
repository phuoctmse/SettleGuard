import uuid
from datetime import datetime

import psycopg
from psycopg.types.json import Jsonb


class NotificationRepository:
    def __init__(self, conn: psycopg.Connection):
        self._conn = conn

    def record(self, type_: str, subject_id: uuid.UUID, payload: dict) -> bool:
        try:
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
        except Exception:
            self._conn.rollback()
            raise

    def list(self, type_: str | None = None, since: datetime | None = None, limit: int = 50) -> list[dict]:
        query = "SELECT id, type, subject_id, payload, created_at FROM notifications WHERE TRUE"
        params: list = []
        if type_ is not None:
            query += " AND type = %s"
            params.append(type_)
        if since is not None:
            query += " AND created_at >= %s"
            params.append(since)
        query += " ORDER BY created_at DESC LIMIT %s"
        params.append(limit)

        with self._conn.cursor() as cur:
            cur.execute(query, params)
            rows = cur.fetchall()

        return [
            {
                "id": str(row[0]),
                "type": row[1],
                "subject_id": str(row[2]),
                "payload": row[3],
                "created_at": row[4].isoformat(),
            }
            for row in rows
        ]
