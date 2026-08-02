# Beryl 7 AI Agent - Getting Started Guide 🚀

This guide provides step-by-step instructions to compile, configure, deploy, and operate the Beryl 7 AI Agent from scratch on an OpenWrt router (**GL.iNet Beryl 7 / GL-MT3600BE**).

---

## 📋 Prerequisites

- **Router Hardware:** GL.iNet Beryl 7 (GL-MT3600BE - Mediatek Filogic 820 ARM64) running OpenWrt 24.10+.
- **Management Workstation:** Linux, macOS, or Windows machine with:
  - **Go 1.21+** installed (for cross-compiling binary).
  - **OpenSSL** (for generating 256-bit RBAC tokens).
  - **SSH / SCP** access to router (`192.168.8.1`).

---

## 🛠️ Step 1: Cross-Compile Native Go ARM64 Binary

On your development machine, clone the repository and cross-compile the Go daemon for Linux ARM64:

```bash
# Clone repository
git clone https://github.com/kazuku5532-ctrl/Beryl7-AI-Agent.git
cd Beryl7-AI-Agent/go-agent

# Cross-compile Linux ARM64 binary
GOOS=linux GOARCH=arm64 go build -o beryl7-agent ./cmd
```

---

## 🔑 Step 2: Generate RBAC Tokens & Configure Environment

### 1. Generate 256-Bit Cryptographic Secrets
Generate strong hex tokens for Admin and Operator roles:

```bash
# Generate Admin Auth Token
AUTH_TOKEN=$(openssl rand -hex 32)
echo "Generated AUTH_TOKEN: $AUTH_TOKEN"

# Generate Operator Approval Token
APPROVE_TOKEN=$(openssl rand -hex 32)
echo "Generated APPROVE_TOKEN: $APPROVE_TOKEN"
```

### 2. Create Environment File (`/etc/beryl7/agent.env`) on Router
Connect to your router via SSH and write the environment configuration file:

```bash
ssh root@192.168.8.1

# Create config directory
mkdir -p /etc/beryl7

# Create environment file with strict 0600 permissions
cat <<EOF > /etc/beryl7/agent.env
AUTH_TOKEN=$AUTH_TOKEN
APPROVE_TOKEN=$APPROVE_TOKEN
LOG_LEVEL=INFO
HEALTH_PORT=8888
BIND_HOST=127.0.0.1
GEMINI_API_KEY=your_gemini_api_key_here

# Dynamic Anomaly Threshold Overrides (Optional)
BERYL7_RAM_EXHAUSTION_PCT=92.0
BERYL7_CPU_SPIKE_LOAD=1.5
BERYL7_LATENCY_SPIKE_MS=100.0
BERYL7_LATENCY_ZSCORE=2.5
BERYL7_BANDWIDTH_BOOST_MBPS=80.0
BERYL7_BANDWIDTH_RESTORE_MBPS=20.0
BERYL7_WIFI_DISCONNECT_COUNT=3
EOF

chmod 0600 /etc/beryl7/agent.env
```

---

## 📡 Step 3: Deploy Binary to Router

From your management workstation, upload the compiled binary to the router:

```bash
# Upload binary via SCP
scp go-agent/beryl7-agent root@192.168.8.1:/usr/bin/beryl7-agent

# Make binary executable on router
ssh root@192.168.8.1 "chmod +x /usr/bin/beryl7-agent"
```

---

## ⚙️ Step 4: Configure OpenWrt `procd` Init Service

Create a native OpenWrt `procd` init service script at `/etc/init.d/beryl7-agent` to ensure the daemon automatically runs at startup and restarts on crash:

```bash
ssh root@192.168.8.1

cat <<'EOF' > /etc/init.d/beryl7-agent
#!/bin/sh /etc/rc.common

START=99
STOP=15
USE_PROCD=1
PROG=/usr/bin/beryl7-agent

start_service() {
    procd_open_instance
    procd_set_param command "$PROG" -config /etc/beryl7/agent.env
    procd_set_param respawn 3600 5 0
    procd_set_param stdout 1
    procd_set_param stderr 1
    procd_close_instance
}
EOF

chmod +x /etc/init.d/beryl7-agent
/etc/init.d/beryl7-agent enable
/etc/init.d/beryl7-agent start
```

---

## 🧪 Step 5: Verify Live Deployment & RBAC API Endpoints

### 1. Check Service Health (`viewer` role)
```bash
curl -X GET http://192.168.8.1:8888/api/health
```

### 2. Verify Live Config Reload (`operator` role)
```bash
curl -X POST http://192.168.8.1:8888/api/config/reload \
  -H "Authorization: Bearer $APPROVE_TOKEN"
```

### 3. Verify Admin Budget Status (`admin` role)
```bash
curl -X GET http://192.168.8.1:8888/api/budget/status \
  -H "Authorization: Bearer $AUTH_TOKEN"
```

---

## 🖥️ Step 6: Open Web Operations Dashboard

1. Double-click [Beryl7_Dashboard_Standalone.html](../dashboard/Beryl7_Dashboard_Standalone.html) in any modern web browser.
2. In the top navbar or Admin Settings modal (`⚙️`), set:
   - **Router API Host Address:** `http://192.168.8.1:8888`
   - **API Authorization Token:** Your `APPROVE_TOKEN` or `AUTH_TOKEN`.
3. View real-time CPU, RAM, hardware temperature, latency, API budget status, and logread stream.
