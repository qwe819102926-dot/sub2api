#!/usr/bin/env python3
"""Exercise the real Caddy binary on isolated loopback ports and temporary logs."""
import base64
import hashlib
import http.client
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
from pathlib import Path
import socket
import subprocess
import tempfile
import threading
import time
import urllib.error
import urllib.request


class Upstream(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    def log_message(self, *_args):
        pass

    def do_GET(self):
        if self.path == "/ws":
            digest = hashlib.sha1((self.headers["Sec-WebSocket-Key"] + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11").encode()).digest()
            self.send_response(101)
            self.send_header("Upgrade", "websocket")
            self.send_header("Connection", "Upgrade")
            self.send_header("Sec-WebSocket-Accept", base64.b64encode(digest).decode())
            self.end_headers()
            self.wfile.write(b"\x81\x02ok")
            self.wfile.flush()
            self.close_connection = True
            return
        if self.path == "/stream":
            self.send_response(200)
            self.send_header("Content-Type", "text/event-stream")
            self.send_header("Connection", "close")
            self.end_headers()
            self.wfile.write(b"data: first\n\n")
            self.wfile.flush()
            time.sleep(0.2)
            self.wfile.write(b"data: last\n\n")
            self.close_connection = True
            return
        body = json.dumps({"path": self.path, "auth": self.headers.get("Authorization"),
                           "cookie": self.headers.get("Cookie"), "key": self.headers.get("X-Api-Key")}).encode()
        self.send_response(200)
        self.send_header("X-Request-ID", "guard-verification-id")
        self.send_header("Set-Cookie", "secret=guard-secret-response")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)


def main():
    upstream = ThreadingHTTPServer(("127.0.0.1", 0), Upstream)
    threading.Thread(target=upstream.serve_forever, daemon=True).start()
    with socket.socket() as available:
        available.bind(("127.0.0.1", 0))
        port = available.getsockname()[1]
    with tempfile.TemporaryDirectory(prefix="sub2api-guard-test-") as folder:
        folder = Path(folder)
        snippet = Path(__file__).with_name("access-guard.caddy").read_text()
        snippet = snippet.replace("/var/log/caddy/sub2api-access.jsonl", str(folder / "access.jsonl"))
        config = folder / "Caddyfile"
        config.write_text("{\nadmin off\nauto_https off\n}\nhttp://127.0.0.1:" + str(port) + " {\n" + snippet +
                          "\nreverse_proxy 127.0.0.1:" + str(upstream.server_port) + "\n}\n")
        subprocess.run(["caddy", "validate", "--config", str(config), "--adapter", "caddyfile"], check=True, capture_output=True)
        with (folder / "runtime.log").open("w") as runtime:
            caddy = subprocess.Popen(["caddy", "run", "--config", str(config), "--adapter", "caddyfile"], stdout=runtime, stderr=runtime)
            try:
                for _ in range(50):
                    try:
                        with socket.create_connection(("127.0.0.1", port), timeout=0.2):
                            break
                    except OSError:
                        time.sleep(0.1)
                base = "http://127.0.0.1:" + str(port)
                blocked = ["/.env", "/.env.bak", "/api/.env", "/.git/HEAD", "/.aws/credentials",
                           "/wp-admin", "//wp-content/plugins/fix/up.php", "/foo.PHP", "/%2eenv", "/wp-login.php"]
                for path in blocked:
                    try:
                        urllib.request.urlopen(base + path, timeout=3)
                        raise AssertionError("expected blocked: " + path)
                    except urllib.error.HTTPError as error:
                        assert error.code == 404, (path, error.code)
                allowed = ["/", "/login", "/admin", "/v1/models", "/v1/responses", "/v1/chat/completions",
                           "/api/v1/auth/oauth/github/callback?code=guard-secret-query",
                           "/api/v1/payment/webhook/easypay?sign=guard-secret-query",
                           "/.well-known/acme-challenge/probe", "/assets/guide.php.png",
                           "/api/v1/plugin-ui/guard-secret-asset-token/index.html"]
                for path in allowed:
                    req = urllib.request.Request(base + path, headers={"Authorization": "Bearer guard-secret-auth",
                            "Cookie": "session=guard-secret-cookie", "X-Api-Key": "guard-secret-key",
                            "Referer": "https://example.invalid/?token=guard-secret-referrer"})
                    with urllib.request.urlopen(req, timeout=3) as response:
                        data = json.load(response)
                        assert data["path"] == path, "query altered in transit"
                        assert data["auth"] == "Bearer guard-secret-auth"
                        assert data["cookie"] == "session=guard-secret-cookie"
                        assert data["key"] == "guard-secret-key"
                with urllib.request.urlopen(base + "/stream", timeout=3) as response:
                    assert b"data: first" in response.readline()
                    assert b"data: last" in response.read()
                connection = http.client.HTTPConnection("127.0.0.1", port, timeout=3)
                connection.request("GET", "/ws", headers={"Connection": "Upgrade", "Upgrade": "websocket",
                    "Sec-WebSocket-Key": base64.b64encode(b"0123456789abcdef").decode(), "Sec-WebSocket-Version": "13"})
                response = connection.getresponse()
                assert response.status == 101
                connection.close()
                time.sleep(0.3)
                raw = (folder / "access.jsonl").read_text()
                assert "guard-secret" not in raw, "credential canary leaked"
                rows = [json.loads(line) for line in raw.splitlines()]
                assert sum(row.get("security_rule") == "sensitive_path" for row in rows) == len(blocked)
                assert any(row.get("request_id") == "guard-verification-id" for row in rows)
                print(json.dumps({"blocked_cases": len(blocked), "allowed_cases": len(allowed),
                                  "query_and_auth_forwarding": "passed", "log_redaction": "passed",
                                  "SSE": "passed", "WebSocket_upgrade": "passed"}))
            finally:
                caddy.terminate()
                caddy.wait(timeout=10)
                upstream.shutdown()


if __name__ == "__main__":
    main()
