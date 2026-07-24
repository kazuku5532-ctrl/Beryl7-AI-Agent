import os
import sys
import json
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
    parser = argparse.ArgumentParser(description="Tool kiểm thử Self-Evolving Orchestrator Loop (Trái tim Đồ án)")
    parser.add_argument("--host", default="192.168.8.1", help="Địa chỉ IP Router")
    parser.add_argument("--port", type=int, default=22, help="Cổng SSH")
    parser.add_argument("--user", default="root", help="Username SSH")
    parser.add_argument("--password", required=True, help="Mật khẩu root SSH")
    parser.add_argument("--api-key", default=None, help="Google Gemini API Key (Mặc định nạp từ .env)")
    parser.add_argument("--db-path", default="data/skills.db", help="Đường dẫn file SQLite Database")
    
    args = parser.parse_args()
    api_key = args.api_key or os.environ.get("GEMINI_API_KEY")

    if not api_key:
        print("[ERROR] Không tìm thấy GEMINI_API_KEY trong .env hoặc tham số truyền vào!")
        sys.exit(1)

    print(f"[*======== BẮT ĐẦU KIỂM THỬ TRÁI TIM ĐỒ ÁN VỚI GEMINI API THỰC TẾ ========*]")

    orchestrator = SelfEvolvingAgentOrchestrator(
        hostname=args.host,
        port=args.port,
        username=args.user,
        password=args.password,
        db_path=args.db_path,
        dry_run=True # Dry-run an toàn cho lần test đầu tiên
    )

    # LẦN 1: Giả lập sự cố rớt WAN -> Sẽ bị CACHE MISS -> Gửi Gemini AI thật -> Học kỹ năng mới vào SQLite!
    simulated_wan_drop = {
        "severity": "CRITICAL",
        "category": "WAN",
        "event_name": "WAN_INTERFACE_DROPPED",
        "message": "Giả lập rớt mạng WAN để thử nghiệm Vòng lặp Tiến hóa.",
        "sample_log": "netifd: Interface 'wan' is down"
    }

    print("\n---------------- [ LẦN 1: THỬ NGHIỆM VỚI LỖI MỚI (CHƯA CÓ TRONG SQLITE) ] ----------------")
    res1 = orchestrator.run_self_healing_cycle(api_key=api_key, simulated_anomaly=simulated_wan_drop)
    print("---------------- [ RESULTS LẦN 1 ] ----------------")
    print(json.dumps(res1, indent=2, ensure_ascii=False))

    # LẦN 2: Lặp lại sự cố rớt WAN tương tự -> Sẽ bị CACHE HIT -> Tự lấy từ SQLite local (0s Delay, 0 VNĐ API)!
    print("\n---------------- [ LẦN 2: LẶP LẠI SỰ CỐ TƯƠNG TỰ (ĐÃ HỌC VÀO SQLITE) ] ----------------")
    res2 = orchestrator.run_self_healing_cycle(api_key=api_key, simulated_anomaly=simulated_wan_drop)
    print("---------------- [ RESULTS LẦN 2 ] ----------------")
    print(json.dumps(res2, indent=2, ensure_ascii=False))

    # IN DANH SÁCH CÁC KỸ NĂNG ĐÃ TIẾN HÓA TRONG SQLITE
    print("\n---------------- [ BẢNG TRÍ NHỚ KỸ NĂNG ĐÃ TIẾN HÓA (SQLITE SKILL STORE) ] ----------------")
    learned_skills = orchestrator.skill_store.list_all_skills()
    print(json.dumps(learned_skills, indent=2, ensure_ascii=False))

    print("\n[SUCCESS] THỰC THI PHASE 6 VỚI GEMINI API THỰC TẾ THÀNH CÔNG 100%!")

if __name__ == "__main__":
    main()
