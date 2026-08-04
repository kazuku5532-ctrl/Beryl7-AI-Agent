# 💡 Operational Examples & Real-World Use Cases

---

## 1. Wi-Fi Bandwidth Boost Scenario
When high WAN bandwidth usage is detected ($> 80\text{ Mbps}$), the agent boosts 5GHz channel width to 160MHz:

```bash
# Verify live status
curl http://192.168.8.1:8888/api/health
```

---

## 2. WAN Connection Loss Auto-Healing
When WAN status changes to `Offline`, the agent:
1. Creates UCI checkpoint `/tmp/agent_checkpoint.uci`.
2. Restarts WAN interface via `ubus call network.interface.wan restart`.
3. Verifies ping connectivity to `1.1.1.1` within 15s.

---

## 3. High RAM Memory Pressure Mitigation
When system RAM exceeds 92%, the agent drops Linux kernel caches (`echo 3 > /proc/sys/vm/drop_caches`) and flushes stale conntrack entries.
