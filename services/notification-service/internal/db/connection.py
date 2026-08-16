import subprocess
from pathlib import Path

import psycopg

MIGRATIONS_DIR = Path(__file__).parent / "migrations"


def connect(dsn: str) -> psycopg.Connection:
    return psycopg.connect(dsn)


def migrate(dsn: str) -> None:
    # Convert Windows path to Unix-style for migrate CLI compatibility
    migrations_path = str(MIGRATIONS_DIR).replace("\\", "/")
    # Add sslmode=disable for testcontainers postgres which doesn't have SSL
    migrate_dsn = dsn if "sslmode" in dsn else f"{dsn}?sslmode=disable"
    subprocess.run(
        ["migrate", "-path", migrations_path, "-database", migrate_dsn, "up"],
        check=True,
    )
