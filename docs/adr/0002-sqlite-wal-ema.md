# ADR 0002: SQLite WAL Storage with EMA Learning 📝

Status: **Accepted**  
Date: **2026-07-28**

---

## Context
Autonomous AI remediations must be learned and executed locally without relying indefinitely on remote Cloud AI API calls.

## Decision
Use an embedded **SQLite database in WAL (Write-Ahead Logging) mode** coupled with an **Exponential Moving Average (EMA)** algorithm for local skill confidence scoring.

## Consequences
- Sub-millisecond (< 0.5 ms) local skill lookup speed.
- Offline autonomous self-healing capability when Cloud AI or WAN is down.
