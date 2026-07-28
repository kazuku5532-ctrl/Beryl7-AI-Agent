# Non-Functional Requirements (NFR) Specification 📐

System: **Beryl 7 AI Agent — Autonomous Network Operations Engine**  
Document Version: **v1.0.0**  
Target Platform: **OpenWrt GL-MT3600BE (Filogic 820 Quad-Core ARM64)**

---

## 1. Performance & Latency Requirements

| Metric | Target Boundary | Strict Maximum | Verification Method |
| :--- | :--- | :--- | :--- |
| **Local Skill Store Lookup** | $< 0.5 \text{ ms}$ | $< 2.0 \text{ ms}$ | SQLite WAL benchmark ($N=1000$) |
| **Cloud AI Decision Latency** | $< 350 \text{ ms}$ | $< 1000 \text{ ms}$ | Gemini 2.5 Flash API RTT |
| **Watchdog UCI Rollback** | $< 800 \text{ ms}$ | $< 2000 \text{ ms}$ | Automatic rollback timer |
| **Telemetry Scraping Rate** | $5.0 \text{ s}$ cycle | $1.0 \text{ s}$ cycle | Main loop ticker |

---

## 2. Resource Footprint Requirements

| Component | Target Allocation | Hard Boundary Limit | Behavior on Limit Breach |
| :--- | :--- | :--- | :--- |
| **Go Daemon RAM** | $< 35 \text{ MB}$ | $< 64 \text{ MB}$ | Go GC forced sweep |
| **SQLite WAL Storage** | $< 5 \text{ MB}$ | $< 15 \text{ MB}$ | Automatic VACUUM & checkpoint |
| **CPU Load** | $< 1.5\%$ | $< 5.0\%$ | Main loop throttling |
| **Disk Write Rate** | $< 100 \text{ KB/day}$ | $< 1 \text{ MB/day}$ | RAM ring buffer logging |

---

## 3. Availability & Reliability Requirements

* **System Availability SLA:** $99.9\%$ uptime ($< 43.8 \text{ minutes}$ downtime/month).
* **Mean Time to Recovery (MTTR):** $< 1.0 \text{ s}$ using local skill cache hit or Watchdog rollback.
* **Failure Isolation:** $100\%$ non-shell execution isolation for UCI configuration changes.
* **Offline Resilience:** System must remain $100\%$ operational offline using local EMA learned skill cache when WAN or Cloud API is unreachable.
