from testcontainers.postgres import PostgresContainer

from internal.db.connection import connect, migrate


def test_migrate_creates_notifications_table():
    with PostgresContainer("postgres:18-alpine") as postgres:
        dsn = postgres.get_connection_url(driver=None)

        migrate(dsn)

        conn = connect(dsn)
        try:
            with conn.cursor() as cur:
                cur.execute(
                    """
                    SELECT column_name FROM information_schema.columns
                    WHERE table_name = 'notifications'
                    ORDER BY column_name
                    """
                )
                columns = {row[0] for row in cur.fetchall()}
        finally:
            conn.close()

        assert columns == {"id", "type", "subject_id", "payload", "created_at"}


def test_migrate_is_idempotent():
    with PostgresContainer("postgres:18-alpine") as postgres:
        dsn = postgres.get_connection_url(driver=None)

        migrate(dsn)
        migrate(dsn)  # must not raise
