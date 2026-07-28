# STRIDE Threat Model Analysis 🛡️

System: **Beryl 7 AI Agent**  
Document Version: **v1.0.0**

---

## 1. STRIDE Category Breakdown

| STRIDE Category | Threat Scenario | Mitigation Mechanism |
| :--- | :--- | :--- |
| **Spoofing** | Unauthorized API requests posing as dashboard | Constant-time bearer token check |
| **Tampering** | Modification of pending approval JSON file | File permissions restricted to `0600` root-only |
| **Repudiation** | Operator denying action approval | Timestamped append-only audit log `/var/log/beryl7_approval_audit.log` |
| **Information Disclosure** | Secret token leakage | Tokens scrubbed from logs; `detect-secrets` CI check |
| **Denial of Service** | Telemetry endpoint flood | Per-IP rate limiting (60 req/min) & timeout limits |
| **Elevation of Privilege** | Arbitrary shell execution | Strict non-shell UCI whitelist execution |
