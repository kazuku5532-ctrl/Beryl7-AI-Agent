"""Lightweight HTTP server providing /health, /metrics (Prometheus), and /metrics/summary endpoints.
"""
import time
import json
import hmac
import threading
from http.server import HTTPServer, BaseHTTPRequestHandler
from typing import Optional, Dict, Any
from agent.metrics import metrics_factory
from agent.logger import agent_logger


class AgentHTTPRequestHandler(BaseHTTPRequestHandler):
    """HTTP Request Handler for Prometheus metrics and health monitoring."""

    auth_token: Optional[str] = None
    start_time: float = time.time()
    ip_request_counts: Dict[str, List[float]] = {}
    rate_limit_lock: threading.Lock = threading.Lock()

    def log_message(self, format: str, *args: Any) -> None:
        agent_logger.info(f"[HTTP Server] {self.address_string()} - {format % args}")

    def _check_rate_limit(self) -> bool:
        client_ip = self.client_address[0]
        now = time.time()
        with self.rate_limit_lock:
            timestamps = self.ip_request_counts.get(client_ip, [])
            timestamps = [t for t in timestamps if now - t < 60.0]
            if len(timestamps) >= 30:
                self.ip_request_counts[client_ip] = timestamps
                return False
            timestamps.append(now)
            self.ip_request_counts[client_ip] = timestamps
            return True

    def do_GET(self) -> None:
        if not self._check_rate_limit():
            self.send_response(429)
            self.send_header("Content-Type", "application/json")
            self.end_headers()
            self.wfile.write(b'{"error":"Too Many Requests: Limit 30 req/min"}')
            return

        if self.path == "/health":
            self.handle_health()
        elif self.path == "/metrics":
            self.handle_metrics()
        elif self.path == "/metrics/summary":
            self.handle_metrics_summary()
        else:
            self.send_response(404)
            self.send_header("Content-Type", "application/json")
            self.end_headers()
            self.wfile.write(b'{"error":"Endpoint Not Found"}')

    def handle_health(self) -> None:
        summary = metrics_factory.get_summary_dict()
        healthy = summary.get("router_reachable", True)

        payload = {
            "healthy": healthy,
            "timestamp": int(time.time()),
            "router_reachable": healthy,
            "cache_size": summary.get("skills_in_store", 0),
            "uptime_seconds": int(time.time() - self.start_time),
        }

        status_code = 200 if healthy else 503
        self.send_response(status_code)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(json.dumps(payload).encode("utf-8"))

    def handle_metrics(self) -> None:
        if self.auth_token:
            auth_header = self.headers.get("Authorization", "")
            expected_auth = f"Bearer {self.auth_token}"
            if not hmac.compare_digest(auth_header.encode("utf-8"), expected_auth.encode("utf-8")):
                self.send_response(401)
                self.send_header("Content-Type", "application/json")
                self.end_headers()
                self.wfile.write(b'{"error":"Unauthorized: Invalid Bearer Token"}')
                return

        metrics_text = metrics_factory.export_prometheus_text()
        self.send_response(200)
        self.send_header("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
        self.end_headers()
        self.wfile.write(metrics_text.encode("utf-8"))

    def handle_metrics_summary(self) -> None:
        summary = metrics_factory.get_summary_dict()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(json.dumps(summary, indent=2).encode("utf-8"))


def start_http_server(port: int = 8888, auth_token: Optional[str] = None) -> HTTPServer:
    """Start HTTP server in background daemon thread."""
    AgentHTTPRequestHandler.auth_token = auth_token
    server = HTTPServer(("0.0.0.0", port), AgentHTTPRequestHandler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    agent_logger.info(f"✓ HTTP Health & Prometheus Server started on 0.0.0.0:{port}")
    return server
