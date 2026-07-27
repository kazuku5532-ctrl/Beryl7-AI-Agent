# 🛠️ Production Troubleshooting & Diagnostic Guide

---

## 1. Skill Store Cache Misses (Always Calling Gemini AI)
* **Symptom:** Every anomaly triggers a Cloud AI request; SQLite skill cache hits remain 0.
* **Diagnosis:**
  1. Check database write permissions: `sqlite3 /tmp/skills.db "PRAGMA integrity_check;"`
  2. Verify confidence threshold in `store.go`: Skills require `confidence >= 0.85` for local execution.
* **Resolution:**
  - Verify `EMAAlpha` scoring is updating confidence after successful actions.
  - Check `/var/log/beryl7_agent.log` for database lock warnings.

---

## 2. Watchdog Rollback Triggered Repeatedly
* **Symptom:** Router configuration rolls back automatically within 30 seconds of an action.
* **Diagnosis:**
  1. Review system log sample in `/var/log/messages`.
  2. Verify `/tmp/agent_checkpoint.uci` snapshot state.
* **Resolution:**
  - Check if target MAC or interface name in UCI action was misconfigured.
  - Verify network cable or physical WAN link is stable.

---

## 3. Gemini API HTTP 429 Rate Limiting
* **Symptom:** Log shows `Gemini API HTTP 429 Rate Limited! Backing off for 5s...`
* **Diagnosis:** Multiple anomalies occurring in short succession exceeding quota.
* **Resolution:**
  - Circuit Breaker will open after 5 consecutive failures for 5 minutes.
  - Rely on SQLite Skill Store local execution ($\ge 0.85$ confidence) which has 0 API cost.

---

## 4. HTTP Health Endpoint Port 8888 Unreachable
* **Symptom:** `curl http://192.168.8.1:8888/api/health` returns `Connection Refused` or `401 Unauthorized`.
* **Diagnosis:**
  1. Check process alive: `pgrep beryl7-agent`
  2. Check firewall rule: `uci show firewall.beryl7_health`
* **Resolution:**
  - Pass valid `Authorization: Bearer <AUTH_TOKEN>` header.
  - Ensure request originates from LAN zone (`192.168.8.0/24`).
