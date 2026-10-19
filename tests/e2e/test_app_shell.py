"""The web build loads through the gateway with the injected key."""
from playwright.sync_api import Page, expect

TABS = ["tab-accounts", "tab-held", "tab-settlements", "tab-alerts"]


def test_app_shell_renders_every_tab(page: Page):
    page.goto("/")
    for tab in TABS:
        expect(page.get_by_test_id(tab)).to_be_visible()
    expect(page.get_by_test_id("client-id-input")).to_be_visible()
