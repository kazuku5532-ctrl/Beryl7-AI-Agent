# Privacy Policy & Data Processing Specification 🔒

---

## 1. Local Processing First Principle

Beryl 7 AI Agent is engineered with a **Local-First Privacy Architecture**. All real-time telemetry collection (CPU, RAM, temperature, ping latency, conntrack sessions) and SQLite SkillStore operations occur **100% locally on the router device**. No personal user traffic, visited URLs, DNS query histories, or payload data are ever transmitted to cloud servers.

---

## 2. Cloud AI Payload Sanitization

When an unclassified system log anomaly requires Cloud AI analysis (via Gemini API):
- **IP Address Anonymization:** Private client IPv4/IPv6 addresses (`192.168.x.x`, `10.x.x.x`) are masked before transmission.
- **MAC Address Anonymization:** Hardware MAC addresses are replaced with anonymized hashes (`XX:XX:XX:XX:XX:XX`).
- **Credential Redaction:** Passwords, tokens, Bearer headers, and JWT keys are automatically stripped using regex sanitization filters.
- **Strict Data Scope:** Only system kernel log messages (`/sbin/logread`) relating to network driver state transitions are sent to the AI API.

---

## 3. Data Retention & Erasure

- **Local Log Rotation:** System logs are bounded to 5 rotated files (`/var/log/beryl7_agent.log.*`) and automatically overwritten.
- **Local Skill Database:** Stored locally at `/root/skills.db`. Operators can erase all learned skills at any time by executing:
  ```bash
  rm -f /root/skills.db*
  ```

---

## 4. GDPR & Enterprise Compliance

- **No User Identification:** The software does not track, collect, or store Personally Identifiable Information (PII).
- **Compliance Certification:** Meets standard enterprise network management data privacy requirements for edge appliances.
