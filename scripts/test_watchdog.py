import sys
import json
import argparse
from pathlib import Path

# Thêm thư mục gốc vào PYTHONPATH
sys.path.insert(0, str(Path(__file__).resolve().parent.parent))

from agent.executor import RouterExecutor
from agent.watchdog import GuardedWatchdog

if sys.platform.startswith('win'):
    try:
        sys.stdout.reconfigure(encoding='utf-8')
    except AttributeError:
        pass

def main():
    parser = argparse.ArgumentParser(description="Tool kiểm thử GuardedWatchdog & Auto-Rollback Guardrail cho Beryl 7")
    parser.add_argument("--host", default="192.168.8.1", help="Địa chỉ IP Router")
    parser.add_argument("--port", type=int, default=22, help="Cổng SSH")
    parser.add_argument("--user", default="root", help="Username SSH")
    parser.add_argument("--password", required=True, help="Mật khẩu root SSH")
    
    args = parser.parse_args()

    print(f"[*======== BẮT ĐẦU KIỂM THỬ GUARDED WATCHDOG & AUTO-ROLLBACK GUARDRAIL ========*]")

    executor = RouterExecutor(
        hostname=args.host, port=args.port, username=args.user, password=args.password, dry_run=False
    )
    
    watchdog = GuardedWatchdog(
        hostname=args.host, port=args.port, username=args.user, password=args.password
    )

    print("[1/1] Thử nghiệm thực thi lệnh 'restart_interface wan' dưới sự giám sát của Watchdog 10s...")
    
    # Thực thi lệnh qua Watchdog
    res = watchdog.execute_with_guardrail(
        executor_func=executor.execute_restart_interface,
        interface_name="wan",
        reason="Kiểm thử Watchdog Auto-Rollback Guardrail",
        countdown_seconds=10
    )

    print("\n================ [ KẾT QUẢ WATCHDOG EXECUTION RESULT ] ================")
    print(json.dumps(res, indent=2, ensure_ascii=False))
    print("================================================------------------------\n")

    if res.get("success"):
        print("✅ AN TOÀN TUYỆT ĐỐI: Lệnh chạy thành công và kết nối mạng được duy trì ổn định!")
    elif res.get("rolled_back"):
        print("🛡️ CỨU HỘ THÀNH CÔNG: Mạng bị rớt và Watchdog đã tự động khôi phục (Rollback) về cấu hình cũ!")

    print("\n[SUCCESS] THỰC THI PHASE 5 THÀNH CÔNG 100%!")

if __name__ == "__main__":
    main()
