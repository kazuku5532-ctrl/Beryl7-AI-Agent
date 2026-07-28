# CI/CD Release Pipeline Specification 🚀

System: **Beryl 7 AI Agent**  
Document Version: **v1.0.0**

---

## 1. End-to-End Pipeline Workflow

```mermaid
graph LR
    A[Code Push] --> B[1. Build]
    B --> C[2. Lint]
    C --> D[3. Unit & Integration Test]
    D --> E[4. Code Coverage >= 90%]
    E --> F[5. Security Audit]
    F --> G[6. SBOM Generation]
    G --> H[7. Cosign Binary Sign]
    H --> I[8. GitHub Release & Artifacts]
```

---

## 2. Quality Gates & Enforcement Rules

| Stage | Command / Tool | Success Criteria |
| :--- | :--- | :--- |
| **Linting** | `golangci-lint run` | 0 errors |
| **Unit Testing** | `go test -v -cover ./...` | Pass & Coverage $\ge 90\%$ |
| **Go Security** | `gosec ./...` | 0 High / Critical issues |
| **Python Security** | `bandit -r agent/` | 0 High issues |
| **Secrets Audit** | `detect-secrets scan` | 0 leaked credentials |
| **SBOM Generation** | `python scripts/generate_sbom.py` | Valid SPDX JSON generated |
| **Binary Verification**| `python scripts/cosign_sign.py` | SHA256 checksum & signature matched |
