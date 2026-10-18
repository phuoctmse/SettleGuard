import httpx


class GatewayClient:
    """httpx client for the gateway. raw_key=None sends no Authorization
    header at all, which is what unauthenticated probes need."""

    def __init__(self, base_url: str, raw_key: str | None = None):
        headers = {"Authorization": f"Bearer {raw_key}"} if raw_key else {}
        self.http = httpx.Client(base_url=base_url, headers=headers, timeout=10.0)
