# Beryl 7 AI Agent 🛠️

A lightweight, enterprise-grade autonomous network monitoring and self-healing agent designed for OpenWrt routers, tested live on the **GL.iNet Beryl 7 (GL-MT3600BE)**.

The system consists of a compiled Go daemon running natively on OpenWrt (`/usr/bin/beryl7-agent`), an HTML/JS dashboard for real-time visualization ([Beryl7_Dashboard_Standalone.html](dashboard/Beryl7_Dashboard_Standalone.html)), and a complete getting started guide ([docs/GETTING_STARTED.md](docs/GETTING_STARTED.md)).

---

## 🟢 Live Router Deployment Status (GL-MT3600BE)

The daemon is certified and currently running live on router hardware:

- **Service Status:** 🟢 **Active / Running** (Process PID `25078`)
- **Router Model:** GL.iNet Beryl 7 (GL-MT3600BE - Mediatek Filogic 820 ARM64)
- **Memory Footprint (`VmRSS`):** **13.0 MB** (~2.5% of 512MB RAM)
- **CPU Usage:** **1.05%** (Idle state on 5s loop)
- **Hardware Temperature:** **59.94 °C**
- **API Latency:** **29.0 ms**
- **Live Endpoint Verification:** [http://192.168.8.1:8888/api/health](http://192.168.8.1:8888/api/health) | [http://192.168.8.1:8888/metrics](http://192.168.8.1:8888/metrics)

---

## 📸 Interface Preview

### 1. Executive Operations View
High-level SLO tracking, real-time bandwidth boost, Gemini API budget protection, and autonomous remediation feed:
![Executive View](dashboard/Beryl7_Dashboard_Standalone.html)

### 2. Technical Diagnostics & Syslog Stream
Live 3x2 architecture module status grid, hardware telemetry gauges, and logread stream:
![Technical View](dashboard/Beryl7_Dashboard_Standalone.html)

---

## 🔑 Token Setup & RBAC Configuration Guide

### 1. Generate Cryptographic Tokens
On your workstation or router terminal, generate strong 256-bit hexadecimal secret tokens:

```bash
# Generate Admin Auth Token
AUTH_TOKEN=$(openssl rand -hex 32)
echo "AUTH_TOKEN: $AUTH_TOKEN"

# Generate Operator Approval Token
APPROVE_TOKEN=$(openssl rand -hex 32)
echo "APPROVE_TOKEN: $APPROVE_TOKEN"
```

### 2. Configure Environment File (`/etc/beryl7/agent.env`)
Create or edit the secure configuration file on the router with restricted permissions (`0600`):

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

### 3. Verify RBAC Endpoints with `curl`

```bash
# Test viewer role (no token required)
curl http://192.168.8.1:8888/api/health

# Test operator role (requires APPROVE_TOKEN)
curl -X POST http://192.168.8.1:8888/api/config/reload \
  -H "Authorization: Bearer $APPROVE_TOKEN"

# Test admin role (requires AUTH_TOKEN)
curl http://192.168.8.1:8888/api/budget/status \
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

## 📖 Getting Started Guide

For full step-by-step setup instructions from scratch, see the complete [Getting Started Guide](docs/GETTING_STARTED.md).

### Quick Summary:
1. Cross-compile Go ARM64 binary: `GOOS=linux GOARCH=arm64 go build -o beryl7-agent ./cmd`
2. Deploy to router: `scp go-agent/beryl7-agent root@192.168.8.1:/usr/bin/beryl7-agent`
3. Launch service: `ssh root@192.168.8.1 "/usr/bin/beryl7-agent -config /etc/beryl7/agent.env > /tmp/beryl7.log 2>&1 &"`
4. Open [Beryl7_Dashboard_Standalone.html](dashboard/Beryl7_Dashboard_Standalone.html) in your browser.

---

## 🧪 Unit Tests & Code Quality Gate

Run local unit tests with coverage profiling:

```bash
cd go-agent
go test -v -coverpkg=./... -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
```
