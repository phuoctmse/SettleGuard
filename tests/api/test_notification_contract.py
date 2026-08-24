def test_list_notifications_defaults(notifications):
    resp = notifications.list_notifications()
    assert resp.status_code == 200
    assert isinstance(resp.json(), list)


def test_list_notifications_rejects_bad_limit(notification_base_url):
    import httpx

    resp = httpx.get(f"{notification_base_url}/notifications", params={"limit": "not-a-number"})
    assert resp.status_code == 400
