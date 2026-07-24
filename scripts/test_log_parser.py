import sys
import json
import argparse
from pathlib import Path

# Thêm thư mục gốc vào PYTHONPATH
sys.path.insert(0, str(Path(__file__).resolve().parent.parent))

from agent.telemetry import RouterTelemetry
from agent.log_parser import RouterLogParser

if sys.platform.startswith('win'):
    try:
        sys.stdout.reconfigure(encoding='utf-8')
    except AttributeError:
        pass

def main():
    parser = argparse.ArgumentParser(description="Tool thử nghiệm Log Parser & Anomaly Detection cho Beryl 7")
    parser.add_argument("--host", default="192.168.8.1", help="Địa chỉ IP Router")
    parser.add_argument("--port", type=int, default=22, help="Cổng SSH")
    parser.add_argument("--user", default="root", help="Username SSH")
    parser.add_argument("--password", required=True, help="Mật khẩu root SSH")
    
    args = parser.parse_args()
    
    print(f"[*======== BẮT ĐẦU KIỂM THỬ LOG PARSER & ANOMALY DETECTION ({args.host}) ========*]")
    
    # 1. Thu thập Telemetry
    print("[1/2] Đang thu thập Telemetry để làm dữ liệu nền...")
    telemetry_collector = RouterTelemetry(
        hostname=args.host,
        port=args.port,
        username=args.user,
        password=args.password
    )
    telemetry_data = telemetry_collector.get_normalized_telemetry()
    
    # 2. Phân tích Log & Phát hiện Bất thường
    print("[2/2] Đang thu thập logread và chạy Engine phát hiện bất thường...")
    log_parser = RouterLogParser(
        hostname=args.host,
        port=args.port,
        username=args.user,
        password=args.password
    )
    anomaly_result = log_parser.detect_anomalies(telemetry_data=telemetry_data)
    
    print("\n================ [ KẾT QUẢ ANOMALY DETECTION SCHEMA (JSON) ] ================")
    print(json.dumps(anomaly_result, indent=2, ensure_ascii=False))
    print("================================================------------------------------\n")
    
    if anomaly_result["has_anomalies"]:
        print(f"⚠️ CẢNH BÁO: Phát hiện {anomaly_result['total_anomalies']} bất thường (Cấp độ cao nhất: {anomaly_result['max_severity']})!")
    else:
        print("✅ HỆ THỐNG HOẠT ĐỘNG BÌNH THƯỜNG: Không phát hiện bất thường nào!")
        
    print("\n[SUCCESS] THỰC THI PHASE 2.2 THÀNH CÔNG 100%!")

if __name__ == "__main__":
    main()
