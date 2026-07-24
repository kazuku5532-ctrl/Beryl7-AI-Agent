import os
import sys
import argparse
from pathlib import Path
from dotenv import load_dotenv

# Thêm thư mục gốc vào PYTHONPATH
sys.path.insert(0, str(Path(__file__).resolve().parent.parent))

from agent.orchestrator import SelfEvolvingAgentOrchestrator

load_dotenv()

if sys.platform.startswith('win'):
    try:
        sys.stdout.reconfigure(encoding='utf-8')
    except AttributeError:
        pass

def main():
    parser = argparse.ArgumentParser(description="Launcher chính thức cho Beryl 7 AI Agent")
    parser.add_argument("--host", default="192.168.8.1", help="Địa chỉ IP Router")
    parser.add_argument("--port", type=int, default=22, help="Cổng SSH")
    parser.add_argument("--user", default="root", help="Username SSH")
    parser.add_argument("--password", default="Kazuku@2k6", help="Mật khẩu root SSH")
    parser.add_argument("--live", action="store_true", help="Kích hoạt thực thi lệnh thật xuống Router (Mặc định: Dry-Run an toàn)")

    args = parser.parse_args()
    api_key = os.environ.get("GEMINI_API_KEY")

    if not api_key:
        print("[ERROR] Không tìm thấy GEMINI_API_KEY trong .env! Vui lòng kiểm tra lại file .env.")
        sys.exit(1)

    mode_name = "LIVE (LỆNH THẬT)" if args.live else "DRY-RUN AN TOÀN"
    print(f"[*======== KÍCH HOẠT BERYL 7 AI AGENT [{mode_name}] ========*]")

    orchestrator = SelfEvolvingAgentOrchestrator(
        hostname=args.host,
        port=args.port,
        username=args.user,
        password=args.password,
        dry_run=not args.live
    )

    res = orchestrator.run_self_healing_cycle(api_key=api_key)
    print("\n[OK] Hoàn thành 1 vòng lặp kiểm tra & tự động hóa mạng.")

if __name__ == "__main__":
    main()
