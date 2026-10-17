# OWASP API2:2023 Broken Authentication -- AUTH-01 in docs/BUSINESS_RULES.md.
from clients.gateway import GatewayClient


def test_issued_key_is_accepted(gateway_base_url, api_key_factory):
    key = api_key_factory()
    resp = GatewayClient(gateway_base_url, key.raw_key).http.get(f"/accounts/clients/{key.client_id}")
    assert resp.status_code == 200
    assert resp.json()["id"] == key.client_id
