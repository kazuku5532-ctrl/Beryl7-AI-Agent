#!/bin/sh
# ==============================================================================
# Beryl7-AI-Agent One-Click Installer for OpenWrt Linux Routers
# Target Architecture: ARM64 (aarch64) / MIPS / x86_64
# Usage: sh -c "$(curl -fsSL https://raw.githubusercontent.com/kazuku5532-ctrl/Beryl7-AI-Agent/main/scripts/install.sh)"
# ==============================================================================

set -e

echo "🚀 Starting Beryl7 AI Agent Installation..."

# 1. Detect Architecture
ARCH=$(uname -m)
case "$ARCH" in
    aarch64|arm64)
        BINARY_ARCH="arm64"
        ;;
    x86_64)
        BINARY_ARCH="amd64"
        ;;
    mips|mipsel)
        BINARY_ARCH="mipsle"
        ;;
    *)
        echo "❌ Unsupported architecture: $ARCH. Falling back to arm64 baseline."
        BINARY_ARCH="arm64"
        ;;
esac

echo "📦 Detected Host Architecture: $ARCH ($BINARY_ARCH)"

# 2. Create Target Directories
mkdir -p /etc/beryl7
mkdir -p /usr/bin
mkdir -p /root

# 3. Create Default Environment Config if not exists
if [ ! -f /etc/beryl7/agent.env ]; then
    echo "⚙️ Creating default environment configuration (/etc/beryl7/agent.env)..."
    cat <<EOF > /etc/beryl7/agent.env
AUTH_TOKEN=$(head -c 16 /dev/urandom | hexencode 2>/dev/null || date +%s | md5sum | head -c 16)
LOG_LEVEL=INFO
HEALTH_PORT=8888
BIND_HOST=0.0.0.0
BERYL7_AIRGAPPED_MODE=false
EOF
    chmod 0600 /etc/beryl7/agent.env
fi

# 4. Install Procd Service Unit
echo "🛠️ Installing OpenWrt procd init script (/etc/init.d/beryl7-agent)..."
cat <<'EOF' > /etc/init.d/beryl7-agent
#!/bin/sh /etc/rc.common

START=99
STOP=01
USE_PROCD=1
PROG=/usr/bin/beryl7-agent

start_service() {
    procd_open_instance
    procd_set_param command "$PROG" -config /etc/beryl7/agent.env
    procd_set_param file /etc/beryl7/agent.env
    procd_set_param respawn 3600 5 0
    procd_set_param env GOMEMLIMIT=15MiB GOGC=20
    procd_set_param stdout 1
    procd_set_param stderr 1
    procd_set_param oom_score_adj -500
    procd_close_instance
}

stop_service() {
    procd_send_signal "$PROG" 15
}
EOF

chmod 0755 /etc/init.d/beryl7-agent

echo "🔒 Locking auto-start on boot (/etc/rc.d/S99beryl7-agent)..."
/etc/init.d/beryl7-agent enable

echo "🛡️ Installing fail-safe cron watchdog in /etc/crontabs/root..."
mkdir -p /etc/crontabs
if ! grep -q "beryl7-agent" /etc/crontabs/root 2>/dev/null; then
    echo "* * * * * pgrep beryl7-agent >/dev/null || /etc/init.d/beryl7-agent start" >> /etc/crontabs/root
fi
/etc/init.d/cron enable 2>/dev/null || true
/etc/init.d/cron start 2>/dev/null || true

echo "🚀 Starting Beryl 7 AI Agent service..."
/etc/init.d/beryl7-agent restart

echo "✅ Beryl7 AI Agent Installation Completed Successfully!"
echo "👉 Service status: /etc/init.d/beryl7-agent status"
