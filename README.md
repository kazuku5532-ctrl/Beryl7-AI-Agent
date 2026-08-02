# Beryl 7 AI Agent 🛠️

An autonomous reactive network remediation engine designed for OpenWrt routers, certified and running live on the **GL.iNet Beryl 7 (GL-MT3600BE)**.

[![Go Version](https://img.shields.io/badge/Go-1.21-blue.svg)](https://golang.org)
[![Architecture](https://img.shields.io/badge/Target_Arch-ARM64_Linux-blue.svg)](https://openwrt.org)
[![Build Status](https://img.shields.io/badge/CI_Gate-100%25_PASS-emerald.svg)](https://github.com/kazuku5532-ctrl/Beryl7-AI-Agent)
[![Status](https://img.shields.io/badge/Production-Certified_Live-emerald.svg)](http://192.168.8.1:8888/api/health)

The system consists of a zero-dependency compiled Go daemon running natively on OpenWrt (`/usr/bin/beryl7-agent`), an HTML/JS dashboard for real-time visualization ([Beryl7_Dashboard_Standalone.html](dashboard/Beryl7_Dashboard_Standalone.html)), a getting started guide ([docs/GETTING_STARTED.md](docs/GETTING_STARTED.md)), and performance benchmark reports ([docs/benchmark.md](docs/benchmark.md)).

---

## 🏛️ System Architecture & Remediation Engine

Beryl 7 AI Agent operates as a **Reactive Network Remediation Engine with Local Deterministic Heuristics & Cloud AI Skill Caching**:

1. **Local Deterministic Remediation (Offline First):** When internet connectivity drops or cloud APIs are unreachable, the daemon executes deterministic local healing rules (e.g., `restart_wan_interface` for WAN drops, `purge_memory_cache` for memory exhaustion, `optimize_wifi_channel` for Wi-Fi stalls) without requiring cloud connectivity.
2. **Action Whitelist Guardrails:** All remedial actions are strictly bounded by an explicit Action Whitelist in `executor.go` (11 whitelisted system actions).
3. **SQLite Skill Caching (`skills.db`):** Successful AI decision pairings are cached locally in SQLite with Exponential Moving Average (EMA) confidence scoring for instant zero-latency local re-use.
4. **4-Level Failsafe Engine (`FailsafeRecovery`):** Level 1 Binary Backup Restore ➔ Level 2 Degraded Mode ➔ Level 3 Factory Reset ➔ Level 4 Operator Alert.

---

## 🟢 Live Router Deployment Status (GL-MT3600BE)

The daemon is certified and currently running live on router hardware:

- **Daemon Version:** 🟢 **v16.0 Enterprise Firmware Upgrade Resilience Engine**
- **Service Status:** 🟢 **Active / Running** (Process PID `11762`)
- **Router Model:** GL.iNet Beryl 7 (GL-MT3600BE - Mediatek Filogic 820 ARM64, 512MB RAM)
- **Memory Footprint (`VmRSS`):** **`9.3 MB`** (9,552 KB - $< 2.0\%$ of 512MB RAM)
- **CPU Usage:** **`1.05%`** (Idle state on 5s loop)
- **Hardware Temperature:** **`60.07 °C`**
- **API Latency:** **`16.24 ms`**
- **Live Endpoint Verification:** [http://192.168.8.1:8888/api/health](http://192.168.8.1:8888/api/health) | [http://192.168.8.1:8888/metrics](http://192.168.8.1:8888/metrics)

---

## 🔑 Token Setup & RBAC Configuration Guide

### 1. Configure Environment File (`/etc/beryl7/agent.env`)
Create or edit the secure configuration file on the router with restricted permissions (`0600`):

```bash
cat <<EOF > /etc/beryl7/agent.env
AUTH_TOKEN=your_admin_secret_token
APPROVE_TOKEN=your_operator_approve_token
LOG_LEVEL=INFO
HEALTH_PORT=8888
BIND_HOST=127.0.0.1
GEMINI_API_KEY=your_gemini_api_key_here
EOF

chmod 0600 /etc/beryl7/agent.env
```

### 2. Verify RBAC Endpoints with `curl`

```bash
# 1. Unauthenticated / Viewer Access (Read-Only Status)
curl -s http://192.168.8.1:8888/api/health

# 2. Operator Access (Approve Action with Token)
curl -s -X POST http://192.168.8.1:8888/api/approve \
  -H "Authorization: Bearer $APPROVE_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"action": "purge_memory_cache"}'

# 3. Admin Access (Hot Config Reload)
curl -s -X POST http://192.168.8.1:8888/api/config/reload \
  -H "Authorization: Bearer $AUTH_TOKEN"
```

---

## 🧪 CI Gate & Verification

- **Unit Test Suite:** 100% PASS across all 10 Go packages (`ai`, `cmd`, `config`, `executor`, `logger`, `parser`, `skillstore`, `telemetry`, `tests`, `watchdog`).
- **Code Coverage Gate:** CI Workflow configured with strict quality gate enforcement (`.github/workflows/coverage.yml`).
