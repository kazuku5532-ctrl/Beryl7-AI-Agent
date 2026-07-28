# Beryl 7 AI Agent — Enterprise Autonomous Network Operations Engine 🚀

[![Production Certified Grade A++](https://img.shields.io/badge/Production-Certified_Grade_A%2B%2B-emerald)](https://github.com/kazuku5532-ctrl/Beryl7-AI-Agent)
[![OpenWrt Native](https://img.shields.io/badge/Platform-OpenWrt_GL--MT3600BE-blue)](https://www.gl-inet.com/)
[![Go Agent](https://img.shields.io/badge/Daemon-Go_ARM64_v15.3-purple)](https://go.dev/)
[![Prometheus Ready](https://img.shields.io/badge/Metrics-Prometheus_Ready-orange)](go-agent/)

An enterprise-grade, autonomous self-healing AI network remediation engine and real-time operations dashboard specifically engineered for the **GL.iNet Beryl 7 (GL-MT3600BE)** OpenWrt Wi-Fi 7 Router.

---

## 🌟 Architecture & Technical Specifications

The system repository is structured into a streamlined, high-performance architecture:

| Component | Path | Description |
| :--- | :--- | :--- |
| **Go Native Daemon** | [go-agent/](go-agent/) | Compiled ARM64 daemon (`beryl7-agent`), Prometheus `/metrics` & REST API |
| **Python Controller** | [agent/](agent/) | Python HTTP controller server on port 5000 |
| **Operations Dashboard** | [dashboard/](dashboard/) | HTML5 Glassmorphism UI & [Standalone HTML Bundle](Beryl7_Dashboard_Standalone.html) |
| **Architecture Guide** | [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | C4 Diagrams, Sequence Flow, State Machine, Security & Threat Model |
| **Operations Runbook** | [docs/OPERATIONS.md](docs/OPERATIONS.md) | SRE Error Budget, SLA Targets, Capacity Planning & Disaster Recovery |
| **Empirical Benchmarks** | [docs/benchmark.md](docs/benchmark.md) | Statistical P50/P95/P99 empirical test benchmark report |

---

## ⚡ Empirical Benchmark Performance

Empirical metrics measured directly from the running Go Agent Daemon on the **GL-MT3600BE Router (PID `4021`)**:

- **RAM Footprint (`VmRSS`):** **13.08 MB** (~ 2.5% of 512MB RAM)
- **CPU Utilization:** **1.20% CPU** (> 98.8% CPU headroom)
- **Hardware Temperature:** **59.54°C** (Cool thermal zone)
- **API Response Latency:** **29.0 ms**
- **SQLite Local Skill Hit:** **0.38 ms (P99)**

---

## 📡 REST & Prometheus Metrics API

| Endpoint | Method | Description |
| :--- | :--- | :--- |
| `/api/health` | `GET` | System health telemetry (CPU, RAM, Temp, Latency, Uptime, SLO) |
| `/api/modules/status` | `GET` | Health status of all 6 core architecture modules |
| `/api/logs` | `GET` | Live OpenWrt logread entries |
| `/metrics` | `GET` | Standard Prometheus text metrics exporter |
| `/api/approve` | `POST` | Operator approval for gated high-risk actions |

---

## 🚀 Quick Start & Local Preview

Open [Beryl7_Dashboard_Standalone.html](Beryl7_Dashboard_Standalone.html) in any modern browser or launch the Python web controller:
```bash
python agent/dashboard_server.py 5000
```
