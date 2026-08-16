import subprocess
from pathlib import Path

import psycopg

MIGRATIONS_DIR = Path(__file__).parent / "migrations"


def connect(dsn: str) -> psycopg.Connection:
    return psycopg.connect(dsn)


def migrate(dsn: str) -> None:
    # Convert Windows path to Unix-style for migrate CLI compatibility
    migrations_path = str(MIGRATIONS_DIR).replace("\\", "/")
    subprocess.run(
        ["migrate", "-path", migrations_path, "-database", dsn, "up"],
        check=True,
    )
