# Beryl 7 AI Agent — Backend REST API Specification 🛠️

Version: **v16.0**  
Server Port: `8888` (Router Go Daemon)  
Authentication: Optional Header `Authorization: Bearer <token>`  
Rate Limit: `60 requests / minute per client IP`  

---

## 📡 Endpoints Summary Table

| Endpoint | HTTP Method | Auth Required | Description |
| :--- | :--- | :--- | :--- |
| `/api/health` | `GET` | No | Primary telemetry metrics (CPU, RAM, Temp, Latency, Uptime, SLO) |
| `/api/modules/status` | `GET` | No | Status of all 6 core architecture modules |
| `/api/logs` | `GET` | No | Live OpenWrt `/sbin/logread` log entries |
| `/api/metrics/history` | `GET` | No | Historical trends for availability & remediation success |
| `/api/cache/stats` | `GET` | No | SQLite Skill Store cache hit rate breakdown |
| `/api/system/info` | `GET` | No | Device hardware & firmware metadata |
| `/api/approve` | `POST` | Yes | Operator approval for gated high-risk actions |
| `/api/settings` | `POST` | Yes | Update server configuration & telemetry intervals |

---

## 🔍 Detailed Endpoint Specifications

### 1. `GET /api/health`
Returns real-time health telemetry from router hardware and agent process.

**Response Schema (`200 OK`):**
```json
{
  "status": "healthy",
  "healthy": true,
  "timestamp": 1785214037,
  "router_reachable": true,
  "cpu_usage_pct": 1.1,
  "ram_usage_pct": 43.9,
  "hardware_temp_c": 60.3,
  "latency_ms": 31.0,
  "uptime_seconds": 32454,
  "cache_hit_rate_pct": 91.4,
  "slo_score_pct": 100.0,
  "wan_status": "Active (1/1)",
  "safe_mode": false,
  "kill_switch": false
}
```

---

### 2. `GET /api/modules/status`
Returns health status of the 6 core architecture components.

**Response Schema (`200 OK`):**
```json
{
  "orchestrator": { "status": "healthy", "priority": "ACTIVE", "interval_s": 5.0 },
  "executor": { "status": "healthy", "whitelist": "100%", "uci_mapping": "MT7993" },
  "ai_client": { "status": "healthy", "model": "Gemini 2.5 Flash", "latency_ms": 280 },
  "watchdog": { "status": "healthy", "checkpoint": "UCI Export", "rollback_rate": 0.0 },
  "log_parser": { "status": "healthy", "source": "/sbin/logread", "regex": "Matched" },
  "skill_store": { "status": "healthy", "storage": "SQLite WAL", "lookup_latency_ms": 0.4 }
}
```

---

### 3. `GET /api/logs`
Returns sanitized log lines from the OpenWrt `/sbin/logread` stream.

**Response Schema (`200 OK`):**
```json
{
  "logs": [
    { "time": "12:04:15", "level": "INFO", "msg": "[Main] Telemetry health check completed cycle #50" }
  ],
  "total": 50
}
```

---

### 4. `POST /api/approve`
Approves a pending AI remediation action queued for operator approval.

**Request Body (`application/json`):**
```json
{
  "action": "restart_wan_interface",
  "approved_by": "operator_admin"
}
```

**Response Schema (`200 OK`):**
```json
{
  "status": "success",
  "message": "Action approved successfully"
}
```

---

## ❌ HTTP Error Response Specifications

### 1. `401 Unauthorized`
Returned when an endpoint requiring Operator or Admin role (e.g. `/api/logs`) is called without a valid Bearer token.
```json
{
  "error": "Unauthorized: Operator or Admin Auth Token required to access system logs"
}
```

### 2. `429 Too Many Requests`
Returned when a client IP exceeds 60 requests within a 60-second sliding window.
```json
{
  "error": "Too Many Requests: Rate limit exceeded"
}
```

### 3. `500 Internal Server Error`
Returned when a configuration reload or internal storage error occurs.
```json
{
  "error": "Internal Error: Failed to reload configuration in-memory"
}
```
