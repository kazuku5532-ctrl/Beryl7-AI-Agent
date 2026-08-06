#!/usr/bin/env python3
"""
Development & CI Helper Script: Deploy Compiled Go ARM64 Binary to OpenWrt Router
STRICTLY FOR WORKSTATION / CI USAGE. ZERO PYTHON REQUIRED ON ROUTER.
"""
import os
import sys
import time
import http.server
import socketserver
import threading
import urllib.request
import paramiko

ROUTER_IP = os.getenv("ROUTER_IP", "192.168.8.1")
ROUTER_USER = os.getenv("ROUTER_USER", "root")
ROUTER_PASS = os.getenv("ROUTER_PASS", "")

if not ROUTER_PASS:
    print("Error: ROUTER_PASS environment variable is not set. Please set $env:ROUTER_PASS='your_password' before running.")
    sys.exit(1)
PORT = 8999
LOCAL_IP = "192.168.8.102"

BINARY_PATH = os.path.abspath(os.path.join(os.path.dirname(__file__), "../../go-agent/beryl7-agent"))

if not os.path.exists(BINARY_PATH):
    print(f"Error: Binary not found at {BINARY_PATH}. Please run go build first.")
    sys.exit(1)

class QuietHandler(http.server.SimpleHTTPRequestHandler):
    def log_message(self, format, *args):
        pass

os.chdir(os.path.dirname(BINARY_PATH))
handler = QuietHandler
httpd = socketserver.TCPServer(("", PORT), handler)
server_thread = threading.Thread(target=httpd.serve_forever, daemon=True)
server_thread.start()
print(f"Started temporary HTTP server at port {PORT}")

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect(ROUTER_IP, username=ROUTER_USER, password=ROUTER_PASS, timeout=10)

def run_ssh(cmd):
    stdin, stdout, stderr = ssh.exec_command(cmd)
    return stdout.read().decode('utf-8', errors='ignore').strip()

print("[1/5] Stopping existing beryl7-agent process on router...")
run_ssh("killall -9 beryl7-agent 2>/dev/null || true")

print("[2/5] Creating binary backup at /usr/bin/beryl7-agent.backup...")
run_ssh("cp /usr/bin/beryl7-agent /usr/bin/beryl7-agent.backup 2>/dev/null || true")

print("[3/5] Downloading compiled ARM64 binary via wget on router...")
dl_cmd = f"wget -O /tmp/beryl7-agent-new http://{LOCAL_IP}:{PORT}/beryl7-agent && mv /tmp/beryl7-agent-new /usr/bin/beryl7-agent"
out = run_ssh(dl_cmd)
print("Download output:", out)

print("[4/5] Setting executable permissions and starting daemon...")
run_ssh("chmod +x /usr/bin/beryl7-agent")
run_ssh("nohup /usr/bin/beryl7-agent -config /etc/beryl7/agent.env > /var/log/beryl7-agent-nohup.log 2>&1 &")

time.sleep(3)
ps_out = run_ssh("ps | grep beryl7-agent | grep -v grep")
print("[5/5] Router Process Status:", ps_out)

httpd.shutdown()
ssh.close()
print("Deployment completed successfully!")
