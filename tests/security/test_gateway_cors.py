# OWASP API8:2023 Security Misconfiguration -- CORS allowlist, fail-closed.
import httpx
import pytest

ALLOWED = "http://localhost:8090"
FOREIGN = ["http://evil.test", "http://localhost:8090.evil.test", "null", "http://localhost:8091"]


def _preflight(base_url: str, origin: str) -> httpx.Response:
    return httpx.options(
        f"{base_url}/accounts/accounts",
        headers={
            "Origin": origin,
            "Access-Control-Request-Method": "GET",
            "Access-Control-Request-Headers": "authorization",
        },
        timeout=5.0,
    )


@pytest.mark.parametrize("origin", FOREIGN)
def test_foreign_origin_gets_no_allow_origin(gateway_base_url, origin):
    simple = httpx.get(f"{gateway_base_url}/health", headers={"Origin": origin}, timeout=5.0)
    assert "access-control-allow-origin" not in simple.headers
    assert "access-control-allow-origin" not in _preflight(gateway_base_url, origin).headers


def test_allowed_origin_is_echoed_exactly(gateway_base_url):
    resp = _preflight(gateway_base_url, ALLOWED)
    assert resp.headers.get("access-control-allow-origin") == ALLOWED
    assert "authorization" in resp.headers.get("access-control-allow-headers", "").lower()


def test_never_wildcard_with_credentials(gateway_base_url):
    for origin in [ALLOWED, *FOREIGN]:
        headers = _preflight(gateway_base_url, origin).headers
        if headers.get("access-control-allow-credentials") == "true":
            assert headers.get("access-control-allow-origin") != "*"
