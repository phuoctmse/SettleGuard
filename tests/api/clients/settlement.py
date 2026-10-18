import httpx


class SettlementClient:
    def __init__(self, base_url: str):
        self._http = httpx.Client(base_url=base_url, timeout=5.0)

    def get_transaction(self, transaction_id: str) -> httpx.Response:
        return self._http.get(f"/transactions/{transaction_id}")

    def list_transactions(self, status: str, *, limit: int | None = None, cursor: str | None = None) -> httpx.Response:
        params = {"status": status}
        if limit is not None:
            params["limit"] = limit
        if cursor is not None:
            params["cursor"] = cursor
        return self._http.get("/transactions", params=params)

    def approve(self, transaction_id: str) -> httpx.Response:
        return self._http.post(f"/transactions/{transaction_id}/approve")

    def reject(self, transaction_id: str) -> httpx.Response:
        return self._http.post(f"/transactions/{transaction_id}/reject")

    def list_settlements(self, *, limit: int | None = None, cursor: str | None = None) -> httpx.Response:
        params = {}
        if limit is not None:
            params["limit"] = limit
        if cursor is not None:
            params["cursor"] = cursor
        return self._http.get("/settlements", params=params)

    def get_settlement(self, settlement_id: str) -> httpx.Response:
        return self._http.get(f"/settlements/{settlement_id}")
