import paramiko
import sys

sys.stdout.reconfigure(encoding='utf-8')

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect("192.168.8.1", 22, "root", "Kazuku@2k6")

cmd = 'curl -s -H "Authorization: Bearer beryl7-secret-health-token" http://127.0.0.1:8888/api/health'
stdin, stdout, stderr = ssh.exec_command(cmd)
res = stdout.read().decode('utf-8').strip()

print("Health Check Response:")
print(res)

ssh.close()
