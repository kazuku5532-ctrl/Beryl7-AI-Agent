# API Versioning & Deprecation Policy 📐

---

## 1. Semantic Versioning (SemVer 2.0.0)

Beryl 7 AI Agent REST API follows [Semantic Versioning 2.0.0](https://semver.org/):

`MAJOR.MINOR.PATCH` (e.g. `v16.0.0`)

- **MAJOR:** Breaking API changes or schema modifications.
- **MINOR:** Backward-compatible new endpoints or telemetry metrics.
- **PATCH:** Backward-compatible bug fixes and security patches.

---

## 2. Backward Compatibility Guarantees

- **Deprecation Lifecycle:** Deprecated endpoints will be supported for a minimum of **12 months** following a deprecation announcement.
- **Header Notice:** Deprecated endpoints will return an HTTP response header: `Warning: 299 - "Endpoint deprecated and will be removed in v17.0"`.
