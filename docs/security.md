# Security Architecture & Compliance 🔒

System: **Beryl 7 AI Agent**  
Document Version: **v1.0.0**

---

## 1. Core Security Control Principles

1. **Non-Shell Execution Safety:** All OpenWrt UCI changes execute directly via compiled parameters (`exec.Command("uci", "set", ...)`), completely preventing command injection vulnerabilities (CWE-78).
2. **Strict Whitelist Enforcement:** Action execution is restricted 100% to explicit pre-approved UCI parameters.
3. **Const-Time Token Security:** Authorization tokens are checked using `subtle.ConstantTimeCompare` to defend against timing side-channel attacks.
4. **CORS & Rate Limiting:** All endpoints enforce strict CORS headers and IP rate limiting (60 requests/minute).
