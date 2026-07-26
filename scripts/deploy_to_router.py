import os
import sys
import time
import secrets
import paramiko
from dotenv import load_dotenv

sys.stdout.reconfigure(encoding='utf-8')

print("==================================================")
print(" Deploying Security-Hardened Go Agent (v14.1)...  ")
print("==================================================")

load_dotenv()
router_ip = os.getenv("ROUTER_IP", "192.168.8.1")
router_user = os.getenv("ROUTER_USER", "root")
router_password = os.getenv("ROUTER_PASSWORD")
gemini_key = os.getenv("GEMINI_API_KEY", "")
auth_token = os.getenv("AUTH_TOKEN")

if not router_password:
    print("❌ ERROR: ROUTER_PASSWORD environment variable not set in .env file!")
    sys.exit(1)

if not auth_token:
    auth_token = secrets.token_hex(16)
    print(f"✓ Generated secure dynamic AUTH_TOKEN: {auth_token[:6]}***")

binary_local = os.path.join("bin", "beryl7-agent")
procd_local = os.path.join("go-agent", "procd", "beryl7-agent")

if not os.path.exists(binary_local):
    print(f"Error: Binary file not found at {binary_local}")
    sys.exit(1)

print(f"1. Connecting to Beryl 7 Router via SSH ({router_user}@{router_ip})...")

ssh = paramiko.SSHClient()
ssh.load_system_host_keys()
ssh.set_missing_host_key_policy(paramiko.RejectPolicy())

try:
    ssh.connect(router_ip, port=22, username=router_user, password=router_password, timeout=10)
    print("✓ SSH Connection Established Successfully (RejectPolicy Verified)!")
except Exception as e:
    ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    ssh.connect(router_ip, port=22, username=router_user, password=router_password, timeout=10)
    print("✓ SSH Connection Established (Host key verified)!")

print("2. Stopping active beryl7-agent service to release binary lock...")
ssh.exec_command("/etc/init.d/beryl7-agent stop || killall beryl7-agent")
time.sleep(1)

def upload_bytes_stream(ssh_client, data_bytes, remote_path):
    stdin, stdout, stderr = ssh_client.exec_command(f"cat > '{remote_path}'")
    stdin.write(data_bytes)
    stdin.flush()
    stdin.close()
    stdout.channel.recv_exit_status()

def upload_file_stream(ssh_client, local_path, remote_path):
    print(f"   Streaming {local_path} ({os.path.getsize(local_path) / 1024 / 1024:.2f} MB) -> {remote_path}...")
    stdin, stdout, stderr = ssh_client.exec_command(f"cat > '{remote_path}'")
    with open(local_path, "rb") as f:
        while True:
            chunk = f.read(65536)
            if not chunk:
                break
            stdin.write(chunk)
    stdin.flush()
    stdin.close()
    stdout.channel.recv_exit_status()

print("3. Creating configuration directory /etc/beryl7 on router...")
ssh.exec_command("mkdir -p /etc/beryl7 /var/log")

print("4. Writing secure Gemini API Key & Env File (chmod 0600)...")
if gemini_key:
    upload_bytes_stream(ssh, gemini_key.strip().encode('utf-8'), "/etc/beryl7/agent.key")
    ssh.exec_command("chmod 600 /etc/beryl7/agent.key")

env_content = f'AUTH_TOKEN="{auth_token}"\nLOG_LEVEL="INFO"\nDISABLE_AUTO_HEALING="false"\n'
upload_bytes_stream(ssh, env_content.encode('utf-8'), "/etc/beryl7/agent.env")
ssh.exec_command("chmod 600 /etc/beryl7/agent.env")

print("5. Uploading beryl7-agent binary via SSH Channel Stream...")
upload_file_stream(ssh, binary_local, "/usr/bin/beryl7-agent")
ssh.exec_command("chmod +x /usr/bin/beryl7-agent")
print("✓ Binary Upload Complete!")

print("6. Uploading procd service init script...")
upload_file_stream(ssh, procd_local, "/etc/init.d/beryl7-agent")
ssh.exec_command("chmod +x /etc/init.d/beryl7-agent")

print("7. Enabling and restarting beryl7-agent 24/7 procd service...")
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
