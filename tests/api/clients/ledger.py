import httpx


class LedgerClient:
    def __init__(self, base_url: str):
        self._http = httpx.Client(base_url=base_url, timeout=5.0)

    def post_transaction(self, entries: list[dict]) -> httpx.Response:
        return self._http.post("/transactions", json={"entries": entries})

    def list_entries(self, *, account_id: str | None = None, transaction_id: str | None = None) -> httpx.Response:
        params = {}
        if account_id:
            params["account_id"] = account_id
        if transaction_id:
            params["transaction_id"] = transaction_id
        return self._http.get("/entries", params=params)
