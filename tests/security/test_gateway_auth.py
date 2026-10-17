# OWASP API2:2023 Broken Authentication -- AUTH-01 in docs/BUSINESS_RULES.md.
import socket
from urllib.parse import urlparse

import httpx
import pytest

from clients.gateway import GatewayClient
from clients.gateway_admin import revoke_key

PROXIED_PREFIXES = ["/ledger/entries", "/accounts/accounts", "/settlement/settlements", "/notifications/notifications"]
UNKNOWN_KEY = "sg_live_" + "A" * 43


def _assert_unauthorized(resp: httpx.Response) -> None:
    assert resp.status_code == 401
    assert resp.json() == {"error": "unauthorized"}
    assert resp.headers.get("X-Request-Id")


def test_issued_key_is_accepted(gateway_base_url, api_key_factory):
    key = api_key_factory()
    resp = GatewayClient(gateway_base_url, key.raw_key).http.get(f"/accounts/clients/{key.client_id}")
    assert resp.status_code == 200
    assert resp.json()["id"] == key.client_id


def _raw_get(base_url: str, path: str, authorization: str) -> httpx.Response:
    # httpx refuses header values with trailing whitespace, so send the bytes ourselves.
    u = urlparse(base_url)
    req = f"GET {path} HTTP/1.1\r\nHost: {u.netloc}\r\nAuthorization: {authorization}\r\nConnection: close\r\n\r\n"
    with socket.create_connection((u.hostname, u.port), timeout=5.0) as sock:
        sock.sendall(req.encode())
        data = b""
        while chunk := sock.recv(65536):
            data += chunk
    head, _, body = data.partition(b"\r\n\r\n")
    lines = head.decode("latin-1").split("\r\n")
    headers = [tuple(line.split(": ", 1)) for line in lines[1:]]
    if any(k.lower() == "transfer-encoding" and v.lower() == "chunked" for k, v in headers):
        out = b""
        while body:
            size, _, rest = body.partition(b"\r\n")
            n = int(size, 16)
            if n == 0:
                break
            out, body = out + rest[:n], rest[n + 2:]
        body = out
        headers = [(k, v) for k, v in headers if k.lower() != "transfer-encoding"]
    return httpx.Response(int(lines[0].split()[1]), headers=headers, content=body)



@pytest.mark.parametrize(
    "authorization",
    [
        None,
        "Basic dXNlcjpwYXNz",
        "Bearer",
        "Bearer ",
        "Bearer    ",
        f"Bearer {UNKNOWN_KEY}",
        "Bearer sk_live_" + "A" * 43,
        f"Token {UNKNOWN_KEY}",
    ],
    ids=["missing", "basic", "bearer-no-space", "bearer-empty", "bearer-blank", "unknown-key", "wrong-prefix", "wrong-scheme"],
)
def test_rejected_authorization_headers(gateway_base_url, authorization):
    headers = {"Authorization": authorization} if authorization is not None else {}
    if authorization is not None and authorization != authorization.rstrip():
        resp = _raw_get(gateway_base_url, "/accounts/accounts", authorization)
    else:
        resp = httpx.get(f"{gateway_base_url}/accounts/accounts", headers=headers, timeout=5.0)
    _assert_unauthorized(resp)


def test_revoked_key_is_rejected(gateway_base_url, api_key_factory):
    key = api_key_factory()
    client = GatewayClient(gateway_base_url, key.raw_key)
    assert client.http.get(f"/accounts/clients/{key.client_id}").status_code == 200

    revoke_key(key.key_id)

    _assert_unauthorized(client.http.get(f"/accounts/clients/{key.client_id}"))


def test_invalid_first_header_does_not_fall_back_to_a_valid_second(gateway_base_url, api_key_factory):
    key = api_key_factory()
    # httpx sends both lines when given a list of tuples.
    resp = httpx.get(
        f"{gateway_base_url}/accounts/clients/{key.client_id}",
        headers=[("Authorization", f"Bearer {UNKNOWN_KEY}"), ("Authorization", f"Bearer {key.raw_key}")],
        timeout=5.0,
    )
    _assert_unauthorized(resp)


def test_rejected_request_is_never_forwarded(gateway_base_url, accounts, ledger):
    # ledger-service writes synchronously, so reading entries straight from
    # its own port right after is a complete check -- no waiting needed.
    client_id = accounts.create_client("Pentest not-forwarded").json()["id"]
    acc1 = accounts.create_account(client_id).json()["id"]
    acc2 = accounts.create_account(client_id).json()["id"]

    resp = GatewayClient(gateway_base_url, UNKNOWN_KEY).http.post(
        "/ledger/transactions",
        json={"entries": [
            {"account_id": acc1, "direction": "debit", "amount": 100, "reason": "pentest"},
            {"account_id": acc2, "direction": "credit", "amount": 100, "reason": "pentest"},
        ]},
    )
    _assert_unauthorized(resp)
    assert ledger.list_entries(account_id=acc1).json() == []


@pytest.mark.parametrize("path", PROXIED_PREFIXES)
def test_every_proxied_prefix_requires_a_key(gateway_base_url, path):
    _assert_unauthorized(httpx.get(f"{gateway_base_url}{path}", timeout=5.0))


def test_health_is_the_only_open_route(gateway_base_url):
    resp = httpx.get(f"{gateway_base_url}/health", timeout=5.0)
    assert resp.status_code == 200
    assert resp.json() == {"status": "ok"}
