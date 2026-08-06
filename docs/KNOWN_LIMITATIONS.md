# Known Scope & Technical Limitations ⚠️

This document outlines the architectural boundaries and known technical scope limitations of the Beryl 7 AI Agent (`v16.0`).

---

## 1. Networking & Protocol Scope

- **IPv4 Focus:** Primary telemetry collection, WAN auto-healing, and DNS ping diagnostics target IPv4 interfaces (`wan`, `eth0`, `ra0`, `rai0`). Advanced IPv6 SLAAC / DHCPv6 prefix delegation is out of current scope.
- **Single-Node Gateway Topology:** Designed specifically for single-router Access Point / Gateway nodes (such as GL.iNet Beryl 7 / GL-MT3600BE). Multi-hop nested Wi-Fi mesh topologies are not supported.

---

## 2. Hardware & Operating System Constraints

- **Hardware Minimums:** Requires an ARMv7 / ARMv8 router with at least **128MB RAM** and **16MB flash storage**.
- **Firmware Requirement:** Engineered exclusively for OpenWrt 21.02+ (certified on GL.iNet v4.9.0 / v5.0+ and OpenWrt 24.10). Not compatible with desktop Linux, x86 servers, or containerized Kubernetes deployments.

---

## 3. Storage & AI Capabilities

- **SkillStore Storage Limits:** Embedded SQLite WAL mode operates with a default 10,000 row cap to prevent disk wear on flash storage.
- **Offline Fallback:** When WAN connectivity is completely lost, Cloud AI (Gemini API) cannot be reached. The daemon relies on local SQLite skill execution ($0.4\text{ms}$) or local emergency escalation (WAN reset, DHCP rebind, cache flush).
