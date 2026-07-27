# 🛡️ OpenWrt Autonomous Remediation Agent for GL.iNet Beryl 7 (GL-MT3600BE)

> **An OpenWrt-Native Daemon for Automated Anomaly Remediation, Health Monitoring, and AI-Assisted Diagnostics**

![Go Version](https://img.shields.io/badge/Go-1.21%2B-00ADD8?style=flat&logo=go)
![OpenWrt Version](https://img.shields.io/badge/OpenWrt-21.02-green)
![AI Diagnostics](https://img.shields.io/badge/AI--Diagnostics-Google%20Gemini%202.5%20Flash-orange)
![State Security](https://img.shields.io/badge/State--Safety-UCI%20Checkpoint%20Watchdog-red)
![License](https://img.shields.io/badge/license-MIT-blue)

---

## 💡 System Architecture (Dual-Execution Model)

**GL.iNet Beryl 7 Autonomous Remediation Agent** is an embedded background daemon designed to monitor network health, detect operational anomalies, and execute automated recovery actions directly on OpenWrt Linux (`aarch64`).

The system implements a **Dual-Execution Architecture**:

1. **Native On-Router Daemon (Primary Execution):** A statically compiled Go binary (`beryl7-agent`, 9.44 MB) operating as an OpenWrt `procd` system service. It executes continuously in the background without external controller dependencies.
2. **Offloaded Controller Pipeline (Secondary / Verification):** A host-side Python controller used for remote deployment, verification drills, and benchmarking.

```text
       ┌────────────────────────────────────────────────────────┐
       │   GL.iNet Beryl 7 (Filogic 850 / OpenWrt 21.02)         │
       │   Native Go Daemon (/usr/bin/beryl7-agent)             │
       └───────────────────────────┬────────────────────────────┘
                                   │ Local Syscall & ubus IPC
                                   ▼
       ┌────────────────────────────────────────────────────────┐
       │        Go Anomaly Remediation Engine                   │
       └─────┬────────────────────────────────────────────┬─────┘
             │                                            │
   (Skill Cache Hit: < 1ms)                    (Cache Miss / Low Confidence)
             ▼                                            ▼
┌─────────────────────────┐                  ┌─────────────────────────┐
│ Pure-Go SQLite Cache    │                  │ Google Gemini 2.5 Flash │
│ (WAL Mode & Delta EMA)  │                  │ Function Calling API    │
└────────────┬────────────┘                  └────────────┬────────────┘
             │                                            │
             └─────────────────────┬──────────────────────┘
                                   ▼
       ┌────────────────────────────────────────────────────────┐
       │ UCI Checkpoint Snapshot & Watchdog Guardrail          │
       └────────────────────────────────────────────────────────┘
```

---

## 🔬 Core Technical Specifications

1. **Native Embedded Execution:**
   - Single static binary (9.44 MB) target compiled for `linux/arm64` (MediaTek Filogic 850 Quad-Core).
   - Minimal resource footprint (~9.44 MB RSS Memory, < 1% CPU utilization under normal telemetry sampling).
   - Process lifecycle managed via OpenWrt `procd` init script (`/etc/init.d/beryl7-agent`).

2. **Adaptive Skill Cache (Exponential Moving Average Scoring):**
   - Embedded Pure-Go SQLite database (`modernc.org/sqlite`) running in WAL mode.
   - Skill confidence evaluation updated dynamically via Delta EMA formula: $C_{new} = C_{old} + \alpha \cdot (Target - C_{old})$.
   - Local remediation execution in `< 1ms` upon skill cache hit ($\ge 0.85$ confidence threshold).

3. **AI-Assisted Diagnostics Engine:**
   - Cloud diagnostic fallback to Google Gemini 2.5 Flash REST API authenticated via `x-goog-api-key`.
   - Circuit breaker pattern (opens after 5 consecutive API errors, 5-minute cooldown) and token-bucket rate limiting.

4. **UCI State Integrity Watchdog:**
   - Pre-execution configuration backup to `/tmp/agent_checkpoint.uci`.
   - Automatic configuration rollback upon link recovery failure or system state deterioration.
   - Safe Mode state machine requiring 3 consecutive successful health checks (90s window) prior to normal operation resumption.

5. **Security Gating & System Hardening:**
   - Atomic PID lock (`/var/run/beryl7-agent.pid`) with Unix signal verification.
   - Linux Out-Of-Memory score adjustment (`OOM_SCORE_ADJ = -500`) to prevent process termination under memory pressure.
   - Authenticated HTTP health endpoint (`:8888`) with constant-time Bearer token comparison, 10-minute approval expiration, and per-IP rate limiting (30 req/min).
   - Per-anomaly cooldown windows (WAN Drop: 90s, RAM Exhaustion: 45s, Wi-Fi Anomaly: 60s) to prevent remediation loops.
   - Idempotent OpenWrt UCI named section configuration (`firewall.block_<mac_hex>`) to prevent flash memory degradation.

---

## 🛠️ Deployment & Operations

### 1. System Requirements
* Target Router: GL.iNet GL-MT3600BE (Beryl 7) running OpenWrt 21.02 (`192.168.8.1`).
* Build Environment: Go 1.21+ or Python 3.10+ (for automated deployment scripts).

---

### 2. Automated Deployment Pipeline

Connect to the router network and execute the deployment automator:

#### 🔹 Python Automator:
```powershell
.\venv\Scripts\python scripts/deploy_to_router.py
```

#### 🔹 PowerShell Automator:
```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\deploy_to_router.ps1
```

The script performs the following steps:
1. Cross-compiles the ARM64 static binary `bin/beryl7-agent`.
2. Uploads the binary and `procd` service script `/etc/init.d/beryl7-agent` via SSH/SFTP.
3. Configures OpenWrt firewall rules (`Allow-Beryl7-Health-LAN`) to restrict port 8888 access to the LAN zone.
4. Starts and verifies the background daemon service.

---

### 3. Verification & Diagnostic Testing

Run the 5-stage automated verification suite:

```powershell
.\venv\Scripts\python scripts/verify_framework.py
```

Execute adaptive skill evaluation and EMA scoring drills:

```powershell
.\venv\Scripts\python scripts/test_evolution_drill.py
```

---

## 📁 Repository Layout

```text
Beryl7-AI-Agent/
├── bin/
│   └── beryl7-agent           # ARM64 static Go binary (9.44 MB)
├── go-agent/                  # Native Go Remediation Daemon (Primary)
│   ├── cmd/
│   │   ├── main.go            # Daemon entrypoint & HTTP server (:8888)
│   │   ├── sys_linux.go       # OpenWrt Linux build tags & OOM score setting
│   │   └── sys_windows.go     # Windows testing build tags
│   ├── config/config.go       # Configuration loader & file permissions (0600)
│   ├── telemetry/telemetry.go # Metric collection & /proc/net/dev bandwidth delta
│   ├── parser/parser.go       # Syslog parser & rate limiter
│   ├── ai/ai_client.go        # Gemini 2.5 Flash client & circuit breaker
│   ├── executor/executor.go   # OpenWrt UCI command executor & fallback channels
│   ├── watchdog/watchdog.go   # UCI checkpoint export/import & safe mode state
│   ├── skillstore/store.go    # Pure-Go SQLite store & EMA scoring engine
│   ├── logger/logger.go       # Rotating log writer
│   ├── tests/                 # Go unit test suite
│   └── procd/beryl7-agent     # OpenWrt procd init script
├── agent/                     # Python offloaded controller (Secondary)
├── scripts/                   # Deployment and testing tooling
│   ├── deploy_to_router.py    # Automated SSH deployment script
│   ├── build_go_binary.ps1    # Cross-compilation script for ARM64
│   ├── verify_framework.py    # 5-stage verification framework
│   ├── test_evolution_drill.py# EMA skill scoring drill script
│   ├── check_health.py        # Health endpoint validation script
│   └── get_temp.py            # Real-time hardware temperature reader
├── docs/                      # Technical documentation & benchmark reports
└── README.md                  # Documentation
```

---

## 📄 License
Distributed under the [MIT License](LICENSE).
