#!/usr/bin/env python3
"""
Test SSH Connectivity Script.
"""
import os
import socket

def test_ssh():
    ip = os.environ.get("ROUTER_IP", "192.168.8.1")
    print(f"Testing SSH Port 22 connectivity to {ip}...")
    try:
        s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        s.settimeout(3)
        res = s.connect_ex((ip, 22))
        s.close()
        if res == 0:
            print("SSH Port 22 is OPEN!")
        else:
            print(f"SSH Port 22 CLOSED or Refused (code {res})")
    except Exception as e:
        print(f"SSH Connection test error: {e}")

if __name__ == "__main__":
    test_ssh()
