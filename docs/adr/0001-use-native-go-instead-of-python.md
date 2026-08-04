# ADR-0001: Use Native Go Daemon Instead of Python Runtime

## Status
**Accepted**

## Context
Early prototypes used Python 3.11 with Paramiko and Google GenAI SDKs. On resource-constrained OpenWrt routers (such as GL.iNet Beryl 7 with 512MB RAM and 16MB flash), the Python interpreter consumed over **45MB RAM** baseline and required multiple heavy shared libraries (`libpython`, `cffi`, `cryptography`).

## Decision
Migrate the entire runtime to a **Native Go Daemon** (`go-agent/cmd/main.go`).

## Consequences
- **Positive:** RAM footprint reduced from 45MB to **7.5MB** ($< 1.5\%$ of RAM). Binary compiles to a single 8.6MB static ELF binary with zero external OS dependencies.
- **Negative:** Requires Go cross-compilation pipeline for ARM64 targets.
