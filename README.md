# Beryl 7 AI Agent 🛠️

A lightweight network monitoring and self-healing agent designed for OpenWrt routers, tested on the **GL.iNet Beryl 7 (GL-MT3600BE)**.

The system consists of a compiled Go daemon running on the router to collect system metrics, a Python controller for optional API routing, and an HTML/JS dashboard for real-time visualization.

---

## 📂 Repository Structure

- `go-agent/` : Go daemon running on OpenWrt to read system telemetry (`/proc`, `ubus`) and handle basic network remediation tasks.
- `agent/` : Python HTTP server providing local API endpoints and serving the dashboard.
- `dashboard/` : Web dashboard interface and single-file bundle ([Beryl7_Dashboard_Standalone.html](Beryl7_Dashboard_Standalone.html)).
- `docs/` : Technical notes including architecture details ([docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)), operational notes ([docs/OPERATIONS.md](docs/OPERATIONS.md)), and benchmark sample logs ([docs/benchmark.md](docs/benchmark.md)).
- `tests/` : Basic unit, integration, and stress test scripts.

---

## ⚡ Measured Resource Usage (GL-MT3600BE)

Sample metrics observed during local testing on GL-MT3600BE (Filogic 820, 512MB RAM, OpenWrt 24.10):

- **Memory (RAM):** ~ 13.0 MB (VmRSS)
- **CPU Usage:** ~ 1.2% (on a 5-second polling loop)
- **Hardware Temperature:** ~ 59.5°C
- **API Latency:** ~ 29 ms (local network)

*Note: Resource consumption may vary depending on router hardware, active network load, and customized polling intervals.*

---

## 📡 Available API Endpoints

| Endpoint | Method | Description | Role Required |
| :--- | :--- | :--- | :--- |
| `/api/health` | `GET` | Returns CPU, RAM, temperature, latency, and uptime data | `viewer` |
| `/api/modules/status` | `GET` | Returns status of internal system modules | `viewer` |
| `/api/logs` | `GET` | Displays recent system log entries | `viewer` |
| `/metrics` | `GET` | Exports basic Prometheus-compatible metrics | `viewer` |
| `/api/budget/status` | `GET` | Current API usage & remaining daily budget | `viewer` |
| `/api/circuit-breaker` | `GET` | Circuit breaker state (`CLOSED`, `OPEN`, `HALF_OPEN`) | `viewer` |
| `/api/approve` | `POST` | Allows manual operator approval for queued actions | `operator` |
| `/api/config/reload` | `POST` | Live in-memory config reload without daemon restart | `operator` |

---

## 🔐 Role-Based Access Control (RBAC)

- **`viewer`**: Read-only access to health, metrics, module status, and logs (No token required or `viewer-token`).
- **`operator`**: High-privilege role to approve queued remediation actions and trigger live config reloads (`APPROVE_TOKEN`).
- **`admin`**: High-privilege access for token rotation, config overrides, and skill store management (`AUTH_TOKEN`).

---

## 🚀 Quick Start & Testing

### 1. View Dashboard Locally
Open [Beryl7_Dashboard_Standalone.html](Beryl7_Dashboard_Standalone.html) directly in any modern browser, or run the local Python server:

```bash
python agent/dashboard_server.py 5000
```

Then visit `http://localhost:5000`.

### 2. Build Go Daemon for Router
To cross-compile the Go binary for Linux ARM64:

```bash
cd go-agent
GOOS=linux GOARCH=arm64 go build -o beryl7-agent ./cmd
```

### 3. Run Unit Tests & Coverage
```bash
cd go-agent
go test -v -coverpkg=./... -coverprofile=coverage.out ./...
```
