# 📘 Production Deployment & Operations Runbook

> **GL.iNet Beryl 7 (GL-MT3600BE) Autonomous Remediation Agent**

---

## 1. System Requirements & Environment Planning
- **Hardware:** GL.iNet GL-MT3600BE (Beryl 7) running OpenWrt 21.02 (`aarch64`).
- **Network Interface:** Ethernet access to router on default gateway `192.168.8.1`, SSH Port 22 open.
- **Environment:** Go 1.21+ (for ARM64 cross-compilation) or Python 3.10+.
- **Secrets:** Google Gemini 2.5 Flash API Key, SSH private key (`~/.ssh/beryl7_rsa`) or admin password.

---

## 2. Pre-Deployment Checklist
- [x] Gemini API key tested and verified via REST API.
- [x] Target Router IP configured (`192.168.8.1`).
- [x] SSH key generated (`ssh-keygen -f ~/.ssh/beryl7_rsa`) and deployed (`ssh-copy-id root@192.168.8.1`).
- [x] Operational environment configuration verified (`.env`).

---

## 3. Step-by-Step Production Deployment
1. **Build Cross-Compiled ARM64 Go Daemon Binary:**
   ```powershell
   powershell -ExecutionPolicy Bypass -File .\scripts\build_go_binary.ps1
   ```
2. **Execute Deployment Automator Script:**
   ```powershell
   .\venv\Scripts\python scripts/deploy_to_router.py
   ```
3. **Verify OpenWrt procd Daemon Service Status:**
   ```bash
   ssh root@192.168.8.1 "ps | grep beryl7-agent"
   ```
4. **Verify Health Endpoint JSON Response:**
   ```bash
   curl -s http://192.168.8.1:8888/api/health -H "Authorization: Bearer <AUTH_TOKEN>"
   ```

---

## 4. Post-Deployment Monitoring & Maintenance
- Check log rotation at `/var/log/beryl7_agent.log`.
- Verify OpenWrt firewall rule `Allow-Beryl7-Health-LAN` restricting port 8888 to `lan` zone.
- Monitor SQLite Skill Store size (`skills.db`) to ensure WAL mode maintenance.
