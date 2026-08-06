#!/bin/bash
# Sample API cURL commands for testing Beryl 7 REST endpoints

ROUTER_IP="192.168.8.1"
PORT="8888"
AUTH_TOKEN="0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
APPROVE_TOKEN="fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"

echo "=== 1. Health Status ==="
curl -s "http://${ROUTER_IP}:${PORT}/api/health"

echo -e "\n\n=== 2. Prometheus Metrics ==="
curl -s "http://${ROUTER_IP}:${PORT}/metrics"

echo -e "\n\n=== 3. Architecture Modules Status ==="
curl -s "http://${ROUTER_IP}:${PORT}/api/modules/status"

echo -e "\n\n=== 4. System Logs (Auth Required) ==="
curl -s "http://${ROUTER_IP}:${PORT}/api/logs" \
  -H "Authorization: Bearer ${AUTH_TOKEN}"

echo -e "\n\n=== 5. Operator Action Approval (Approve Token Required) ==="
curl -s -X POST "http://${ROUTER_IP}:${PORT}/api/approve" \
  -H "Authorization: Bearer ${APPROVE_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"action": "restart_wan_interface", "approved_by": "operator_admin"}'

echo -e "\n\n=== 6. Config Reload ==="
curl -s -X POST "http://${ROUTER_IP}:${PORT}/api/config/reload" \
  -H "Authorization: Bearer ${AUTH_TOKEN}"
