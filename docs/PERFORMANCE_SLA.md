# Performance Service Level Agreement (SLA) & Latency Targets ⚡

---

## 1. System Response Time & Latency Targets

| Operations & Endpoints | Target P50 Latency | Target P95 Latency | Target P99 Latency | Measured Production |
| :--- | :---: | :---: | :---: | :---: |
| **SQLite Skill Store Lookup** | $\le 0.5\text{ ms}$ | $\le 1.0\text{ ms}$ | $\le 2.0\text{ ms}$ | **$0.40\text{ ms}$** |
| **HTTP Health API (`/api/health`)** | $\le 20.0\text{ ms}$ | $\le 35.0\text{ ms}$ | $\le 50.0\text{ ms}$ | **$16.24\text{ ms}$** |
| **Prometheus Scrape (`/metrics`)** | $\le 5.0\text{ ms}$ | $\le 10.0\text{ ms}$ | $\le 15.0\text{ ms}$ | **$3.10\text{ ms}$** |
| **Local Anomaly Auto-Healing** | $\le 1.0\text{ s}$ | $\le 2.0\text{ s}$ | $\le 3.0\text{ s}$ | **$0.85\text{ s}$** |
| **Cloud AI Analysis (Gemini API)** | $\le 300\text{ ms}$ | $\le 600\text{ ms}$ | $\le 1200\text{ ms}$ | **$280.0\text{ ms}$** |

---

## 2. Resource Consumption Bounds

| Resource Parameter | SLA Upper Bound | Measured Baseline | Action on Violation |
| :--- | :---: | :---: | :--- |
| **RAM Footprint (`VmRSS`)** | $\le 16.0\text{ MB}$ | **`7.5 MB`** | Force GC & restart process if $> 32\text{MB}$ |
| **CPU Load (Idle 5s Ticker)** | $\le 2.0\%$ | **`0.90%`** | Back off telemetry ticker interval |
| **Static Binary Storage** | $\le 12.0\text{ MB}$ | **`8.6 MB`** | Binary stripping (`-s -w` flags) |

---

## 3. Availability & Recovery SLO

- **System Availability Target:** **99.95%** uptime.
- **Watchdog Rollback Deadline:** **30 seconds** max dead man's switch trigger.
- **Auto-Healing Recovery Success Target:** **$\ge 95.0\%$** resolution rate.
