import httpx


class SettlementClient:
    def __init__(self, base_url: str):
        self._http = httpx.Client(base_url=base_url, timeout=5.0)

    def get_transaction(self, transaction_id: str) -> httpx.Response:
        return self._http.get(f"/transactions/{transaction_id}")

    def list_transactions(self, status: str) -> httpx.Response:
        return self._http.get("/transactions", params={"status": status})

    def approve(self, transaction_id: str) -> httpx.Response:
        return self._http.post(f"/transactions/{transaction_id}/approve")

    def reject(self, transaction_id: str) -> httpx.Response:
        return self._http.post(f"/transactions/{transaction_id}/reject")

    def list_settlements(self) -> httpx.Response:
        return self._http.get("/settlements")

    def get_settlement(self, settlement_id: str) -> httpx.Response:
        return self._http.get(f"/settlements/{settlement_id}")
