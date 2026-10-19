"""A revoked key shows the error state and no data -- never a crash, never rows."""
import re

from playwright.sync_api import Browser, expect

from clients.gateway_admin import revoke_key


def test_revoked_key_shows_the_error_state(browser: Browser, web_app_url, api_key_factory, accounts):
    key = api_key_factory()
    # Seed one account (direct port, setup only) so "no rows" means the data
    # exists but is withheld, not that there was nothing to show.
    accounts.create_account(key.client_id).raise_for_status()
    revoke_key(key.key_id)
    context = browser.new_context(
        base_url=web_app_url,
        extra_http_headers={"Authorization": f"Bearer {key.raw_key}"},
    )
    page = context.new_page()
    try:
        page.goto("/")
        page.get_by_test_id("client-id-input").fill(key.client_id)
        page.get_by_test_id("view-accounts-button").click()

        # react-query retries three times before showing the error (~7.5 s).
        expect(page.get_by_text("Failed to load accounts.")).to_be_visible(timeout=20_000)
        expect(page.get_by_test_id(re.compile(r"^account-card-"))).to_have_count(0)
    finally:
        context.close()
