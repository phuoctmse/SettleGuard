"""Approve and reject a held transaction from the Held tab (SETTLEMENT-05)."""
from playwright.sync_api import Page, expect


def _open_held_tab(page: Page, tx_id: str):
    page.goto("/")  # seeded before the first visit, so the tab fetches it
    page.get_by_test_id("tab-held").click()
    expect(page.get_by_test_id(f"held-tx-{tx_id}")).to_be_visible()


def test_approve_removes_the_row_and_releases_the_transaction(page: Page, held_tx, settlement):
    tx_id = held_tx()
    _open_held_tab(page, tx_id)

    page.get_by_test_id(f"approve-{tx_id}").click()

    expect(page.get_by_test_id(f"held-tx-{tx_id}")).to_have_count(0)
    # The 5 s batch may already have settled it.
    assert settlement.get_transaction(tx_id).json()["status"] in ("pending_settlement", "settled")


def test_reject_removes_the_row_and_marks_the_transaction_rejected(page: Page, held_tx, settlement):
    tx_id = held_tx()
    _open_held_tab(page, tx_id)

    page.get_by_test_id(f"reject-{tx_id}").click()

    expect(page.get_by_test_id(f"held-tx-{tx_id}")).to_have_count(0)
    assert settlement.get_transaction(tx_id).json()["status"] == "rejected"
