# Beryl 7 AI Agent — Production Deployment Guide 📖

This guide outlines step-by-step instructions for deploying the **Beryl 7 AI Agent Go Daemon** onto the **GL.iNet Beryl 7 (GL-MT3600BE)** router running OpenWrt.

---

## 📋 Prerequisites

1. **GL.iNet Beryl 7 Router (GL-MT3600BE)** running OpenWrt v24+ (IP: `192.168.8.1`).
2. SSH access enabled on router with root privileges.
3. Go compiler (v1.22+) installed on local build workstation.
4. Python 3.10+ installed on local workstation (for CI/CD and deployment tooling).

---

## 📦 Step 1: Cross-Compile Go Agent Daemon

On your local workstation, cross-compile the Go daemon for Linux ARM64 architecture:

```powershell
cd go-agent
$env:GOOS="linux"
$env:GOARCH="arm64"
go build -o beryl7-agent ./cmd
```

---

## 🚀 Step 2: Deploy Binary to Router

Transfer the compiled binary `beryl7-agent` to the router `/usr/bin/` directory and grant execution permissions:

### Method A: Automated Deployment Script (Recommended)
```powershell
$env:ROUTER_PASS="your_router_password"
python tools/dev_scripts/deploy_router.py
```

### Method B: Direct HTTP Wget Transfer
On local workstation, host the binary folder:
```powershell
python -m http.server 8000 --directory go-agent
```
On router via SSH (`ssh root@192.168.8.1`):
```bash
wget -O /usr/bin/beryl7-agent http://<YOUR_WORKSTATION_IP>:8000/beryl7-agent
chmod +x /usr/bin/beryl7-agent
```

---

## ⚙️ Step 3: Run Go Daemon as Background Service

On router via SSH, launch the daemon service:

```bash
/etc/init.d/beryl7-agent start
```

Verify daemon is running:
```bash
ps | grep beryl7-agent
```

---

## 🧪 Step 4: Verify Management API & Prometheus Endpoints

Test HTTP API endpoints directly on router or from local network:

```bash
# 1. Health Status
curl -s http://192.168.8.1:8888/api/health

# 2. Prometheus Metrics
curl -s http://192.168.8.1:8888/metrics
```

---

## 🤖 Step 5: Telegram Bot Operations

Once `TELEGRAM_BOT_TOKEN` and `TELEGRAM_CHAT_ID` are configured in `/etc/beryl7/agent.key` and `/etc/beryl7/agent.env`:
- Send `/status` to the bot to inspect real-time router vitals.
- Send `/health` to trigger on-demand anomaly checks and auto-remediation.
- Send `/boost` to activate 160MHz Wi-Fi 7 mode.
- Send `/reboot` to reboot the router remotely.
