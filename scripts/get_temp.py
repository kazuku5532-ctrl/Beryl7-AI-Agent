import os
import sys
import paramiko
from dotenv import load_dotenv

sys.stdout.reconfigure(encoding='utf-8')

load_dotenv()
router_ip = os.getenv("ROUTER_IP", "192.168.8.1")
router_user = os.getenv("ROUTER_USER", "root")
router_password = os.getenv("ROUTER_PASSWORD")

if not router_password:
    print("❌ ERROR: ROUTER_PASSWORD environment variable not set in .env file!")
    sys.exit(1)

ssh = paramiko.SSHClient()
ssh.load_system_host_keys()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())

try:
    ssh.connect(router_ip, 22, router_user, router_password, timeout=10)
except Exception as e:
    print(f"SSH Connection Error: {e}")
    sys.exit(1)

cmd = """
for f in /sys/class/thermal/thermal_zone*/temp /sys/class/thermal/thermal_zone*/type /sys/class/hwmon/hwmon*/temp1_input; do
    if [ -f "$f" ]; then
        echo "$f: $(cat $f)"
    fi
done
"""

stdin, stdout, stderr = ssh.exec_command(cmd)
res = stdout.read().decode('utf-8').strip()

print("Thermal Sensor Output:")
print(res)

ssh.close()
