def test_create_client_rejects_empty_name(accounts):
    resp = accounts.create_client("")
    assert resp.status_code == 400


def test_create_and_get_client(accounts):
    created = accounts.create_client("Test Client Co").json()
    assert created["status"] == "active"

    fetched = accounts.get_client(created["id"])
    assert fetched.status_code == 200
    assert fetched.json()["name"] == "Test Client Co"


def test_get_client_not_found(accounts):
    resp = accounts.get_client("00000000-0000-0000-0000-000000000000")
    assert resp.status_code == 404


def test_create_account_rejects_unknown_client(accounts):
    resp = accounts.create_account("00000000-0000-0000-0000-000000000000")
    assert resp.status_code == 404


def test_create_account_rejects_suspended_client(accounts):
    client = accounts.create_client("Suspend Me Co").json()
    accounts.update_client_status(client["id"], "suspended")

    resp = accounts.create_account(client["id"])

    assert resp.status_code == 422


def test_list_accounts_by_client(accounts):
    client = accounts.create_client("List Accounts Co").json()
    created = accounts.create_account(client["id"], external_ref="ext-1").json()

    resp = accounts.list_accounts(client["id"])

    assert resp.status_code == 200
    assert created["id"] in [a["id"] for a in resp.json()]
