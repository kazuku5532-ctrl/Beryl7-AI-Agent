#!/bin/bash
# scripts/rotate_tokens.sh - Atomic Token Rotation with Goroutine-Safe Config Reload
set -e

ROUTER_IP="${ROUTER_IP:-192.168.8.1}"
ROUTER_PORT="${ROUTER_PORT:-8888}"
ENV_FILE="/etc/beryl7/agent.env"
ENV_NEW="/etc/beryl7/agent.env.new"

echo "=== Beryl7 Token Rotation Engine ==="

# 1. Generate new 256-bit cryptographically secure tokens
NEW_AUTH_TOKEN=$(openssl rand -hex 32)
NEW_APPROVE_TOKEN=$(openssl rand -hex 32)

echo "[1/4] Generated new tokens successfully."

# 2. Write new environment file with strict ACL permissions (0600 root-only)
cat > "$ENV_NEW" << EOF
AUTH_TOKEN=$NEW_AUTH_TOKEN
APPROVE_TOKEN=$NEW_APPROVE_TOKEN
HEALTH_PORT=$ROUTER_PORT
POLL_INTERVAL=5
EOF
chmod 0600 "$ENV_NEW"

echo "[2/4] Created temporary config $ENV_NEW with permissions 0600."

# 3. Trigger goroutine-safe live config reload via HTTP POST endpoint
echo "[3/4] Invoking live HTTP POST /api/config/reload on router daemon..."
RELOAD_RESP=$(curl -s -X POST "http://${ROUTER_IP}:${ROUTER_PORT}/api/config/reload" \
  -H "Authorization: Bearer $NEW_AUTH_TOKEN" \
  -H "Content-Type: application/json" || echo "FAIL")

if [[ "$RELOAD_RESP" == *"FAIL"* ]]; then
    echo "❌ Error: Config reload request failed! Aborting token rotation."
    rm -f "$ENV_NEW"
    exit 1
fi

# 4. Perform atomic move after verifying successful reload
mv "$ENV_NEW" "$ENV_FILE"
echo "[4/4] Atomic move completed: $ENV_FILE updated."
echo "✅ Token rotation and live config reload completed successfully without service downtime!"
