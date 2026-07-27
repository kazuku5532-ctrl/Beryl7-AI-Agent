#!/usr/bin/env python3
"""
Kiểm tra Live HTTP Health Endpoint /api/health của Daemon beryl7-agent.
"""
import os
import sys
import requests
from dotenv import load_dotenv

load_dotenv()

ROUTER_IP = os.environ.get("ROUTER_IP", "192.168.8.1")
HEALTH_PORT = int(os.environ.get("HEALTH_PORT", 8888))
AUTH_TOKEN = os.environ.get("AUTH_TOKEN", "")

def check_health():
    url = f"http://{ROUTER_IP}:{HEALTH_PORT}/api/health"
    headers = {"Authorization": f"Bearer {AUTH_TOKEN}"} if AUTH_TOKEN else {}
    print(f"📡 Đang kết nối tới Health Check Endpoint: {url}")
    try:
        r = requests.get(url, headers=headers, timeout=5)
        print(f"HTTP Status: {r.status_code}")
        if r.status_code == 200:
            print("Response JSON:")
            print(r.json())
        else:
            print("Response Text:", r.text)
    except Exception as e:
        print(f"❌ Lỗi kết nối Health Check: {e}")

if __name__ == "__main__":
    check_health()
