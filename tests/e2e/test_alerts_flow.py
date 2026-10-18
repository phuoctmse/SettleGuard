"""A held transaction raises a risk_hold alert that the Alerts tab shows."""
from playwright.sync_api import Page, expect


def test_risk_hold_alert_is_listed(page: Page, held_tx, notifications, wait_until):
    tx_id = held_tx()

    def find_notification():
        for n in notifications.list_notifications(type_="risk_hold", limit=200).json():
            if n["subject_id"] == tx_id:
                return n["id"]
        return None

    notification_id = wait_until(find_notification)

    page.goto("/")
    page.get_by_test_id("tab-alerts").click()  # first visit: fresh fetch
    expect(page.get_by_test_id(f"alert-card-{notification_id}")).to_be_visible()
