# Operational Runbook & Disaster Recovery Playbooks 📖

System: **Beryl 7 AI Agent**  
Document Version: **v1.0.0**

---

## 1. Answers & Recovery Playbooks for the 6 Reviewer Questions

### Q1: Why did AI choose a specific action?
- **Diagnostic Procedure:** Inspect `/var/log/beryl7_approval_audit.log` and telemetry log stream. AI choices are logged with reasoning text, input log snippet, and confidence score.
- **Verification Command:** `journalctl -u beryl7-agent | grep "Gemini AI Action Approved"`

### Q2: What if AI makes a wrong decision?
- **Safety Mechanism:** All AI actions require `Confidence >= Risk Threshold` (e.g., WAN reset requires 0.85). If confidence is below threshold, action is queued for Operator Approval at `/var/run/beryl7_pending_approval.json`.
- **Manual Override:** Execute `uci revert network && uci commit network && /etc/init.d/network reload`.

### Q3: What if rollback fails?
- **Safety Mechanism:** Watchdog executes dual-layer recovery:
  1. Primary: `uci import < /tmp/agent_checkpoint.uci && uci commit`
  2. Secondary Fallback: Force default WAN DHCP (`uci set network.wan.proto=dhcp && uci commit network && /etc/init.d/network reload`).

### Q4: What if the SQLite database corrupts?
- **Safety Mechanism:** Skill Store automatically detects SQLite read errors, isolates corrupted DB to `/var/lib/beryl7/skills_corrupt.db`, and initializes a fresh database from the backup dump at `/tmp/beryl7_skills_backup.sql`.

### Q5: What if WAN flaps repeatedly?
- **Safety Mechanism:** Exponential Backoff & Damping Window. If WAN drops > 3 times within 60 seconds, remediation enters a 30-second cooldown window to prevent flap storms.

### Q6: What if the agent process crashes?
- **Safety Mechanism:** OpenWrt `procd` service manager automatically restarts the binary (`respawn`). On startup, `acquirePIDLock` cleans stale `/var/run/beryl7.pid` files cleanly.
