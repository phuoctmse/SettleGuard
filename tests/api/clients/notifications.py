import httpx


class NotificationsClient:
    def __init__(self, base_url: str):
        self._http = httpx.Client(base_url=base_url, timeout=5.0)

    def list_notifications(self, *, type_: str | None = None, limit: int | None = None) -> httpx.Response:
        params = {}
        if type_:
            params["type"] = type_
        if limit:
            params["limit"] = limit
        return self._http.get("/notifications", params=params)
