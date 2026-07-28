# Beryl 7 AI Agent — Production Deployment Guide 📖

This guide outlines step-by-step instructions for deploying the **Beryl 7 AI Agent Go Daemon** onto the **GL.iNet Beryl 7 (GL-MT3600BE)** router running OpenWrt, and connecting the **Enterprise Operations Dashboard**.

---

## 📋 Prerequisites

1. **GL.iNet Beryl 7 Router (GL-MT3600BE)** running OpenWrt v24+ (IP: `192.168.8.1`).
2. SSH access enabled on router with root privileges.
3. Go compiler (v1.22+) installed on local build workstation.
4. Python 3.10+ installed on local workstation (for python fallback controller).

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

### Method A: Direct HTTP Wget Transfer (Recommended)
On local workstation, host the binary folder:
```powershell
python -m http.server 8000 --directory go-agent
```
On router via SSH (`ssh root@192.168.8.1`):
```bash
wget -O /usr/bin/beryl7-agent http://<YOUR_WORKSTATION_IP>:8000/beryl7-agent
chmod +x /usr/bin/beryl7-agent
```

### Method B: Base64 SSH Stream
```powershell
python scratch/deploy.py
```

---

## ⚙️ Step 3: Run Go Daemon as Background Service

On router via SSH, launch the daemon in background mode with custom polling interval (5 seconds):

```bash
nohup /usr/bin/beryl7-agent -interval 5 > /tmp/beryl7.log 2>&1 &
```

Verify daemon is running:
```bash
ps | grep beryl7-agent
```

Test HTTP API endpoint directly on router:
```bash
curl http://192.168.8.1:8888/api/health
```

---

## 💻 Step 4: Launch Enterprise Operations Dashboard

### Option 1: Standalone HTML File (Zero Installation)
Simply double-click and open [c:\Users\kazuk\Documents\Beryl7_Dashboard_Standalone.html](file:///c:/Users/kazuk\Documents\Beryl7_Dashboard_Standalone.html) in Chrome, Edge, or Firefox.

### Option 2: Python Web Controller Server
Run the python dashboard server on local workstation:
```bash
python agent/dashboard_server.py 5000
```
Access in browser: `http://localhost:5000`

---

## 🔧 Step 5: Configure Admin Settings in Dashboard

1. Click the **Gear Icon** (`⚙️`) in the top navigation bar of the Dashboard.
2. Enter your Router IP (`http://192.168.8.1:8888`).
3. Click **Save & Apply Settings**. The settings are persisted in `localStorage` and telemetry streaming starts immediately.
