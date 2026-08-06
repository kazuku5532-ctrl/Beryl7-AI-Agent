#!/usr/bin/env python3
"""
Development & CI Helper Script: Deploy Compiled Go ARM64 Binary to OpenWrt Router
STRICTLY FOR WORKSTATION / CI USAGE. ZERO EXTRA LIBRARIES REQUIRED (100% NATIVE PARAMIKO).
"""
import os
import sys
import time
import paramiko

# STRICT ENFORCEMENT: ZERO HARDCODED SECRETS ALLOWED IN REPOSITORY.
ROUTER_IP = os.getenv("ROUTER_IP", "192.168.8.1")
ROUTER_USER = os.getenv("ROUTER_USER", "root")
ROUTER_PASS = os.getenv("ROUTER_PASS", "")

if not ROUTER_PASS:
    print("Error: ROUTER_PASS environment variable is not set. Please set $env:ROUTER_PASS='your_password' before running.")
    sys.exit(1)

BINARY_PATH = os.path.abspath(os.path.join(os.path.dirname(__file__), "../../go-agent/beryl7-agent"))

if not os.path.exists(BINARY_PATH):
    print(f"Error: Binary not found at {BINARY_PATH}. Please run go build first.")
    sys.exit(1)

print(f"Connecting via SSH to OpenWrt Router at {ROUTER_IP}...")
ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect(ROUTER_IP, username=ROUTER_USER, password=ROUTER_PASS, timeout=10)

def run_ssh(cmd, check_status=True):
    stdin, stdout, stderr = ssh.exec_command(cmd)
    out = stdout.read().decode('utf-8', errors='ignore').strip()
    err = stderr.read().decode('utf-8', errors='ignore').strip()
    exit_status = stdout.channel.recv_exit_status()
    if check_status and exit_status != 0:
        print(f"SSH Command Failed (exit code {exit_status}): {cmd}\nStderr: {err}")
        ssh.close()
        sys.exit(1)
    return out

print("[1/5] Stopping existing beryl7-agent process on router...")
run_ssh("killall -9 beryl7-agent 2>/dev/null || true", check_status=False)

print("[2/5] Creating binary backup at /usr/bin/beryl7-agent.backup...")
run_ssh("cp /usr/bin/beryl7-agent /usr/bin/beryl7-agent.backup 2>/dev/null || true", check_status=False)

print("[3/5] Uploading compiled ARM64 binary via native Paramiko SSH binary stream...")
try:
    stdin, stdout, stderr = ssh.exec_command("cat > /tmp/beryl7-agent-new")
    with open(BINARY_PATH, "rb") as f:
        stdin.write(f.read())
    stdin.close()
    out = stdout.read().decode('utf-8', errors='ignore').strip()
    err = stderr.read().decode('utf-8', errors='ignore').strip()
    exit_status = stdout.channel.recv_exit_status()
    if exit_status != 0:
        print(f"Binary Upload Failed (exit status {exit_status}): {err}")
        ssh.close()
        sys.exit(1)
    print("Binary uploaded successfully via native Paramiko SSH stream.")
except Exception as e:
    print(f"File Transfer Failed: {e}")
    ssh.close()
    sys.exit(1)

print("[4/5] Moving binary, setting permissions, and starting procd daemon...")
run_ssh("mv /tmp/beryl7-agent-new /usr/bin/beryl7-agent")
run_ssh("chmod +x /usr/bin/beryl7-agent")

# Native OpenWrt Procd service start (checking executable permission -x), with fallback to standalone nohup
start_out = run_ssh("if [ -x /etc/init.d/beryl7-agent ]; then /etc/init.d/beryl7-agent restart; else nohup /usr/bin/beryl7-agent -config /etc/beryl7/agent.env > /var/log/beryl7-agent-nohup.log 2>&1 & fi")
print("Daemon Start Output:", start_out)

time.sleep(3)
ps_out = run_ssh("ps | grep beryl7-agent | grep -v grep", check_status=False)
print("[5/5] Router Process Status:", ps_out)

ssh.close()
print("Deployment completed successfully!")
