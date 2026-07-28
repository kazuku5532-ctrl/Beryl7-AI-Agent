# Beryl 7 AI Agent — Enterprise Autonomous Network Operations Engine 🚀

[![Production Certified Grade A++](https://img.shields.io/badge/Production-Certified_Grade_A%2B%2B-emerald)](https://github.com/kazuku5532-ctrl/Beryl7-AI-Agent)
[![OpenWrt Native](https://img.shields.io/badge/Platform-OpenWrt_GL--MT3600BE-blue)](https://www.gl-inet.com/)
[![Go Agent](https://img.shields.io/badge/Daemon-Go_ARM64_v15.3-purple)](https://go.dev/)
[![OpenAPI 3.0](https://img.shields.io/badge/API-OpenAPI_3.0-green)](api/openapi.yaml)
[![Prometheus](https://img.shields.io/badge/Metrics-Prometheus_Ready-orange)](prometheus/prometheus.yml)

An enterprise-grade, autonomous self-healing AI network remediation engine and real-time operations dashboard specifically engineered for the **GL.iNet Beryl 7 (GL-MT3600BE)** OpenWrt Wi-Fi 7 Router.

---

## 📚 Enterprise Documentation Suite

The system includes a complete enterprise documentation architecture:

| Category | Document Link | Description |
| :--- | :--- | :--- |
| **Requirements & Limits** | [docs/nfr.md](file:///c:/Users/kazuk/Documents/Beryl7-AI-Agent/docs/nfr.md) | Non-Functional Requirements (NFR) & Latency Boundaries |
| **Capacity & Scalability**| [docs/capacity-planning.md](file:///c:/Users/kazuk/Documents/Beryl7-AI-Agent/docs/capacity-planning.md) | Load Tiers (20, 100, 500 clients) & Degradation Thresholds |
| **Compatibility** | [docs/compatibility-matrix.md](file:///c:/Users/kazuk/Documents/Beryl7-AI-Agent/docs/compatibility-matrix.md) | OpenWrt 23.x/24.x & Hardware Support Matrix |
| **Risk Management** | [docs/risk-register.md](file:///c:/Users/kazuk/Documents/Beryl7-AI-Agent/docs/risk-register.md) | STRIDE Risk Register, Probabilities & Mitigations |
| **SRE Error Budget** | [docs/error-budget.md](file:///c:/Users/kazuk/Documents/Beryl7-AI-Agent/docs/error-budget.md) | 99.9% Availability SLA & Burn Rate Alert Rules |
| **Release Pipeline** | [docs/release-pipeline.md](file:///c:/Users/kazuk/Documents/Beryl7-AI-Agent/docs/release-pipeline.md) | CI/CD Quality Gates, SBOM & Cosign Signatures |
| **Benchmarks** | [docs/benchmark.md](file:///c:/Users/kazuk/Documents/Beryl7-AI-Agent/docs/benchmark.md) | Statistical P95/P99 Empirical Test Benchmark Report |
| **Performance** | [docs/performance.md](file:///c:/Users/kazuk/Documents/Beryl7-AI-Agent/docs/performance.md) | Automated Performance Regression Guidelines |
| **Network Telemetry** | [docs/network-kpi.md](file:///c:/Users/kazuk/Documents/Beryl7-AI-Agent/docs/network-kpi.md) | Network SLI/SLO/SLA Targets & Conntrack Counters |
| **Security & Threat** | [docs/security.md](file:///c:/Users/kazuk/Documents/Beryl7-AI-Agent/docs/security.md) & [docs/threat-model.md](file:///c:/Users/kazuk/Documents/Beryl7-AI-Agent/docs/threat-model.md) | Security Control Architecture & STRIDE Analysis |
| **Architecture** | [docs/architecture.md](file:///c:/Users/kazuk/Documents/Beryl7-AI-Agent/docs/architecture.md) | C4 Component Diagrams, Sequence & State Machine |
| **Operations Playbook** | [docs/runbook.md](file:///c:/Users/kazuk/Documents/Beryl7-AI-Agent/docs/runbook.md) | Disaster Recovery & Reviewer 6 Question Answers |
| **API Contract** | [api/openapi.yaml](file:///c:/Users/kazuk/Documents/Beryl7-AI-Agent/api/openapi.yaml) | Official OpenAPI 3.0 Specification |

---

## 📊 Prometheus & Grafana Observability

- **Prometheus Endpoints:** `http://192.168.8.1:8888/metrics` & `http://localhost:5000/metrics`
- **Scrape Configuration:** [prometheus/prometheus.yml](file:///c:/Users/kazuk/Documents/Beryl7-AI-Agent/prometheus/prometheus.yml)
- **Alert Rules:** [prometheus/alerts.yml](file:///c:/Users/kazuk/Documents/Beryl7-AI-Agent/prometheus/alerts.yml)
- **Grafana Dashboard:** [grafana/beryl7_dashboard.json](file:///c:/Users/kazuk/Documents/Beryl7-AI-Agent/grafana/beryl7_dashboard.json)

---

## 🛠️ REST API Specification Summary

| Endpoint | Method | Description |
| :--- | :--- | :--- |
| `/api/health` | `GET` | System telemetry (CPU, RAM, Temp, Latency, Uptime, SLO) |
| `/api/modules/status` | `GET` | Health status of all 6 core architecture modules |
| `/api/logs` | `GET` | Live OpenWrt logread entries |
| `/metrics` | `GET` | Standard Prometheus text metrics exporter |
| `/api/approve` | `POST` | Operator approval for gated high-risk actions |

---

## 🚀 Quick Start & Local Preview

Open [Beryl7_Dashboard_Standalone.html](file:///c:/Users/kazuk/Documents/Beryl7_Dashboard_Standalone.html) in any modern browser or launch the Python web controller:
```bash
python agent/dashboard_server.py 5000
```
