# ADR-0002: Use Embedded SQLite in WAL Mode Instead of Client-Server Database

## Status
**Accepted**

## Context
The agent requires a persistent store for local skill caching, anomaly logs, and confidence scores. Client-server databases (like PostgreSQL or MySQL) require dedicated server processes, high memory overhead, and network IPC setup.

## Decision
Use `modernc.org/sqlite` embedded CGO-free driver with **Write-Ahead Logging (WAL)** mode enabled (`PRAGMA journal_mode = WAL;`).

## Consequences
- **Positive:** Zero client-server process overhead, fast sub-millisecond lookups ($0.4\text{ms}$), automatic atomic crash resilience, and CGO-free static cross-compilation.
- **Negative:** Concurrency limited to single-writer multi-reader pattern (mitigated by Go mutex locks).
