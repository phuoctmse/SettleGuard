import threading
import urllib.request

from internal.api.health import create_server


def test_health_endpoint_returns_200():
    server = create_server("127.0.0.1", 0)  # port 0 = OS picks a free port
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
    server = create_server("127.0.0.1", 0)
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
