# 🚀 Migration, Upgrade, Rollback & Uninstall Guide

---

## 1. Upgrade Procedure (v15.x ➔ v16.0)

### Pre-Deployment Checklist
- [x] Verify router disk space: `df -h /` (At least 16MB free space).
- [x] Verify router RAM: `free -m` (At least 64MB available RAM).
- [x] Backup existing skills database: `cp /root/skills.db /root/skills.db.v15.bak`.
- [x] Backup configuration environment: `cp /etc/beryl7/agent.env /etc/beryl7/agent.env.bak`.

### Zero-Downtime Binary Upgrade
1. Cross-compile latest v16.0 ARM64 binary on management workstation:
   ```bash
   GOOS=linux GOARCH=arm64 go build -o beryl7-agent ./cmd
   ```
2. Upload new binary to temporary staging location on router:
   ```bash
   scp go-agent/beryl7-agent root@192.168.8.1:/tmp/beryl7-agent-new
   ```
3. Atomic swap and restart via OpenWrt `procd`:
   ```bash
   ssh root@192.168.8.1 "chmod +x /tmp/beryl7-agent-new && mv /tmp/beryl7-agent-new /usr/bin/beryl7-agent && /etc/init.d/beryl7-agent restart"
   ```
4. Verify live service health:
   ```bash
   curl http://192.168.8.1:8888/api/health
   ```

---

## 2. Emergency Rollback Procedure (v16.0 ➔ v15.x)

If an issue occurs post-upgrade, execute an instant binary rollback:

```bash
ssh root@192.168.8.1
# Stop running daemon
/etc/init.d/beryl7-agent stop

# Restore backup binary and database
cp /usr/bin/beryl7-agent.backup /usr/bin/beryl7-agent
cp /root/skills.db.v15.bak /root/skills.db

# Restart daemon
/etc/init.d/beryl7-agent start
```

---

## 3. Complete Uninstall & Cleanup Procedure

To cleanly remove Beryl 7 AI Agent from your OpenWrt router:

```bash
ssh root@192.168.8.1

# 1. Stop and disable procd init service
/etc/init.d/beryl7-agent stop
/etc/init.d/beryl7-agent disable
rm -f /etc/init.d/beryl7-agent

# 2. Remove binaries, configurations, and logs
rm -f /usr/bin/beryl7-agent /usr/bin/beryl7-agent.backup
rm -rf /etc/beryl7
rm -f /var/log/beryl7_agent.log*
rm -f /var/run/beryl7-agent.pid

# 3. Remove databases and checkpoints
rm -f /root/skills.db* /root/.agent_checkpoint.uci /tmp/agent_checkpoint.uci

# 4. Reload procd
/etc/init.d/network reload
```
