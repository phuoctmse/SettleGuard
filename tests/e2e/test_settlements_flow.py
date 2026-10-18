"""An approved transaction lands in a settlement that the Settlements tab shows."""
from playwright.sync_api import Page, expect


def test_settlement_containing_the_transaction_is_listed(page: Page, held_tx, settlement, wait_until):
    tx_id = held_tx()
    resp = settlement.approve(tx_id)
    assert resp.status_code == 200, resp.text

    def find_settlement():
        for s in settlement.list_settlements().json()["items"]:  # newest first, first page (50)
            if tx_id in s["transaction_ids"]:
                return s["id"]
        return None

    settlement_id = wait_until(find_settlement)  # one 5 s batch cycle

    page.goto("/")
    page.get_by_test_id("tab-settlements").click()  # first visit: fresh fetch
    expect(page.get_by_test_id(f"settlement-card-{settlement_id}")).to_be_visible()
