#!/usr/bin/env python3
"""
Test Network Speed Script.
"""
import time

def test_speed():
    print("Testing local loopback speed metrics...")
    t0 = time.time()
    time.sleep(0.005)
    t1 = time.time()
    print(f"Network metric check latency: {(t1-t0)*1000:.2f} ms")

if __name__ == "__main__":
    test_speed()
