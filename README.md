# Beryl 7 AI Agent 🛠️

A lightweight, enterprise-grade autonomous network monitoring and self-healing agent designed for OpenWrt routers, tested live on the **GL.iNet Beryl 7 (GL-MT3600BE)**.

The system consists of a compiled Go daemon running natively on OpenWrt (`/usr/bin/beryl7-agent`), an HTML/JS dashboard for real-time visualization ([Beryl7_Dashboard_Standalone.html](dashboard/Beryl7_Dashboard_Standalone.html)), and automated operational playbooks.

---

## 📸 Interface Preview & Live Telemetry

### 1. Executive Operations View
High-level SLO tracking, real-time bandwidth boost, Gemini API budget protection, and autonomous remediation feed:
![Executive View](dashboard/Beryl7_Dashboard_Standalone.html)

### 2. Technical Diagnostics & Syslog Stream
Live 3x2 architecture module status grid, hardware telemetry gauges, and logread stream:
![Technical View](dashboard/Beryl7_Dashboard_Standalone.html)

---

## ⚡ Measured Live Telemetry (GL-MT3600BE Hardware)

Metrics measured directly from the active Go Daemon process (**PID `25078`**) running on GL-MT3600BE (Mediatek Filogic 820 ARM64, 512MB RAM, OpenWrt 24.10):

| Telemetry Parameter | Measured Value | Target Boundary | Evaluation Status |
| :--- | :--- | :--- | :--- |
| **System Availability** | **100% Operational** | `healthy` | 🟢 **Passed** |
| **Memory Footprint (`VmRSS`)** | **13.0 MB** (13,080 KB) | $< 64.0 \text{ MB}$ | 🟢 **Ultra-Light (~2.5% RAM)** |
| **CPU Utilization** | **1.05% CPU** | $< 5.0\%$ | 🟢 **Idle (> 98.9% Headroom)** |
| **Hardware Temp** | **59.94 °C** | $< 85.0 ^\circ\text{C}$ | 🟢 **Optimal Cool Zone** |
| **Network API Latency** | **29.0 ms** | $< 500.0 \text{ ms}$ | 🟢 **Instant Response** |

---

## 🔑 Token Setup & RBAC Configuration Guide

### Step 1: Generate Secure Tokens
On your management machine or router terminal, generate strong 256-bit hexadecimal secret tokens:

```bash
# Generate Admin Auth Token
AUTH_TOKEN=$(openssl rand -hex 32)
echo "AUTH_TOKEN: $AUTH_TOKEN"

# Generate Operator Approval Token
APPROVE_TOKEN=$(openssl rand -hex 32)
echo "APPROVE_TOKEN: $APPROVE_TOKEN"
```

### Step 2: Configure Environment File (`/etc/beryl7/agent.env`)
Create or edit the secure agent configuration file on the router with restricted permissions (`0600`):

```bash
cat <<EOF > /etc/beryl7/agent.env
AUTH_TOKEN=$AUTH_TOKEN
APPROVE_TOKEN=$APPROVE_TOKEN
LOG_LEVEL=INFO
HEALTH_PORT=8888
GEMINI_API_KEY=your_gemini_api_key_here
EOF

chmod 0600 /etc/beryl7/agent.env
```

### Step 3: Verify Role Access with `curl`

#### A. Viewer Access (Read-only status endpoints)
```bash
# Query health status (no token required or viewer role)
curl -X GET http://192.168.8.1:8888/api/health
```

#### B. Operator Access (Live Config Reload without restart)
```bash
# Reload in-memory config using APPROVE_TOKEN
curl -X POST http://192.168.8.1:8888/api/config/reload \
  -H "Authorization: Bearer $APPROVE_TOKEN"
```

#### C. Admin Access (Full administrative overrides)
```bash
# Verify Admin authorization token
curl -X GET http://192.168.8.1:8888/api/budget/status \
  -H "Authorization: Bearer $AUTH_TOKEN"
```

---

## 📡 Available API Endpoints & Access Control Matrix

| Endpoint | Method | Role Required | Description |
| :--- | :---: | :---: | :--- |
| `/api/health` | `GET` | `viewer` | Returns CPU, RAM, temperature, latency, uptime |
| `/api/modules/status` | `GET` | `viewer` | Status of 6 internal architecture modules |
| `/api/logs` | `GET` | `viewer` | Live logread syslog entries |
| `/metrics` | `GET` | `viewer` | Prometheus-compatible metrics export |
| `/api/budget/status` | `GET` | `viewer` | Daily API usage & remaining budget ($3.00 USD/day max) |
| `/api/circuit-breaker` | `GET` | `viewer` | State machine status (`CLOSED`, `OPEN`, `HALF_OPEN`) |
| `/api/config/reload` | `POST` | `operator` / `admin` | In-memory live config reload without service restart |
| `/api/approve` | `POST` | `operator` / `admin` | Manual approval for queued high-risk actions |

---

## 🚀 Getting Started (Deployment from Scratch)

### 1. Build ARM64 Binary
Cross-compile the native Go daemon for OpenWrt Linux ARM64:

```bash
cd go-agent
GOOS=linux GOARCH=arm64 go build -o beryl7-agent ./cmd
```

### 2. Deploy & Run on Router
Transfer binary to router and start native service:

```bash
# Upload binary to router
scp go-agent/beryl7-agent root@192.168.8.1:/usr/bin/beryl7-agent
ssh root@192.168.8.1 "chmod +x /usr/bin/beryl7-agent"

# Start background service
ssh root@192.168.8.1 "/usr/bin/beryl7-agent -config /etc/beryl7/agent.env > /tmp/beryl7.log 2>&1 &"
```

### 3. Open Standalone Dashboard
Double-click [Beryl7_Dashboard_Standalone.html](dashboard/Beryl7_Dashboard_Standalone.html) in any web browser to view live router metrics in real-time.

---

## 🧪 Unit Tests & Code Quality Gate

Run local unit tests with full statement coverage profiling:

```bash
cd go-agent
go test -v -coverpkg=./... -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
```
