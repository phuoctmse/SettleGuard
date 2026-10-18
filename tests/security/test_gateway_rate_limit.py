# OWASP API4:2023 Unrestricted Resource Consumption. Runs against the
# low-limit gateway instance (:8001, IP=20/min, client=10/min). Order
# matters: the per-client test runs first while the IP bucket is still full.
import httpx

IP_LIMIT = 20
CLIENT_LIMIT = 10


def _assert_rate_limited(resp: httpx.Response) -> None:
    assert resp.status_code == 429
    assert resp.json() == {"error": "rate limit exceeded"}
    assert resp.headers.get("X-Request-Id")


def test_per_client_limit(rate_limited_gateway_url, api_key_factory):
    key = api_key_factory()
    http = httpx.Client(base_url=rate_limited_gateway_url, headers={"Authorization": f"Bearer {key.raw_key}"}, timeout=5.0)

    statuses = [http.get(f"/accounts/clients/{key.client_id}") for _ in range(2 * CLIENT_LIMIT + 1)]
    first_limited = next(i for i, r in enumerate(statuses) if r.status_code == 429)

    assert first_limited <= CLIENT_LIMIT
    _assert_rate_limited(statuses[first_limited])


def test_unauthenticated_flood_is_throttled_before_auth(rate_limited_gateway_url):
    # Key guessing: the IP tier runs before auth, so wrong keys hit 429
    # rather than an unbounded stream of 401s.
    http = httpx.Client(base_url=rate_limited_gateway_url, headers={"Authorization": "Bearer sg_live_" + "B" * 43}, timeout=5.0)

    responses = [http.get("/accounts/accounts") for _ in range(2 * IP_LIMIT + 1)]

    assert {r.status_code for r in responses} <= {401, 429}
    limited = [r for r in responses if r.status_code == 429]
    assert limited, "no 429 after 2x the IP limit"
    _assert_rate_limited(limited[0])
