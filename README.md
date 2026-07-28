# Beryl 7 AI Agent — Enterprise Autonomous Network Operations Dashboard 🚀

[![Production Grade](https://img.shields.io/badge/Production-Certified_Grade_A%2B%2B-emerald)](https://github.com/kazuku5532-ctrl/Beryl7-AI-Agent)
[![OpenWrt Native](https://img.shields.io/badge/Platform-OpenWrt_GL--MT3600BE-blue)](https://www.gl-inet.com/)
[![Go Agent](https://img.shields.io/badge/Daemon-Go_ARM64_v15.3-purple)](https://go.dev/)

An enterprise-grade, autonomous self-healing AI network remediation engine and real-time operations dashboard specifically engineered for the **GL.iNet Beryl 7 (GL-MT3600BE)** OpenWrt Wi-Fi 7 Router.

---

## 🌟 Key Capabilities & Features

### 1. 📊 Executive View & SLO Audit Matrix
- **Real-time Availability & MTTR KPIs:** Monitors system uptime, mean time to fix (< 1 ms local SQLite cache hit speed), and overall SLO compliance.
- **7-Day Trend Charting:** Visualizes system uptime and AI remediation success rate over time.
- **Autonomous Action Feed:** Live feed displaying the latest AI self-healing actions with risk confidence scores and source provenance.

### 2. ⚡ Technical View & Telemetry Stream
- **Module Health Grid (6/6 Core Modules):**
  - `Orchestrator Loop` (5s Priority Gating)
  - `Executor Engine` (Non-shell isolated UCI execution)
  - `Cloud AI Client` (Gemini 2.5 Flash with fallback)
  - `Watchdog Engine` (Selective UCI Checkpoint & Rollback)
  - `Log Parser` (Real `/sbin/logread` regex stream)
  - `Skill Store` (SQLite WAL Exponential Moving Average Learning)
- **Real Telemetry Gauges:** Dynamic CPU load, memory usage (aligned with GL.iNet Admin Panel formula), hardware thermal zone temperature, and ping latency.
- **Logread Stream & Terminal:** Filterable log reader with level filtering (ALL, INFO, WARN, ERROR), regex search, and page pagination (50 logs/page).

### 3. 🌐 Public Status Board
- A clean, executive-facing status page displaying operational status, 24-hour uptime, network latency, and active incident status.

### 4. ⚙️ Interactive Controls & Admin Settings
- **Persistent Admin Panel:** Configure Router API Host IP, Auth Token, and Telemetry Interval with automatic `localStorage` persistence.
- **Interactive Network Topology Map:** Visualizes network interfaces, Wi-Fi 7 radio status (2.4GHz & 5GHz), and native daemon state.
- **AI Decision Audit Log:** Audit trail of autonomous self-healing decisions.
- **Printable PDF Export:** Fully styled `@media print` layout for exporting formal operational reports.
- **Keyboard Navigation & Hotkeys:**
  - `1` : Executive View
  - `2` : Technical View
  - `3` : Status Board
  - `R` : Manual Refresh
  - `D` : Toggle Dark / Light Theme
  - `E` : Export CSV
  - `?` : Keyboard Shortcuts Help Modal

---

## 🛠️ System Architecture & API Specification

The dashboard communicates directly with either the **OpenWrt Native Go Daemon** or the **Python Local Controller Server**:

| Endpoint | Method | Description |
| :--- | :--- | :--- |
| `/api/health` | `GET` | System telemetry (CPU, RAM, Temp, Latency, Uptime, SLO) |
| `/api/modules/status` | `GET` | Health status of all 6 core architecture modules |
| `/api/logs` | `GET` | Live OpenWrt logread entries |
| `/api/metrics/history` | `GET` | Historical availability and success rate trends |
| `/api/cache/stats` | `GET` | SQLite Skill Store cache hit rate breakdown |

---

## 🚀 Quick Start & Local Preview

Open [dashboard/index.html](file:///c:/Users/kazuk/Documents/Beryl7-AI-Agent/dashboard/index.html) or the portable single-file bundle [Beryl7_Dashboard_Standalone.html](file:///c:/Users/kazuk/Documents/Beryl7_Dashboard_Standalone.html) in any modern web browser.

To run the Python local controller server on port 5000:
```bash
python agent/dashboard_server.py 5000
```
Then navigate to `http://localhost:5000`.
