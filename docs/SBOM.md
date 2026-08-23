# Software Bill of Materials (SBOM) 📦

## 📋 System Software Architecture & Component Inventory

| Component Name | Type | Version / Range | License | Purpose / Scope |
| :--- | :--- | :--- | :--- | :--- |
| **Go Standard Library** | Core Language | `1.21.0+` | BSD-3-Clause | Core concurrency, networking, HTTP server |
| **modernc.org/sqlite** | Go Driver | `v1.28.0+` | CGO-Free BSD-3-Clause | Embedded SQLite engine with WAL support |
| **paramiko** | Python Tooling | `v3.4.0+` | LGPL-2.1 | Workstation deployment SSH client |
| **google-genai** | AI API SDK | `v0.1.1+` | Apache-2.0 | Gemini 2.5 Flash API client |

---

## 🛡️ Supply Chain Security & Vulnerability Scans

- **Static Security Analysis:** Evaluated with `gosec` (Zero G101/G104/G204 issues).
- **Dependency Audit:** Managed via Go Modules (`go.mod` / `go.sum`).
- **Binary Integrity:** Released binaries are accompanied by `SHA256SUMS` checksums.
