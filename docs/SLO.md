# 📈 Service Level Objectives (SLO) & Alert Definitions

---

## Service Level Objectives (SLOs)

| SLO Metric | Target Objective | Measurement Method | Alert Threshold |
| :--- | :---: | :--- | :--- |
| **System Availability** | **99.5%** | Percentage of time `/api/health` returns HTTP 200 | `< 99.0%` over 1 hour window |
| **Cache Hit Rate** | **> 85.0%** | `beryl7_cache_hits_total / (hits + misses)` | `< 70.0%` over 30 min window |
| **Remediation Speed (MTTR)** | **< 100 ms (Cache)** | Latency from anomaly detection to UCI reload | `> 500 ms` for local cache |
| **Watchdog Rollback Rate** | **< 2.0%** | `rollbacks / total_actions_executed` | `> 5.0%` over 1 hour window |
| **False Positive Rate** | **< 5.0%** | Unnecessary action executions on healthy state | `> 10.0%` over 24 hour window |

---

## Alert Definitions & Escalation Matrix

1. **RouterUnreachable (Critical):**
   - Condition: `beryl7_router_reachable == 0` for 5 minutes.
   - Action: Check physical router power, WAN/LAN cable connectivity.
2. **LowCacheHitRate (Warning):**
   - Condition: Cache Hit Rate $< 70\%$ for 15 minutes.
   - Action: Inspect SQLite Skill Store for unlearned novel anomalies.
3. **HighRAMConsumption (Warning):**
   - Condition: `beryl7_system_ram_usage_percent > 90%` for 5 minutes.
   - Action: Daemon triggers `purge_memory_cache` (`sysctl -w vm.drop_caches=3`).
