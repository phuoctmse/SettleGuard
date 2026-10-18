"""Client lookup -> account list -> account detail, through the gateway."""
import uuid

from playwright.sync_api import Page, expect


def test_account_list_and_balance_match_the_api(page: Page, accounts, ledger, wait_until):
    client_id = accounts.create_client(f"E2E accounts {uuid.uuid4()}").json()["id"]
    debit = accounts.create_account(client_id).json()["id"]
    credit = accounts.create_account(client_id).json()["id"]
    resp = ledger.post_transaction([
        {"account_id": debit, "direction": "debit", "amount": 100, "reason": "e2e"},
        {"account_id": credit, "direction": "credit", "amount": 100, "reason": "e2e"},
    ])
    assert resp.status_code == 201, resp.text
    # Balances are projected asynchronously from ledger events.
    wait_until(lambda: True if accounts.get_account(debit).json()["balance"] == -100 else None)

    page.goto("/")
    page.get_by_test_id("client-id-input").fill(client_id)
    page.get_by_test_id("view-accounts-button").click()

    expect(page.get_by_test_id(f"account-card-{credit}")).to_be_visible()
    page.get_by_test_id(f"account-card-{debit}").click()
    expect(page.get_by_test_id("account-balance")).to_have_text("Balance: -100")
