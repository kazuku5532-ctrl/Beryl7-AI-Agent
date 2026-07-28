# ADR 0001: Native Go Agent Daemon Architecture 📝

Status: **Accepted**  
Date: **2026-07-28**

---

## Context
OpenWrt router hardware (GL-MT3600BE) operates under constrained memory (512MB RAM) and requires low latency, single-binary deployment with zero heavy external dependencies.

## Decision
Build the core router daemon using compiled **Go (`linux/arm64`)** rather than Python or Node.js.

## Consequences
- Single statically-linked ~14MB binary with zero runtime dependencies.
- Sub-millisecond execution latency and low RAM footprint (< 35MB).
