# Security Policy & Operational Hardening Guide 🛡️

## 📧 Reporting Vulnerabilities

To report a security vulnerability, please email `security@beryl7.local` with full technical reproduction details and proof-of-concept. All valid reports will be addressed within 24-48 hours.

---

## 🔐 1. Credential & Token Management Best Practices

- **RBAC Token Generation:**
  Always generate 256-bit cryptographically secure hex tokens using OpenSSL:
  ```bash
  AUTH_TOKEN=$(openssl rand -hex 32)
  APPROVE_TOKEN=$(openssl rand -hex 32)
  ```
- **Token Rotation Procedure:**
  Rotate tokens every 90 days or immediately following an operator role change:
  1. Update `/etc/beryl7/agent.env` on the router.
  2. Execute `chmod 0600 /etc/beryl7/agent.env`.
  3. Reload configuration without downtime: `curl -X POST http://127.0.0.1:8888/api/config/reload -H "Authorization: Bearer <AUTH_TOKEN>"`.

---

## 🔑 2. Gemini API Key Storage Security

- **Secure Key File Location:**
  Store `GEMINI_API_KEY` at `/etc/beryl7/agent.key` with strict read-only permissions:
  ```bash
  chmod 0400 /etc/beryl7/agent.key
  chown root:root /etc/beryl7/agent.key
  ```
- **Environment Variable Fallback:**
  Alternatively, store in `/etc/beryl7/agent.env` with `0600` permissions. Never commit API keys to version control.

---

## 🧱 3. Network Isolation & OpenWrt Firewall Rules

- **Localhost Binding Recommendation:**
  By default, bind the daemon to localhost (`127.0.0.1:8888`):
  ```bash
  BIND_HOST=127.0.0.1
  ```
- **LAN Access Protection:**
  If accessing from local management workstations, restrict port `8888` to the LAN interface only and block WAN traffic in OpenWrt firewall (`/etc/config/firewall`):
  ```uci
  config rule
      option name 'Block-Beryl7-WAN-Access'
      option src 'wan'
      option dest_port '8888'
      option target 'DROP'
  ```

---

## 📋 4. Audit Logging & Rate Limiting

- **Audit Trail:**
  All automated AI remediation actions and operator approvals are recorded in `/var/log/beryl7_agent.log` with timestamp, role, and action payload.
- **Rate Limiting Engine:**
  The daemon enforces a strict sliding-window rate limit of **60 requests / minute per client IP** to prevent brute-force attacks and denial-of-service attempts.

---

## 🗑️ 5. Log & Backup Retention Policy

- Log rotation maintains up to 5 historical log files (`.1` through `.5`) at 2MB per file, preventing disk exhaustion on flash storage.
- Corrupted database dumps and rollback checkpoints (`/tmp/agent_checkpoint.uci`) are automatically sanitized and pruned with restricted permissions (`0600`).
