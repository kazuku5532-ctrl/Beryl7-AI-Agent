#!/bin/bash
# scripts/health_check.sh - Health Check & Binary Backup Rollback Trigger
set -e

ROUTER_IP="${ROUTER_IP:-192.168.8.1}"
ROUTER_PORT="${ROUTER_PORT:-8888}"
HEALTH_URL="http://${ROUTER_IP}:${ROUTER_PORT}/api/health"
LATENCY_THRESHOLD_MS=500
BINARY_CURRENT="/usr/bin/beryl7-agent"
BINARY_BACKUP="/usr/bin/beryl7-agent.backup"

echo "=== Beryl7 Health Check & Automated Rollback Monitor ==="

# 1. Probe health endpoint and measure round-trip time
HTTP_RESP=$(curl -s -w "\n%{http_code}\n%{time_total}" "$HEALTH_URL" || echo -e "FAIL\n500\n1.0")
RESPONSE_BODY=$(echo "$HTTP_RESP" | head -n 1)
HTTP_CODE=$(echo "$HTTP_RESP" | sed -n '2p')
TIME_TOTAL=$(echo "$HTTP_RESP" | tail -n 1)

LATENCY_MS=$(python3 -c "print(int(float('$TIME_TOTAL') * 1000))" 2>/dev/null || echo "999")

echo "Health probe response code: $HTTP_CODE | Latency: ${LATENCY_MS}ms"

# 2. Check latency threshold against SLO (500ms)
if [ "$HTTP_CODE" -eq 200 ] && [ "$LATENCY_MS" -le "$LATENCY_THRESHOLD_MS" ]; then
    echo "🟢 SLO Passed: Router daemon is healthy and operating within latency boundaries."
    exit 0
fi

echo "❌ SLO Violation Detected! HTTP Code: $HTTP_CODE | Latency: ${LATENCY_MS}ms (Threshold: ${LATENCY_THRESHOLD_MS}ms)"

# 3. Check if backup binary exists for automated rollback
if [ ! -f "$BINARY_BACKUP" ]; then
    echo "⚠️ Warning: Backup binary $BINARY_BACKUP not found! Automatic rollback skipped."
    exit 1
fi

# 4. Trigger binary backup rollback
echo "🔄 Restoring previous binary from $BINARY_BACKUP to $BINARY_CURRENT..."
cp "$BINARY_BACKUP" "$BINARY_CURRENT"
chmod +x "$BINARY_CURRENT"

echo "Restarting service..."
/etc/init.d/beryl7-agent restart || true

echo "✅ Binary rollback completed! Monitoring recovered service..."
