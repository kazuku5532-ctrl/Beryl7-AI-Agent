# 🛠️ Comprehensive Production Troubleshooting & Diagnostic Guide

---

## 1. Daemon Won't Start (Process Immediate Exit)
* **Symptoms:** Running `/etc/init.d/beryl7-agent start` exits immediately; PID file is missing.
* **Diagnosis:**
  1. Check execution permissions: `ls -la /usr/bin/beryl7-agent` (must be `chmod +x`).
  2. Check config file permissions: `ls -la /etc/beryl7/agent.env` (must be `chmod 0600`).
  3. Check validation error output by running manually: `/usr/bin/beryl7-agent -config /etc/beryl7/agent.env`
  4. Check if port 8888 is bound by another service: `netstat -tlpn | grep 8888`
* **Resolution:**
  - Generate missing tokens using `openssl rand -hex 32` and write to `/etc/beryl7/agent.env`.
  - Fix permissions: `chmod +x /usr/bin/beryl7-agent && chmod 0600 /etc/beryl7/agent.env`.
  - Kill stale process using port 8888: `fuser -k 8888/tcp`.

---

## 2. High CPU / Memory Leak Profiling
* **Symptoms:** Daemon memory footprint grows beyond 16MB or CPU usage stays above 2%.
* **Diagnosis:**
  1. Inspect live memory: `top -p $(pgrep beryl7-agent)`
  2. Check OpenWrt logread for goroutine leak warnings: `logread | grep beryl7-agent`
* **Resolution:**
  - Expected baseline is **7.5MB RAM** and **<1.0% CPU**.
  - Restart service to clear stale buffers: `/etc/init.d/beryl7-agent restart`.
  - Verify SQLite WAL autocheckpoint is running: `sqlite3 /root/skills.db "PRAGMA wal_autocheckpoint;"`.

---

## 3. Database Corruption & Manual WAL Recovery
* **Symptoms:** Log shows `CRITICAL SQLITE INTEGRITY FAILURE`.
* **Diagnosis:**
  1. Run manual integrity check: `sqlite3 /root/skills.db "PRAGMA integrity_check;"`
* **Resolution:**
  - The daemon automatically exports a corrupted dump to `/root/skills.db.corrupt.<timestamp>.sql` and restores from `/root/skills.db.bak`.
  - To manually rebuild skillstore:
    ```bash
    rm -f /root/skills.db /root/skills.db-wal /root/skills.db-shm
    cp /root/skills.db.bak /root/skills.db
    /etc/init.d/beryl7-agent restart
    ```

---

## 4. Circuit Breaker Stuck OPEN
* **Symptoms:** Log shows `Circuit Breaker is OPEN! AI calls blocked.`
* **Diagnosis:** 5 consecutive Gemini API network/auth failures triggered the 5-minute timeout window.
* **Resolution:**
  - Check Gemini API Key validity in `/etc/beryl7/agent.env`.
  - Check router WAN internet connectivity: `ping -c 2 1.1.1.1`.
  - Force reset circuit breaker by restarting daemon: `/etc/init.d/beryl7-agent restart`.

---

## 5. Standalone Dashboard CORS or Connection Warning
* **Symptoms:** Dashboard shows `Unable to reach Router API at http://192.168.8.1:8888`.
* **Diagnosis:** Browser CORS origin mismatch or wrong IP configured in Dashboard.
* **Resolution:**
  - Click the **Admin Settings (Gear Icon)** in Standalone Dashboard and set Router Host to your router's exact IP (e.g. `192.168.8.1` or `192.168.1.1`).
  - Pass valid `Authorization: Bearer <AUTH_TOKEN>` in Admin Settings.
  - Verify `CORSAllowedOrigins` in `agent.env` includes `null` for standalone HTML opening.

---

## 6. Skill Store Cache Misses (Always Calling Gemini AI)
* **Symptoms:** Every anomaly triggers a Cloud AI request; SQLite skill cache hits remain 0.
* **Diagnosis:**
  1. Verify skill confidence threshold: Skills require `confidence >= 0.70` for local execution.
* **Resolution:**
  - Successful auto-healings automatically boost confidence by $\alpha = 0.3$.
  - Check `/var/log/beryl7_agent.log` for database write errors.

---

## 7. Watchdog Rollback Triggered Repeatedly
* **Symptoms:** Router configuration rolls back automatically within 30 seconds of an action.
* **Diagnosis:**
  1. Review system log sample in `/sbin/logread`.
  2. Verify `/tmp/agent_checkpoint.uci` snapshot state.
* **Resolution:**
  - Check if target interface name in UCI action was misconfigured.
  - Verify network cable or physical WAN link is stable.

---

## 8. Gemini API HTTP 429 Rate Limiting
* **Symptoms:** Log shows `Gemini API HTTP 429 Rate Limited! Backing off for 5s...`
* **Diagnosis:** Multiple anomalies occurring in short succession exceeding free-tier quota.
* **Resolution:**
  - Circuit Breaker will open after 5 consecutive failures for 5 minutes.
  - Rely on SQLite Skill Store local execution ($\ge 0.70$ confidence) which has 0 API cost.
