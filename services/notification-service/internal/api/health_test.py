import json
import threading
import urllib.request
import uuid
from unittest.mock import MagicMock

from internal.api.health import create_server


def test_health_endpoint_returns_200():
    server = create_server("127.0.0.1", 0, MagicMock())  # port 0 = OS picks a free port
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        port = server.server_address[1]
        response = urllib.request.urlopen(f"http://127.0.0.1:{port}/health")
        assert response.status == 200
    finally:
        server.shutdown()
        thread.join()


def test_unknown_path_returns_404():
    server = create_server("127.0.0.1", 0, MagicMock())
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        port = server.server_address[1]
        try:
            urllib.request.urlopen(f"http://127.0.0.1:{port}/nope")
            assert False, "expected HTTPError"
        except urllib.error.HTTPError as e:
            assert e.code == 404
    finally:
        server.shutdown()
        thread.join()


def test_notifications_endpoint_returns_repo_list():
    repo = MagicMock()
    repo.list.return_value = [
        {"id": str(uuid.uuid4()), "type": "risk_hold", "subject_id": str(uuid.uuid4()), "payload": {}, "created_at": "2026-08-20T00:00:00+00:00"}
    ]
    server = create_server("127.0.0.1", 0, repo)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        port = server.server_address[1]
        response = urllib.request.urlopen(f"http://127.0.0.1:{port}/notifications")
        body = json.loads(response.read())
        assert response.status == 200
        assert body == repo.list.return_value
    finally:
        server.shutdown()
        thread.join()


def test_notifications_endpoint_passes_query_params_to_repo():
    repo = MagicMock()
    repo.list.return_value = []
    server = create_server("127.0.0.1", 0, repo)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        port = server.server_address[1]
        urllib.request.urlopen(f"http://127.0.0.1:{port}/notifications?type=risk_hold&limit=10")
        repo.list.assert_called_once_with(type_="risk_hold", since=None, limit=10)
    finally:
        server.shutdown()
        thread.join()


def test_notifications_endpoint_rejects_invalid_limit():
    repo = MagicMock()
    server = create_server("127.0.0.1", 0, repo)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        port = server.server_address[1]
        try:
            urllib.request.urlopen(f"http://127.0.0.1:{port}/notifications?limit=not-a-number")
            assert False, "expected HTTPError"
        except urllib.error.HTTPError as e:
            assert e.code == 400
    finally:
        server.shutdown()
        thread.join()
