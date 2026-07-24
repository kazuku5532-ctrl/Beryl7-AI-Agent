import sys
import json
import time
import argparse
from pathlib import Path

# Thêm thư mục gốc vào PYTHONPATH để import được agent module
sys.path.insert(0, str(Path(__file__).resolve().parent.parent))

from agent.telemetry import RouterTelemetry

if sys.platform.startswith('win'):
    try:
        sys.stdout.reconfigure(encoding='utf-8')
    except AttributeError:
        pass

def main():
    parser = argparse.ArgumentParser(description="Tool thử nghiệm thu thập Router Telemetry từ Beryl 7")
    parser.add_argument("--host", default="192.168.8.1", help="Địa chỉ IP Router")
    parser.add_argument("--port", type=int, default=22, help="Cổng SSH")
    parser.add_argument("--user", default="root", help="Username SSH")
    parser.add_argument("--password", required=True, help="Mật khẩu root SSH")
    
    args = parser.parse_args()
    
    print(f"[*======== BẮT ĐẦU THU THẬP TELEMETRY TỪ BERYL 7 ({args.host}) ========*]")
    
    collector = RouterTelemetry(
        hostname=args.host,
        port=args.port,
        username=args.user,
        password=args.password
    )
    
    # Lần 1: Khởi tạo sample
    print("[1/2] Lấy dữ liệu lần 1 (Khởi tạo baseline)...")
    data_1 = collector.get_normalized_telemetry()
    
    if data_1.get("status") == "error":
        print(f"[ERROR] Thất bại: {data_1.get('message')}")
        sys.exit(1)
        
    print(f"[OK] Tìm thấy {data_1['connected_clients_count']} thiết bị đang kết nối Wi-Fi.")
    print("Wait 3 giây để tính tốc độ mạng Realtime (Delta KB/s)...")
    time.sleep(3)
    
    # Lần 2: Tính tốc độ delta
    print("[2/2] Lấy dữ liệu lần 2 (Tính toán băng thông Realtime)...")
    data_2 = collector.get_normalized_telemetry()
    
    print("\n================ [ CHUẨN HÓA TELEMETRY SCHEMA (JSON) ] ================")
    print(json.dumps(data_2, indent=2, ensure_ascii=False))
    print("================================================-------------------------\n")
    print("[SUCCESS] THU THẬP TELEMETRY THÀNH CÔNG 100%!")

if __name__ == "__main__":
    main()
