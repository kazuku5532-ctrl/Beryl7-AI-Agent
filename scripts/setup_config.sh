#!/bin/bash
set -euo pipefail

echo "==> Setting up Beryl 7 Go Agent Configuration Directory..."
mkdir -p /etc/beryl7

if [ ! -f /etc/beryl7/agent.key ]; then
    echo "Creating secure API key file at /etc/beryl7/agent.key (chmod 600)..."
    touch /etc/beryl7/agent.key
    chmod 600 /etc/beryl7/agent.key
fi

if [ ! -f /etc/beryl7/agent.env ]; then
    echo "Generating dynamic random AUTH_TOKEN for /etc/beryl7/agent.env (chmod 600)..."
    RANDOM_TOKEN=$(openssl rand -hex 16 2>/dev/null || head -c 16 /dev/urandom | xxd -p)
    cat > /etc/beryl7/agent.env << EOF
# Beryl 7 Go Agent Configuration
AUTH_TOKEN="${RANDOM_TOKEN}"
LOG_LEVEL="INFO"
DISABLE_AUTO_HEALING="false"
EOF
    chmod 600 /etc/beryl7/agent.env
fi

echo "==> Configuration setup complete!"
