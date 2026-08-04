# 🗺️ Product Roadmap & Release Strategy (v16.0 ➔ v18.0)

---

## 🟢 v16.0 Enterprise Hardened Release (Current — Q3 2026)
- [x] **100% Native Go Daemon Migration:** Zero-dependency static compilation for ARM64 Linux.
- [x] **SQLite Skill Store WAL Mode:** Embedded zero-lock skill store with Exponential Weighted Moving Average (EMA) scoring.
- [x] **Lock-Free Atomic Config Engine:** `atomic.Value` configuration hot-reloading via `/api/config/reload`.
- [x] **Comprehensive Prometheus & Grafana Metric Suite:** 12 exported metrics with 10-panel Grafana Dashboard.
- [x] **Watchdog Dead Man's Switch & Fallback:** 30s background shell watcher with active 15s health check loop.
- [x] **Security & Quality Compliance:** 88.4% test coverage, rate-limiting, RBAC tokens, and SPDX 2.3 SBOM.

---

## 🟡 v17.0 Multi-Node Mesh Resilience Engine (Planned — Q4 2026)
- [ ] **Distributed Skillstore Synchronization:** Peer-to-peer (P2P) synchronization of high-confidence remediation skills across OpenWrt Mesh nodes.
- [ ] **eBPF Packet Telemetry:** Kernel-level eBPF packet inspection for zero-overhead micro-burst detection.
- [ ] **OAuth2 / OIDC Single Sign-On:** Integration with Keycloak / Authentik for enterprise operator authentication.
- [ ] **Dynamic AI Model Failover:** Automatic fallback between Gemini 2.5 Flash, Gemini 2.5 Pro, and local llama.cpp endpoints.

---

## 🔵 v18.0 Autonomous AI Self-Tuning Engine (Planned — Q1 2027)
- [ ] **Reinforcement Learning Agent (RL):** On-device RL model tuning channel width and transmit power per client device.
- [ ] **Automated Dynamic Firmware Patching:** Self-testing firmware upgrades with live kexec rollback capability.
- [ ] **Multi-WAN Intelligent Load Balancing:** Real-time latency & packet loss steering across Starlink, 5G cellular, and Fiber WANs.
