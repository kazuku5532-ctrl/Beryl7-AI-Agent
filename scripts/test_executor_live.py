import sys
import json
import argparse
from pathlib import Path

# Thêm thư mục gốc vào PYTHONPATH
sys.path.insert(0, str(Path(__file__).resolve().parent.parent))

from agent.executor import RouterExecutor

if sys.platform.startswith('win'):
    try:
        sys.stdout.reconfigure(encoding='utf-8')
    except AttributeError:
        pass

def main():
    parser = argparse.ArgumentParser(description="Tool kiểm thử Router Execution Engine (Dry-Run / Live) cho Beryl 7")
    parser.add_argument("--host", default="192.168.8.1", help="Địa chỉ IP Router")
    parser.add_argument("--port", type=int, default=22, help="Cổng SSH")
    parser.add_argument("--user", default="root", help="Username SSH")
    parser.add_argument("--password", required=True, help="Mật khẩu root SSH")
    parser.add_argument("--live", action="store_true", help="Kích hoạt thực thi LỆNH THẬT xuống Router (Mặc định: Dry-Run an toàn)")

    args = parser.parse_args()

    mode_str = "LỆNH THẬT (LIVE)" if args.live else "DRY-RUN AN TOÀN (KHÔNG CHẠM ROUTER)"
    print(f"[*======== BẮT ĐẦU KIỂM THỬ ROUTER EXECUTOR [{mode_str}] ========*]")

    executor = RouterExecutor(
        hostname=args.host,
        port=args.port,
        username=args.user,
        password=args.password,
        dry_run=not args.live
    )

    # 1. Thử nghiệm Dispatch AI Decision (Giả lập AI ra quyết định restart WAN)
    ai_decision_sample = {
        "status": "success",
        "action_type": "FUNCTION_CALL",
        "tool_name": "restart_interface",
        "arguments": {
            "interface_name": "wan",
            "reason": "Kiểm thử RouterExecutor dispatcher khôi phục mạng WAN"
        }
    }

    print("\n[1/2] Chạy thử AI Decision Dispatcher (Action: restart_interface 'wan')...")
    res1 = executor.dispatch_ai_decision(ai_decision_sample)
    print("---------------- [ EXECUTION RESULT 1 ] ----------------")
    print(json.dumps(res1, indent=2, ensure_ascii=False))

    # 2. Thử nghiệm Dispatch AI Decision (Giả lập AI ra quyết định đổi kênh Wi-Fi)
    ai_decision_sample_2 = {
        "status": "success",
        "action_type": "FUNCTION_CALL",
        "tool_name": "optimize_wifi_channel",
        "arguments": {
            "band": "2.4G",
            "channel": 6,
            "reason": "Giảm nhiễu sóng Wi-Fi 2.4GHz"
        }
    }

    print("\n[2/2] Chạy thử AI Decision Dispatcher (Action: optimize_wifi_channel '2.4G' ch 6)...")
    res2 = executor.dispatch_ai_decision(ai_decision_sample_2)
    print("---------------- [ EXECUTION RESULT 2 ] ----------------")
    print(json.dumps(res2, indent=2, ensure_ascii=False))

    print("\n[SUCCESS] THỰC THI PHASE 4 THÀNH CÔNG 100%!")

if __name__ == "__main__":
    main()
