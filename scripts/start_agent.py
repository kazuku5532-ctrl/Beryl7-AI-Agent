import os
import sys
import argparse
from pathlib import Path
from dotenv import load_dotenv

# Thêm thư mục gốc vào PYTHONPATH
sys.path.insert(0, str(Path(__file__).resolve().parent.parent))

from agent.orchestrator import SelfEvolvingAgentOrchestrator
from agent.logger import agent_logger

load_dotenv()

def main():
    parser = argparse.ArgumentParser(description="Launcher chính thức cho Beryl 7 AI Agent")
    parser.add_argument("--host", default=os.environ.get("ROUTER_HOST", "192.168.8.1"), help="Địa chỉ IP Router (Mặc định đọc từ .env)")
    parser.add_argument("--port", type=int, default=int(os.environ.get("ROUTER_PORT", 22)), help="Cổng SSH")
    parser.add_argument("--user", default=os.environ.get("ROUTER_USER", "root"), help="Username SSH")
    parser.add_argument("--password", default=os.environ.get("ROUTER_PASSWORD"), help="Mật khẩu root SSH (Mặc định đọc từ .env)")
    parser.add_argument("--live", action="store_true", help="Kích hoạt thực thi lệnh thật xuống Router (Mặc định: Dry-Run an toàn)")

    args = parser.parse_args()
    api_key = os.environ.get("GEMINI_API_KEY")
    password = args.password or os.environ.get("ROUTER_PASSWORD")

    if not password:
        agent_logger.error("❌ Không tìm thấy ROUTER_PASSWORD trong .env hoặc tham số truyền vào!")
        sys.exit(1)

    if not api_key:
        agent_logger.error("❌ Không tìm thấy GEMINI_API_KEY trong .env! Vui lòng kiểm tra lại file .env.")
        sys.exit(1)

    mode_name = "LIVE (LỆNH THẬT)" if args.live else "DRY-RUN AN TOÀN"
    agent_logger.info(f"[*======== KÍCH HOẠT BERYL 7 AI AGENT [{mode_name}] ========*]")

    orchestrator = SelfEvolvingAgentOrchestrator(
        hostname=args.host,
        port=args.port,
        username=args.user,
        password=password,
        dry_run=not args.live
    )

    res = orchestrator.run_self_healing_cycle(api_key=api_key)
    agent_logger.info("[OK] Hoàn thành 1 vòng lặp kiểm tra & tự động hóa mạng.")

if __name__ == "__main__":
    main()
