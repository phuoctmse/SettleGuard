"""Issue and revoke gateway API keys through the real adminctl CLI.

adminctl writes straight to the gateway's database and prints the raw key
exactly once; there is no HTTP endpoint for this by design
(services/gateway/README.md). GATEWAY_ADMINCTL points at a prebuilt binary
(CI); without it we fall back to `go run ./cmd/adminctl` inside
services/gateway (local).
"""
import os
import re
import shlex
import subprocess
from dataclasses import dataclass
from pathlib import Path

GATEWAY_DIR = Path(__file__).resolve().parents[3] / "services" / "gateway"
GATEWAY_DATABASE_URL = os.environ.get(
    "GATEWAY_DATABASE_URL",
    "postgres://gateway:gateway@localhost:5437/gateway?sslmode=disable",
)

_ID_LINE = re.compile(r"^id:\s+([0-9a-f-]{36})$", re.MULTILINE)
_KEY_LINE = re.compile(r"^key:\s+(sg_live_[A-Za-z0-9_-]{43})$", re.MULTILINE)


@dataclass(frozen=True)
class IssuedKey:
    client_id: str
    key_id: str
    raw_key: str


def _adminctl(*args: str) -> str:
    cmd = shlex.split(os.environ.get("GATEWAY_ADMINCTL", "go run ./cmd/adminctl"))
    result = subprocess.run(
        [*cmd, *args],
        cwd=GATEWAY_DIR,
        env={**os.environ, "DATABASE_URL": GATEWAY_DATABASE_URL},
        capture_output=True,
        text=True,
        timeout=120,
    )
    if result.returncode != 0:
        raise RuntimeError(f"adminctl {args[0]} failed: {result.stderr.strip()}")
    return result.stdout


def create_key(client_id: str, label: str) -> IssuedKey:
    out = _adminctl("create", "--client-id", client_id, "--label", label)
    key_id, raw = _ID_LINE.search(out), _KEY_LINE.search(out)
    if not key_id or not raw:
        raise RuntimeError(f"unexpected adminctl output:\n{out}")
    return IssuedKey(client_id=client_id, key_id=key_id.group(1), raw_key=raw.group(1))


def revoke_key(key_id: str) -> None:
    _adminctl("revoke", "--id", key_id)
