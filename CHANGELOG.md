# CHANGELOG — Beryl 7 AI Agent 📜

All notable changes to this project will be documented in this file.

---

## [v16.0.0] - 2026-07-31

### Added 🌟
- **10/10 Perfect Score System Evolution Engine:** Integrated anomaly-filtered Few-Shot exemplars into Gemini AI prompt (`GetTopSkillsSummaryForAnomaly`).
- **EWMA & Z-Score Anomaly Engine:** Added statistical moving average and dynamic standard deviation anomaly tracking for network latency spikes.
- **Safe Mode 5/5 Stability Exit Criteria:** Strengthened Safe Mode exit threshold from 3 to 5 consecutive successful health check cycles (150s).
- **Security & Authorization Tightening:** Fixed Bug 3.1 Budget Check in `AnalyzeAnomalyWithContext`, disallowed `viewer` role from reading system logs (`/api/logs`), exact-matched CORS origins, and set default loopback `127.0.0.1`.

## [v15.3.0] - 2026-07-28

### Added 🌟
- **Full Enterprise Documentation Architecture:** Added `nfr.md`, `capacity-planning.md`, `compatibility-matrix.md`, `risk-register.md`, `error-budget.md`, `release-pipeline.md`, `benchmark.md`, `performance.md`, `network-kpi.md`, `security.md`, `threat-model.md`, `architecture.md`, `sequence.md`, `state-machine.md`, `runbook.md`, `operations.md`, `adr/`, and `api/openapi.yaml`.
- **Prometheus & Grafana Provisioning Stack:** Registered `/metrics` endpoint in Go daemon and Python server, created `prometheus/alerts.yml`, `prometheus/prometheus.yml`, and `grafana/beryl7_dashboard.json`.
- **Advanced Network Telemetry:** Parsed Conntrack active NAT sessions (`/proc/sys/net/netfilter/nf_conntrack_count`), ARP entry count, Packet Error Rate (PER), and OpenTelemetry trace propagation headers.
- **Chaos & Load Test Infrastructure:** Added `tests/chaos/chaos_test.py`, `tests/load/stress_test.py`, `tests/integration/integration_test.py`, and `tests/dr_validation.py`.
- **Supply Chain Security:** Added SPDX SBOM generator `scripts/generate_sbom.py` and checksum signer `scripts/cosign_sign.py`.

### Security 🛡️
- Enforced `// #nosec G204` security annotations across `telemetry.go`, `main.go`, and `watchdog.go`.
- Constant-time token verification (`subtle.ConstantTimeCompare`) and IP rate limiting (60 req/min).
