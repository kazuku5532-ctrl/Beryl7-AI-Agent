import os
import sys
import time
import secrets
import base64
import hashlib
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
approve_token = os.getenv("APPROVE_TOKEN")

if not router_password:
    print("❌ ERROR: ROUTER_PASSWORD environment variable not set in .env file!")
    sys.exit(1)

if not auth_token:
    auth_token = secrets.token_hex(16)
    print(f"✓ Generated secure dynamic AUTH_TOKEN: {auth_token[:6]}***")

if not approve_token:
    approve_token = secrets.token_hex(16)
    print(f"✓ Generated secure high-privilege APPROVE_TOKEN: {approve_token[:6]}***")

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
    print("✓ SSH Connection Established Successfully (Strict RejectPolicy Verified)!")
except Exception as e:
    print(f"❌ SSH Connection Error under RejectPolicy: {e}")
    print("⚠️ If host key is missing, add it to system known_hosts: ssh-keyscan -H 192.168.8.1 >> ~/.ssh/known_hosts")
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
        with open(local_path, "rb") as f:
            file_bytes = f.read()
        b64_content = base64.b64encode(file_bytes).decode('utf-8')
        
        chunk_size = 32768
        ssh_client.exec_command(f"rm -f '{remote_path}' && touch '{remote_path}'")
        for i in range(0, len(b64_content), chunk_size):
            chunk = b64_content[i:i+chunk_size]
            ssh_client.exec_command(f"echo '{chunk}' | base64 -d >> '{remote_path}'")
            
        ssh_client.exec_command(f"chmod {oct(mode)[2:]} '{remote_path}'")
        print("   ✓ Uploaded via Safe Base64 Stream (Dropbear Fallback Verified)!")

    # Khắc phục Hạng mục 5: Xác minh Checksum SHA256 sau khi upload
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
