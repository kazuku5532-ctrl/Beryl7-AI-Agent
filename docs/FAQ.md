# ❓ Frequently Asked Questions (FAQ)

---

### Q1: Is Beryl 7 AI Agent designed for x86 Linux servers or Kubernetes?
**No.** Beryl 7 AI Agent is engineered exclusively for OpenWrt Linux routers (such as GL.iNet Beryl 7 / GL-MT3600BE). It interacts directly with OpenWrt's `uci`, `ubus`, and `/sbin/logread` kernel interfaces.

### Q2: What happens if the router loses internet connectivity?
The agent continues operating locally using the embedded SQLite SkillStore. Any anomaly with a learned skill confidence $\ge 0.70$ is executed locally without calling Cloud AI.

### Q3: How much RAM does the daemon consume?
The native Go daemon consumes **7.5 MB RAM** (`VmRSS`), which is less than 1.5% of the GL-MT3600BE's 512MB RAM.

### Q4: Does the daemon require CGO or gcc on the router?
**No.** The daemon uses `modernc.org/sqlite` which is a 100% pure Go implementation. The binary is cross-compiled statically without CGO.

### Q5: How does the Watchdog prevent bad network configuration lockouts?
Before executing any network change, the Watchdog creates a snapshot at `/tmp/agent_checkpoint.uci`. It launches a 30s background script that automatically executes `uci revert` unless the daemon completes post-action health verification within 15 seconds.

### Q6: Can I change the default HTTP management port 8888?
Yes. Set `HEALTH_PORT=9999` in `/etc/beryl7/agent.env` and restart the daemon. CORS headers will automatically adjust.

### Q7: How do I generate secure authentication tokens?
Run `openssl rand -hex 32` on your workstation or router to generate 256-bit hex tokens for `AUTH_TOKEN` and `APPROVE_TOKEN`.

### Q8: What AI models are supported?
By default, the agent uses **Gemini 2.5 Flash** for log analysis and action recommendation due to its low latency (~280ms) and high reasoning capabilities.

### Q9: What is the daily API budget limit?
The default daily budget cap is **$1.00 USD**. Circuit breakers open automatically after 5 consecutive API failures.

### Q10: How do I view Prometheus metrics?
Scrape `http://<router-ip>:8888/metrics` using Prometheus or import `grafana/beryl7_dashboard.json` into Grafana.

### Q11: How many log backups are preserved during log rotation?
Up to **5 historical backup files** (`.1` through `.5`) are kept, bounded at 2MB per file.

### Q12: Can I disable auto-healing and operate in monitor-only mode?
Yes. Set `BERYL7_DISABLE_HEALING=true` in `/etc/beryl7/agent.env`.

### Q13: What happens during a firmware upgrade?
The daemon registers `/etc/beryl7/agent.env` and `/root/skills.db` in `/etc/sysupgrade.conf` so configuration and learned skills persist across OpenWrt firmware upgrades.

### Q14: How do I test the binary without deploying to a physical router?
Use the included `Dockerfile` and `docker-compose.yml` to run an emulated OpenWrt environment locally.

### Q15: How does CORS origin handling work for the HTTP API?
CORS headers allow local LAN subnets (`192.168.8.x`, `127.0.0.1`, `localhost`) or custom origins specified in `CORS_ORIGINS` in `/etc/beryl7/agent.env`.

### Q16: How is Wi-Fi bandwidth boost triggered?
When WAN download throughput exceeds `80.0 Mbps`, the agent boosts 5GHz channel width from 80MHz to 160MHz (`htmode EHT160`). When throughput drops below `20.0 Mbps` for 60 seconds, it restores 80MHz.

### Q17: How is Exponential Weighted Moving Average (EMA) calculated?
$$\text{Confidence}_{\text{new}} = \text{Confidence}_{\text{old}} + \alpha \times (\text{Target} - \text{Confidence}_{\text{old}})$$
where $\alpha = 0.3$ and Target = $1.0$ for success, $0.0$ for failure.

### Q18: Where are database corruption backups stored?
If SQLite integrity fails, a dump is exported to `/root/skills.db.corrupt.<timestamp>.sql` and restored from `/root/skills.db.bak`.

### Q19: How do I check the daemon version from CLI?
Run `/usr/bin/beryl7-agent -version` from the router terminal.

### Q20: What is the license for this repository?
Beryl 7 AI Agent is open-source software licensed under the **MIT License**.
