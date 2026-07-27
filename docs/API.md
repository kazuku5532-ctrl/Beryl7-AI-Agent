# 🔌 API Specification & Prometheus Metrics Documentation

---

## 1. Health Check Endpoint

- **URL:** `GET /api/health` (or `GET /health`)
- **Port:** `8888`
- **Authentication:** Bearer Token (`Authorization: Bearer <AUTH_TOKEN>`)
- **Response Format:** `application/json`

### Example Success Response (HTTP 200):
```json
{
  "status": "healthy",
  "uptime_seconds": 86400,
  "last_action": "purge_memory_cache",
  "wan_status": "Active (1/1)",
  "cpu_usage_pct": 0.85,
  "ram_usage_pct": 34.2,
  "hardware_temp_c": 58.8,
  "latency_ms": 12.4,
  "safe_mode": false,
  "kill_switch": false
}
```

---

## 2. Operator Approval Endpoint

- **URL:** `POST /api/approve`
- **Port:** `8888`
- **Authentication:** High-Privilege Operator Bearer Token (`Authorization: Bearer <APPROVE_TOKEN>`)
- **Fail-Closed Policy:** Requires `APPROVE_TOKEN` to be distinct from `AUTH_TOKEN`.
- **Hạn ngạch Expiry:** Pending requests expire after 10 minutes.

### Example Response (HTTP 200):
```json
{
  "status": "approved_and_executed",
  "action": "block_device",
  "details": "Operator approval verified and executed successfully"
}
```

---

## 3. Prometheus Metrics Endpoint

- **URL:** `GET /metrics`
- **Port:** `8888`
- **Authentication:** Bearer Token
- **Response Format:** `text/plain; version=0.0.4`

### Available Metrics:
- `beryl7_anomalies_detected_total{category="...", severity="..."}` (Counter)
- `beryl7_cache_hits_total` (Counter)
- `beryl7_cache_misses_total` (Counter)
- `beryl7_skills_in_store` (Gauge)
- `beryl7_system_cpu_load_1m` (Gauge)
- `beryl7_system_ram_usage_percent` (Gauge)
- `beryl7_router_reachable` (Gauge)
