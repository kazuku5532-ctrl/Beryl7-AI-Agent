# Beryl 7 AI Agent 🛠️

An autonomous, self-healing, and self-optimizing network intelligence daemon designed for OpenWrt routers, certified and running live on **GL.iNet Beryl 7 (GL-MT3600BE)** hardware.

[![Go Version](https://img.shields.io/badge/Go-1.21-blue.svg)](https://golang.org)
[![Architecture](https://img.shields.io/badge/Target_Arch-ARM64_Linux-blue.svg)](https://openwrt.org)
[![Build Status](https://img.shields.io/badge/CI_Gate-100%25_PASS-emerald.svg)](https://github.com/kazuku5532-ctrl/Beryl7-AI-Agent)
[![Status](https://img.shields.io/badge/Production-Certified_Live-emerald.svg)](http://192.168.8.1:8888/api/health)

The project delivers a zero-dependency, native Go daemon (`/usr/bin/beryl7-agent`), an embedded HTTP management API on port `8888`, a standalone dashboard UI ([Beryl7_Dashboard_Standalone.html](dashboard/Beryl7_Dashboard_Standalone.html)), getting started documentation ([docs/GETTING_STARTED.md](docs/GETTING_STARTED.md)), and performance benchmark reports ([docs/benchmark.md](docs/benchmark.md)).

---

## 🏛️ The 7 Core Architectural Pillars

Beryl 7 AI Agent operates as a self-contained autonomous network intelligence system built around 7 core architectural pillars:

1. **⚡ Self-Optimizing Network:** Monitors real-time throughput and dynamically boosts Wi-Fi 7 channel width (e.g. 160MHz max performance) during heavy usage while restoring eco mode (80MHz) during low-traffic periods.
2. **🧬 Self-Evolving:** Embedded SQLite (`skills.db`) learning engine with Exponential Weighted Moving Average (EMA) confidence scoring that learns effective remediation actions and prunes underperforming ones.
3. **🦎 Self-Adaptive:** Dynamic firmware capability matrix detection supporting OpenWrt and GL.iNet releases (4.9.0, 5.0+), adapting ubus RPC calls and feature flags without code recompilation.
4. **🤖 AI-Powered (Gemini API Integration):** Cloud AI reasoning for unclassified system log anomalies, protected by token bucket rate limiters, circuit breaker safeguards, and daily budget caps ($1.00 USD).
5. **🛡️ Self-Securing:** Enterprise RBAC security with separate `AUTH_TOKEN` and `APPROVE_TOKEN` roles, restricted CORS origins, shell parameter sanitization (`shlex.quote`), and secure file permissions (`0600`/`0755`).
6. **🚑 Self-Healing:** Autonomous reactive remediation for WAN drops, memory exhaustion, Wi-Fi stalls, and latency spikes, protected by hardware watchdog guardrails and automated UCI rollback timers.
7. **🌊 Self-Smoothing:** EWMA latency smoothing, debounced metric collection, statistical Z-score anomaly detection, and asynchronous non-blocking post-action telemetry verification.

---

## 🟢 Certified Live Deployment Status (GL-MT3600BE)

The daemon is certified and running live on router hardware:

- **Daemon Version:** 🟢 **v16.0 Enterprise Firmware Upgrade Resilience Engine**
- **Service Status:** 🟢 **Active / Running** (Process PID `18095` - Verified Post Sudden Power Loss & Cold Reboot)
- **Router Model:** GL.iNet Beryl 7 (GL-MT3600BE - Mediatek Filogic 820 ARM64, 512MB RAM)
- **Memory Footprint (`VmRSS`):** **`7.5 MB`** (7,576 KB - $< 1.5\%$ of 512MB RAM)
- **CPU Usage:** **`0.90%`** (Idle state on 5s telemetry loop with Hardware Acceleration enabled)
- **Hardware Temperature:** **`58.70 °C`**
- **API Latency:** **`16.24 ms`**
- **Live Endpoint Verification:** [http://192.168.8.1:8888/api/health](http://192.168.8.1:8888/api/health) | [http://192.168.8.1:8888/metrics](http://192.168.8.1:8888/metrics)

---

## 🔑 Configuration & Environment Variables

### 1. Environment File (`/etc/beryl7/agent.env`)

Create or edit the secure environment file on the router with restricted file permissions (`0600`):

```bash
cat <<EOF > /etc/beryl7/agent.env
AUTH_TOKEN=your_admin_secret_token
APPROVE_TOKEN=your_operator_approve_token
LOG_LEVEL=INFO
HEALTH_PORT=8888
BIND_HOST=127.0.0.1
GEMINI_API_KEY=your_gemini_api_key_here

# Dynamic Anomaly Thresholds (Optional Overrides)
BERYL7_RAM_EXHAUSTION_PCT=92.0
BERYL7_CPU_SPIKE_LOAD=1.5
BERYL7_LATENCY_SPIKE_MS=100.0
BERYL7_LATENCY_ZSCORE=2.5
BERYL7_BANDWIDTH_BOOST_MBPS=80.0
BERYL7_BANDWIDTH_RESTORE_MBPS=20.0
BERYL7_WIFI_DISCONNECT_COUNT=3
EOF

chmod 0600 /etc/beryl7/agent.env
```

### 2. Verify RBAC Endpoints with `curl`

```bash
# 1. Unauthenticated / Viewer Access (Read-Only Health Check)
curl -s http://192.168.8.1:8888/api/health

# 2. Operator Access (Execute Remedial Action with Token)
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
- **Resilience Certification:** Certified against cold reboots, sudden power outages, and firmware preservation (`/etc/sysupgrade.conf`).
- **Code Coverage Gate:** CI Workflow configured with strict quality gate enforcement (`.github/workflows/coverage.yml`).
