# CHANGELOG — Beryl 7 AI Agent 📜

All notable changes to this project will be documented in this file.

---

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
