import httpx


class AccountsClient:
    def __init__(self, base_url: str):
        self._http = httpx.Client(base_url=base_url, timeout=5.0)

    def create_client(self, name: str) -> httpx.Response:
        return self._http.post("/clients", json={"name": name})

    def get_client(self, client_id: str) -> httpx.Response:
        return self._http.get(f"/clients/{client_id}")

    def update_client_status(self, client_id: str, status: str) -> httpx.Response:
        return self._http.patch(f"/clients/{client_id}/status", json={"status": status})

    def create_account(self, client_id: str, external_ref: str = "") -> httpx.Response:
        return self._http.post("/accounts", json={"client_id": client_id, "external_ref": external_ref})

    def get_account(self, account_id: str) -> httpx.Response:
        return self._http.get(f"/accounts/{account_id}")

    def list_accounts(self, client_id: str) -> httpx.Response:
        return self._http.get("/accounts", params={"client_id": client_id})

    def update_account_status(self, account_id: str, status: str) -> httpx.Response:
        return self._http.patch(f"/accounts/{account_id}/status", json={"status": status})
