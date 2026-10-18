"""A real browser enforces the gateway's CORS allowlist (spec §7.2)."""
from playwright.sync_api import Browser, Page

FETCH = """async ([url, key]) => {
  try {
    const r = await fetch(url, { headers: { Authorization: 'Bearer ' + key } });
    return 'status ' + r.status;
  } catch (e) {
    return 'blocked: ' + e.name;
  }
}"""


def _client_url(e2e_key) -> str:
    return f"http://localhost:8000/accounts/clients/{e2e_key.client_id}"


def test_allowed_origin_can_call_the_gateway(page: Page, e2e_key):
    page.goto("/")  # http://localhost:8090 -- the allowlisted origin
    assert page.evaluate(FETCH, [_client_url(e2e_key), e2e_key.raw_key]) == "status 200"


def test_foreign_origin_is_blocked_by_the_browser(browser: Browser, e2e_key, web_app_url):
    # Same build, different origin string: 127.0.0.1 is not in CORS_ALLOWED_ORIGINS.
    foreign = web_app_url.replace("localhost", "127.0.0.1")
    context = browser.new_context()
    page = context.new_page()
    try:
        page.goto(foreign)
        assert page.evaluate(FETCH, [_client_url(e2e_key), e2e_key.raw_key]) == "blocked: TypeError"
    finally:
        context.close()
