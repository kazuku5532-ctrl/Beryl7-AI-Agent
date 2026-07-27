#!/usr/bin/env python3
"""
Khởi chạy Python Secondary Offloaded Agent từ Laptop.
"""
import sys
from agent.orchestrator import SelfEvolvingAgentOrchestrator

def main():
    print("🚀 Bắt đầu khởi chạy Python Backup Agent...")
    agent = SelfEvolvingAgentOrchestrator()
    agent.run_single_loop()

if __name__ == "__main__":
    main()
