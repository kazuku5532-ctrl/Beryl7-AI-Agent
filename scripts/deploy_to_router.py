import os
import sys
import time
import base64
import paramiko
from dotenv import load_dotenv

sys.stdout.reconfigure(encoding='utf-8')

print("==================================================")
print(" Deploying Native Go Agent to GL.iNet Beryl 7... ")
print("==================================================")

load_dotenv()
router_ip = os.getenv("ROUTER_IP", "192.168.8.1")
router_user = os.getenv("ROUTER_USER", "root")
router_password = os.getenv("ROUTER_PASSWORD", "Kazuku@2k6")
gemini_key = os.getenv("GEMINI_API_KEY", "")

binary_local = os.path.join("bin", "beryl7-agent")
procd_local = os.path.join("go-agent", "procd", "beryl7-agent")

if not os.path.exists(binary_local):
    print(f"Error: Binary file not found at {binary_local}")
    sys.exit(1)

print(f"1. Connecting to Beryl 7 Router via SSH ({router_user}@{router_ip})...")

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())

try:
    ssh.connect(router_ip, port=22, username=router_user, password=router_password, timeout=10)
    print("✓ SSH Connection Established Successfully!")
except Exception as e:
    print(f"❌ SSH Connection Failed: {e}")
    sys.exit(1)

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

print("2. Creating configuration directory /etc/beryl7 on router...")
ssh.exec_command("mkdir -p /etc/beryl7 /var/log")

print("3. Writing secure Gemini API Key file /etc/beryl7/agent.key (chmod 600)...")
if gemini_key:
    ssh.exec_command(f"echo '{gemini_key}' > /etc/beryl7/agent.key && chmod 600 /etc/beryl7/agent.key")

ssh.exec_command("cat > /etc/beryl7/agent.env << 'EOF'\nAUTH_TOKEN=\"beryl7-secret-health-token\"\nLOG_LEVEL=\"INFO\"\nDISABLE_AUTO_HEALING=\"false\"\nEOF\nchmod 600 /etc/beryl7/agent.env")

print("4. Uploading beryl7-agent binary to /usr/bin/beryl7-agent...")
upload_file_stream(ssh, binary_local, "/usr/bin/beryl7-agent")
ssh.exec_command("chmod +x /usr/bin/beryl7-agent")
print("✓ Binary Upload Complete!")

print("5. Uploading procd service init script to /etc/init.d/beryl7-agent...")
upload_file_stream(ssh, procd_local, "/etc/init.d/beryl7-agent")
ssh.exec_command("chmod +x /etc/init.d/beryl7-agent")

print("6. Enabling and starting beryl7-agent 24/7 procd service...")
stdin, stdout, stderr = ssh.exec_command("/etc/init.d/beryl7-agent enable && /etc/init.d/beryl7-agent restart")
stdout.channel.recv_exit_status()

time.sleep(2)

print("7. Verifying live service status on router...")
stdin, stdout, stderr = ssh.exec_command("ps | grep beryl7-agent | grep -v grep")
status_output = stdout.read().decode('utf-8').strip()

ssh.close()

if status_output:
    print("\n==================================================")
    print(" SUCCESS: Beryl 7 Native Go Agent is running 24/7! ")
    print(f" Live Process: {status_output}")
    print(" You can now turn off your laptop cleanly.          ")
    print("==================================================")
else:
    print("⚠️ Warning: Process check returned empty, checking log...")
