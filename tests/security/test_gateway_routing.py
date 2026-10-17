# OWASP API8:2023 Security Misconfiguration -- the prefix router must not be
# talked into skipping auth or reaching a different upstream.
import http.client
import json
from urllib.parse import urlsplit

import pytest

TRAVERSALS = [
    "/ledger/../accounts/clients/{cid}",
    "/ledger/%2e%2e/accounts/clients/{cid}",
    "/ledger%2F..%2Faccounts/clients/{cid}",
    "//accounts/clients/{cid}",
    "/LEDGER/../accounts/clients/{cid}",
    "/ledgerx/../accounts/clients/{cid}",
]


def _raw_get(base_url: str, path: str, raw_key: str | None = None) -> tuple[int, bytes]:
    parts = urlsplit(base_url)
    conn = http.client.HTTPConnection(parts.hostname, parts.port, timeout=5.0)
    headers = {"Authorization": f"Bearer {raw_key}"} if raw_key else {}
    try:
        conn.request("GET", path, headers=headers)  # path sent verbatim
        resp = conn.getresponse()
        return resp.status, resp.read()
    finally:
        conn.close()


@pytest.mark.parametrize("template", TRAVERSALS)
def test_traversal_without_key_never_succeeds(gateway_base_url, api_key_factory, template):
    key = api_key_factory()
    status, _ = _raw_get(gateway_base_url, template.format(cid=key.client_id))
    assert status in (400, 401, 404)


@pytest.mark.parametrize("template", TRAVERSALS)
def test_traversal_with_key_never_reaches_another_upstream(gateway_base_url, api_key_factory, template):
    key = api_key_factory()
    status, body = _raw_get(gateway_base_url, template.format(cid=key.client_id), key.raw_key)
    assert status != 200 or key.client_id not in body.decode(errors="replace")
    assert status < 500


def test_unknown_prefix_is_the_gateways_own_404(gateway_base_url, api_key_factory):
    key = api_key_factory()
    status, body = _raw_get(gateway_base_url, "/admin/keys", key.raw_key)
    assert status == 404
    assert json.loads(body) == {"error": "not found"}
