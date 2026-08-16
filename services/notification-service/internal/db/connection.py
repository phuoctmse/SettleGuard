import subprocess
from pathlib import Path
from urllib.parse import parse_qsl, urlencode, urlsplit, urlunsplit

import psycopg

MIGRATIONS_DIR = Path(__file__).parent / "migrations"


def connect(dsn: str) -> psycopg.Connection:
    return psycopg.connect(dsn)


def _ensure_sslmode_in_dsn(dsn: str) -> str:
    """
    Ensure DSN has an explicit sslmode parameter.

    If sslmode is not already present in the DSN, defaults to 'disable'.
    This is a permissive default applied to any DSN without an explicit sslmode,
    including production DSNs. Use this for environments that don't enforce SSL
    (e.g., testcontainers local dev, internal networks).

    Properly merges query parameters to handle DSNs that already have other
    query params (e.g. ?connect_timeout=5).
    """
    parsed = urlsplit(dsn)

    # Parse existing query parameters
    query_params = dict(parse_qsl(parsed.query))

    # Only add sslmode if not already present
    if "sslmode" not in query_params:
        query_params["sslmode"] = "disable"

    # Reconstruct DSN with merged query params
    new_query = urlencode(query_params)
    return urlunsplit((parsed.scheme, parsed.netloc, parsed.path, new_query, parsed.fragment))


def migrate(dsn: str) -> None:
    # Convert Windows path to Unix-style for migrate CLI compatibility
    migrations_path = str(MIGRATIONS_DIR).replace("\\", "/")
    # Ensure sslmode is set in DSN for environments without SSL enforcement
    migrate_dsn = _ensure_sslmode_in_dsn(dsn)
    subprocess.run(
        ["migrate", "-path", migrations_path, "-database", migrate_dsn, "up"],
        check=True,
    )
