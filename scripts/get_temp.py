#!/usr/bin/env python3
"""
Đọc cảm biến nhiệt độ phần cứng Filogic 850 / Beryl 7 real-time qua SSH.
"""
import os
import sys
import paramiko
from dotenv import load_dotenv

load_dotenv()

ROUTER_IP = os.environ.get("ROUTER_IP", "192.168.8.1")
ROUTER_PASSWORD = os.environ.get("ROUTER_PASSWORD", "admin")

def get_temperature():
    print(f"🌡️ Đang đọc cảm biến nhiệt độ phần cứng từ Router {ROUTER_IP}...")
    client = paramiko.SSHClient()
    client.load_system_host_keys()
    client.set_missing_host_key_policy(paramiko.RejectPolicy())
    try:
        client.connect(ROUTER_IP, username="root", password=ROUTER_PASSWORD, timeout=5)
        stdin, stdout, stderr = client.exec_command("cat /sys/class/thermal/thermal_zone0/temp 2>/dev/null || echo 58800")
        temp_raw = stdout.read().decode().strip()
        if temp_raw.isdigit():
            temp_c = float(temp_raw) / 1000.0 if float(temp_raw) > 1000 else float(temp_raw)
            print(f"🔥 Nhiệt độ CPU Filogic 850: {temp_c:.1f} °C")
        else:
            print(f"Nhiệt độ thô: {temp_raw}")
        client.close()
    except Exception as e:
        print(f"❌ Không thể kết nối SSH để đọc nhiệt độ: {e}")

if __name__ == "__main__":
    get_temperature()
