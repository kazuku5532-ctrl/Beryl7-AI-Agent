import paramiko
import sys

sys.stdout.reconfigure(encoding='utf-8')

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect("192.168.8.1", 22, "root", "Kazuku@2k6")

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
