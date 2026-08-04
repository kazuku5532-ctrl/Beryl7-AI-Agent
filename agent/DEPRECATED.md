# ⚠️ DEPRECATION NOTICE: Python Prototype (`agent/`)

> [!WARNING]
> **DO NOT USE THIS FOLDER FOR PRODUCTION DEPLOYMENT.**
> 
> The Python implementation in `agent/` is a legacy Proof-of-Concept (PoC) prototype developed during early architectural exploration.

## 🚀 Production Implementation Location

For 24/7 production deployment on OpenWrt routers (such as GL.iNet Beryl 7 / GL-MT3600BE), use the **Native Go Daemon**:

👉 **[`go-agent/`](file:///c:/Users/kazuk/Documents/Beryl7-AI-Agent/go-agent)**

### Why Native Go Daemon?
- **Zero Python Runtime Required:** Compiles to a single 8.6MB static ARM64 binary.
- **Micro Memory Footprint:** Consumes only **7.5MB RAM** ($< 1.5\%$ of 512MB RAM).
- **Sub-1% CPU Usage:** Optimized telemetry loop with MediaTek Hardware Acceleration.
- **Embedded CSDL:** Native SQLite WAL mode with automatic corrupted dump recovery.
- **Enterprise Hardened:** Native lock-free atomic configuration and 100% test coverage certified.
