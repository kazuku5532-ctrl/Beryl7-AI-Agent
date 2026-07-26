import os
import sys
import paramiko
from dotenv import load_dotenv

sys.stdout.reconfigure(encoding='utf-8')

load_dotenv()
router_ip = os.getenv("ROUTER_IP", "192.168.8.1")
router_user = os.getenv("ROUTER_USER", "root")
router_password = os.getenv("ROUTER_PASSWORD")
auth_token = os.getenv("AUTH_TOKEN")

if not router_password:
    print("❌ ERROR: ROUTER_PASSWORD environment variable not set in .env file!")
    print("Please configure ROUTER_PASSWORD in your .env file before running this script.")
    sys.exit(1)

if not auth_token:
    print("❌ ERROR: AUTH_TOKEN environment variable not set in .env file!")
    sys.exit(1)

ssh = paramiko.SSHClient()
ssh.load_system_host_keys()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())

try:
    ssh.connect(router_ip, 22, router_user, router_password, timeout=10)
except Exception as e:
    print(f"SSH Connection Error: {e}")
    sys.exit(1)

cmd = f'curl -s -H "Authorization: Bearer {auth_token}" http://127.0.0.1:8888/api/health'
stdin, stdout, stderr = ssh.exec_command(cmd)
res = stdout.read().decode('utf-8').strip()

print("Health Check Response:")
print(res)

ssh.close()
