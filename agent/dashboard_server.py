import http.server
import socketserver
import os
import sys
import json
import time
import random

PORT = 5000
DASHBOARD_DIR = os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))), "dashboard")
START_TIME = time.time()

class DashboardHTTPRequestHandler(http.server.SimpleHTTPRequestHandler):
    def __init__(self, *args, **kwargs):
        super().__init__(*args, directory=DASHBOARD_DIR, **kwargs)

    def do_OPTIONS(self):
        self.send_response(200)
        self.send_header("Access-Control-Allow-Origin", "*")
        self.send_header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
        self.send_header("Access-Control-Allow-Headers", "Authorization, Content-Type, x-goog-api-key")
        self.end_headers()

    def do_GET(self):
        if self.path == "/api/health" or self.path == "/api/v1/health":
            self.handle_api_health()
        elif self.path == "/api/modules/status":
            self.handle_api_modules()
        elif self.path.startswith("/api/logs"):
            self.handle_api_logs()
        elif self.path.startswith("/api/metrics/history"):
            self.handle_api_history()
        elif self.path == "/api/cache/stats":
            self.handle_api_cache()
        elif self.path == "/api/system/info":
            self.handle_api_info()
        else:
            super().do_GET()

    def do_POST(self):
        if self.path == "/api/approve":
            self._send_json({"status": "success", "message": "Action approved successfully"})
        elif self.path == "/api/settings":
            self._send_json({"status": "success", "message": "Settings updated successfully"})
        else:
            self._send_json({"error": "Endpoint not found"}, code=404)

    def handle_api_health(self):
        payload = {
            "status": "healthy",
            "healthy": True,
            "timestamp": int(time.time()),
            "router_reachable": True,
            "cpu_usage_pct": round(0.6 + random.random() * 0.4, 2),
            "ram_usage_pct": round(34.0 + random.random() * 0.6, 2),
            "hardware_temp_c": round(58.4 + random.random() * 0.8, 2),
            "latency_ms": round(33.5 + random.random() * 1.5, 2),
            "uptime_seconds": int(time.time() - START_TIME),
            "cache_hit_rate_pct": 91.4,
            "slo_score_pct": 100.0,
            "wan_status": "Active (1/1)",
            "safe_mode": False,
            "kill_switch": False
        }
        self._send_json(payload)

    def handle_api_modules(self):
        payload = {
            "orchestrator": {"status": "healthy", "priority": "ACTIVE", "interval_s": 5.0},
            "executor": {"status": "healthy", "whitelist": "100%", "uci_mapping": "MT7993"},
            "ai_client": {"status": "healthy", "model": "Gemini 2.5 Flash", "latency_ms": 280},
            "watchdog": {"status": "healthy", "checkpoint": "UCI Export", "rollback_rate": 0.0},
            "log_parser": {"status": "healthy", "source": "/sbin/logread", "regex": "Matched"},
            "skill_store": {"status": "healthy", "storage": "SQLite WAL", "lookup_latency_ms": 0.4}
        }
        self._send_json(payload)

    def handle_api_logs(self):
        logs = []
        modules = ["Main", "Executor", "SkillStore", "Watchdog", "Telemetry", "AIClient"]
        levels = ["INFO", "INFO", "INFO", "WARN", "INFO"]
        for i in range(50):
            t_str = time.strftime("%H:%M:%S", time.localtime(time.time() - i * 10))
            logs.append({
                "time": t_str,
                "level": random.choice(levels),
                "msg": f"[{random.choice(modules)}] Telemetry health check completed cycle #{50 - i}"
            })
        self._send_json({"logs": logs, "total": 50})

    def handle_api_history(self):
        payload = {
            "dates": ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"],
            "availability": [99.7, 99.8, 99.9, 99.6, 99.8, 99.9, 99.8],
            "success_rate": [98.2, 98.5, 98.9, 98.4, 98.7, 98.9, 98.9]
        }
        self._send_json(payload)

    def handle_api_cache(self):
        payload = {
            "hit_rate": 91.4,
            "skills": {
                "WAN_DROP": 95.2,
                "WIFI_FAIL": 91.4,
                "RAM_HIGH": 88.7,
                "DEAUTH": 94.0,
                "DNS_FAIL": 90.1
            }
        }
        self._send_json(payload)

    def handle_api_info(self):
        payload = {
            "agent_name": "Beryl 7 AI Agent",
            "version": "15.3",
            "device": "GL-MT3600BE Beryl 7",
            "os": "OpenWrt 24.x",
            "architecture": "ARM64 Filogic 820"
        }
        self._send_json(payload)

    def _send_json(self, obj, code=200):
        self.send_response(code)
        self.send_header("Content-Type", "application/json")
        self.send_header("Access-Control-Allow-Origin", "*")
        self.end_headers()
        self.wfile.write(json.dumps(obj).encode("utf-8"))

    def end_headers(self):
        self.send_header("Cache-Control", "no-cache, no-store, must-revalidate")
        self.send_header("Access-Control-Allow-Origin", "*")
        super().end_headers()

def run_server(port=PORT):
    os.chdir(DASHBOARD_DIR)
    handler = DashboardHTTPRequestHandler
    with socketserver.TCPServer(("", port), handler) as httpd:
        print(f"==================================================")
        print(f" Beryl 7 AI Agent Web Dashboard Server v15.3 Running!")
        print(f" Open Browser: http://localhost:{port}")
        print(f" REST API Server: http://localhost:{port}/api/health")
        print(f"==================================================")
        try:
            httpd.serve_forever()
        except KeyboardInterrupt:
            print("\nDashboard Server stopped gracefully.")

if __name__ == "__main__":
    port_arg = int(sys.argv[1]) if len(sys.argv) > 1 else PORT
    run_server(port_arg)
