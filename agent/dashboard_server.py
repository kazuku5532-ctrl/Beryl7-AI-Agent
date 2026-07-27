import http.server
import socketserver
import os
import sys

PORT = 5000
DASHBOARD_DIR = os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))), "dashboard")

class DashboardHTTPRequestHandler(http.server.SimpleHTTPRequestHandler):
    def __init__(self, *args, **kwargs):
        super().__init__(*args, directory=DASHBOARD_DIR, **kwargs)

    def end_headers(self):
        self.send_header("Cache-Control", "no-cache, no-store, must-revalidate")
        self.send_header("Access-Control-Allow-Origin", "*")
        super().end_headers()

def run_server(port=PORT):
    os.chdir(DASHBOARD_DIR)
    handler = DashboardHTTPRequestHandler
    with socketserver.TCPServer(("", port), handler) as httpd:
        print(f"==================================================")
        print(f" Beryl 7 AI Agent Web Dashboard Server Running! ")
        print(f" Open Browser: http://localhost:{port}")
        print(f"==================================================")
        try:
            httpd.serve_forever()
        except KeyboardInterrupt:
            print("\nDashboard Server stopped gracefully.")

if __name__ == "__main__":
    port_arg = int(sys.argv[1]) if len(sys.argv) > 1 else PORT
    run_server(port_arg)
