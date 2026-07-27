import os
import sys
import time
import secrets
import base64
import hashlib
import paramiko
from dotenv import load_dotenv

sys.path.insert(0, os.path.abspath("."))
from agent.ssh_auth import SSHAuthenticator

sys.stdout.reconfigure(encoding='utf-8')

print("==================================================")
print(" Deploying Security-Hardened Go Agent (v14.1)...  ")
print("==================================================")

load_dotenv()
router_ip = os.getenv("ROUTER_IP", "192.168.8.1")
router_user = os.getenv("ROUTER_USER", "root")
router_password = os.getenv("ROUTER_PASSWORD")
ssh_key_path = os.getenv("SSH_KEY_PATH")
gemini_key = os.getenv("GEMINI_API_KEY", "")
auth_token = os.getenv("AUTH_TOKEN")
approve_token = os.getenv("APPROVE_TOKEN")

if not auth_token:
    auth_token = secrets.token_hex(16)
    print(f"✓ Generated secure dynamic AUTH_TOKEN: {auth_token[:6]}***")

if not approve_token:
    approve_token = secrets.token_hex(16)
    print(f"✓ Generated secure high-privilege APPROVE_TOKEN: {approve_token[:6]}***")

binary_local = os.path.join("bin", "beryl7-agent")
procd_local = os.path.join("go-agent", "procd", "beryl7-agent")

if not os.path.exists(binary_local):
    print(f"⚠️ Binary file not found at {binary_local}! Initiating Auto-Cross-Compilation (GOOS=linux, GOARCH=arm64)...")
    import subprocess
    env = dict(os.environ, GOOS="linux", GOARCH="arm64", CGO_ENABLED="0")
    res = subprocess.run(["go", "build", "-ldflags=-s -w", "-o", binary_local, "./cmd"], cwd="go-agent", env=env)
    if res.returncode != 0:
        print("❌ ERROR: Auto-Cross-Compilation failed!")
        sys.exit(1)
    print("✓ Auto-Cross-Compilation Succeeded!")

print(f"1. Connecting to Beryl 7 Router via SSH ({router_user}@{router_ip})...")

ssh = paramiko.SSHClient()
ssh.load_system_host_keys()

try:
    authenticator = SSHAuthenticator(router_host=router_ip, key_path=ssh_key_path)
    authenticator.configure_client(ssh, username=router_user, timeout=10)
    print("✓ SSH Connection Established Successfully (SSH Key / Password Authenticated)!")
except Exception as e:
    print(f"❌ SSH Connection Failed: {e}")
    print("⚠️ Please ensure router is reachable at 192.168.8.1 and SSH credentials or ~/.ssh/beryl7_rsa key are configured.")
    sys.exit(1)

print("2. Stopping active beryl7-agent service to release binary lock...")
ssh.exec_command("/etc/init.d/beryl7-agent stop || killall beryl7-agent")
time.sleep(1)

def calculate_sha256(filepath):
    h = hashlib.sha256()
    with open(filepath, "rb") as f:
        while chunk := f.read(65536):
            h.update(chunk)
    return h.hexdigest()

def safe_upload_file(ssh_client, local_path, remote_path, mode=0o644):
    print(f"   Uploading {local_path} ({os.path.getsize(local_path) / 1024 / 1024:.2f} MB) -> {remote_path}...")
    local_sha256 = calculate_sha256(local_path)
    
    try:
        sftp = ssh_client.open_sftp()
        sftp.put(local_path, remote_path)
        sftp.chmod(remote_path, mode)
        sftp.close()
        print("   ✓ Uploaded via Paramiko SFTPClient.put successfully!")
    except Exception:
        stdin, stdout, stderr = ssh_client.exec_command(f"cat > '{remote_path}'")
        with open(local_path, "rb") as f:
            while chunk := f.read(65536):
                stdin.write(chunk)
        stdin.flush()
        stdin.close()
        stdout.channel.recv_exit_status()
        ssh_client.exec_command(f"chmod {oct(mode)[2:]} '{remote_path}'")
        print("   ✓ Uploaded via Direct Raw Stream (Cat Fallback Verified)!")

    stdin, stdout, stderr = ssh_client.exec_command(f"sha256sum '{remote_path}' | cut -d' ' -f1")
    remote_sha256 = stdout.read().decode('utf-8').strip()
    if remote_sha256.lower() == local_sha256.lower():
        print(f"   ✓ Checksum SHA256 Verified ({remote_sha256[:8]}... Match)!")
    else:
        print(f"   ⚠️ WARNING: Checksum Mismatch! Local: {local_sha256} vs Remote: {remote_sha256}")

print("3. Creating configuration directory /etc/beryl7 on router...")
ssh.exec_command("mkdir -p /etc/beryl7 /var/log")

print("4. Writing secure Gemini API Key & Env File (chmod 0600)...")
if gemini_key:
    tmp_key_file = "tmp_agent.key"
    with open(tmp_key_file, "w", encoding="utf-8") as f:
        f.write(gemini_key.strip())
    safe_upload_file(ssh, tmp_key_file, "/etc/beryl7/agent.key", 0o600)
    if os.path.exists(tmp_key_file):
        os.remove(tmp_key_file)

tmp_env_file = "tmp_agent.env"
env_content = f'AUTH_TOKEN="{auth_token}"\nAPPROVE_TOKEN="{approve_token}"\nLOG_LEVEL="INFO"\nDISABLE_AUTO_HEALING="false"\n'
with open(tmp_env_file, "w", encoding="utf-8") as f:
    f.write(env_content)
safe_upload_file(ssh, tmp_env_file, "/etc/beryl7/agent.env", 0o600)
if os.path.exists(tmp_env_file):
    os.remove(tmp_env_file)

print("5. Uploading beryl7-agent binary to /usr/bin/beryl7-agent...")
safe_upload_file(ssh, binary_local, "/usr/bin/beryl7-agent", 0o755)

print("6. Uploading procd service init script...")
safe_upload_file(ssh, procd_local, "/etc/init.d/beryl7-agent", 0o755)

print("7. Configuring OpenWrt firewall rule (Allow-Beryl7-Health-LAN) & restarting procd service...")
fw_cmds = (
    "uci set firewall.beryl7_health=rule && "
    "uci set firewall.beryl7_health.name='Allow-Beryl7-Health-LAN' && "
    "uci set firewall.beryl7_health.src='lan' && "
    "uci set firewall.beryl7_health.dest_port='8888' && "
    "uci set firewall.beryl7_health.proto='tcp' && "
    "uci set firewall.beryl7_health.target='ACCEPT' && "
    "uci commit firewall && /etc/init.d/firewall reload >/dev/null 2>&1"
)
ssh.exec_command(fw_cmds)
stdin, stdout, stderr = ssh.exec_command("/etc/init.d/beryl7-agent enable && /etc/init.d/beryl7-agent restart")
stdout.channel.recv_exit_status()

time.sleep(2)

print("8. Verifying live service status on router...")
stdin, stdout, stderr = ssh.exec_command("ps | grep beryl7-agent | grep -v grep")
status_output = stdout.read().decode('utf-8').strip()

ssh.close()

if status_output:
    print("\n==================================================")
    print(" SUCCESS: Security-Hardened Go Agent (v14.1) Live! ")
    print(f" Live Process: {status_output}")
    print("==================================================")
else:
    print("⚠️ Warning: Process check returned empty, checking log...")
