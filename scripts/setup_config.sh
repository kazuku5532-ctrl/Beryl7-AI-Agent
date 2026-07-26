#!/bin/bash
set -euo pipefail

echo "==> Setting up Beryl 7 Go Agent Configuration Directory..."
mkdir -p /etc/beryl7

if [ ! -f /etc/beryl7/agent.key ]; then
    echo "Creating secure API key file at /etc/beryl7/agent.key (chmod 600)..."
    echo "YOUR_GEMINI_API_KEY_HERE" > /etc/beryl7/agent.key
    chmod 600 /etc/beryl7/agent.key
fi

if [ ! -f /etc/beryl7/agent.env ]; then
    echo "Creating config environment file at /etc/beryl7/agent.env (chmod 600)..."
    cat > /etc/beryl7/agent.env << 'EOF'
# Beryl 7 Go Agent Configuration
AUTH_TOKEN="beryl7-secret-health-token"
LOG_LEVEL="INFO"
DISABLE_AUTO_HEALING="false"
EOF
    chmod 600 /etc/beryl7/agent.env
fi

echo "==> Configuration setup complete!"
