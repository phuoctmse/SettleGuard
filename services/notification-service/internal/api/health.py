import json
from datetime import datetime
from http.server import BaseHTTPRequestHandler, HTTPServer
from urllib.parse import parse_qs, urlparse

MAX_LIMIT = 200
DEFAULT_LIMIT = 50


class _NotificationServer(HTTPServer):
    def __init__(self, address, repo):
        super().__init__(address, _Handler)
        self.repo = repo


class _Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        parsed = urlparse(self.path)
        if parsed.path == "/health":
            self._write(200, b"ok", content_type="text/plain")
        elif parsed.path == "/notifications":
            self._handle_notifications(parsed)
        else:
            self.send_response(404)
            self.end_headers()

    def _handle_notifications(self, parsed):
        params = parse_qs(parsed.query)
        type_ = params.get("type", [None])[0]
        since_raw = params.get("since", [None])[0]
        limit_raw = params.get("limit", [str(DEFAULT_LIMIT)])[0]

        since = None
        if since_raw is not None:
            try:
                since = datetime.fromisoformat(since_raw)
            except ValueError:
                self._write(400, b'{"error":"invalid since"}')
                return

        try:
            limit = min(int(limit_raw), MAX_LIMIT)
        except ValueError:
            self._write(400, b'{"error":"invalid limit"}')
            return

        notifications = self.server.repo.list(type_=type_, since=since, limit=limit)
        self._write(200, json.dumps(notifications).encode())

    def _write(self, status: int, body: bytes, content_type: str = "application/json"):
        self.send_response(status)
        self.send_header("Content-Type", content_type)
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, format, *args):
        pass


def create_server(host: str, port: int, repo) -> HTTPServer:
    return _NotificationServer((host, port), repo)
