import os
import sys
import json
import argparse
from pathlib import Path
from dotenv import load_dotenv

# Thêm thư mục gốc vào PYTHONPATH
sys.path.insert(0, str(Path(__file__).resolve().parent.parent))

from agent.telemetry import RouterTelemetry
from agent.log_parser import RouterLogParser
from agent.ai_client import RouterAIAgent

load_dotenv()

if sys.platform.startswith('win'):
    try:
        sys.stdout.reconfigure(encoding='utf-8')
    except AttributeError:
        pass

def main():
    parser = argparse.ArgumentParser(description="Tool kiểm thử Gemini AI Agent & Function Calling cho Beryl 7")
    parser.add_argument("--host", default="192.168.8.1", help="Địa chỉ IP Router")
    parser.add_argument("--port", type=int, default=22, help="Cổng SSH")
    parser.add_argument("--user", default="root", help="Username SSH")
    parser.add_argument("--password", required=True, help="Mật khẩu root SSH")
    parser.add_argument("--api-key", default=None, help="Google Gemini API Key (Mặc định nạp từ .env)")
    parser.add_argument("--simulate-wan-drop", action="store_true", help="Giả lập tình huống rớt WAN để test AI Action")

    args = parser.parse_args()
    api_key = args.api_key or os.environ.get("GEMINI_API_KEY")

    if not api_key:
        print("[ERROR] Không tìm thấy GEMINI_API_KEY trong .env hoặc tham số truyền vào!")
        sys.exit(1)

    print(f"[*======== BẮT ĐẦU KIỂM THỬ GEMINI AI AGENT & FUNCTION CALLING ========*]")

    # 1. Thu thập Telemetry & Anomaly Report thực tế từ Beryl 7
    print("[1/3] Đang thu thập Telemetry & Anomaly Report thực tế từ Beryl 7...")
    telemetry_collector = RouterTelemetry(
        hostname=args.host, port=args.port, username=args.user, password=args.password
    )
    telemetry_data = telemetry_collector.get_normalized_telemetry()

    log_parser = RouterLogParser(
        hostname=args.host, port=args.port, username=args.user, password=args.password
    )
    anomaly_data = log_parser.detect_anomalies(telemetry_data=telemetry_data)

    # 2. Giả lập kịch bản sự cố nếu người dùng chọn
    if args.simulate_wan_drop:
        print("⚠️ [SIMULATION MODE] Giả lập sự cố: Mất kết nối WAN Interface...")
        anomaly_data["has_anomalies"] = True
        anomaly_data["max_severity"] = "CRITICAL"
        anomaly_data["anomalies"].append({
            "severity": "CRITICAL",
            "category": "WAN",
            "event_name": "WAN_INTERFACE_DROPPED",
            "message": "Cổng WAN bị ngắt kết nối mạng rớt gói tin 100%.",
            "sample_log": "netifd: Interface 'wan' is now down"
        })
        telemetry_data["wan"]["is_connected"] = False
        telemetry_data["wan"]["ip_address"] = "Disconnected"

    # 3. Khởi tạo Gemini AI Agent và gửi prompt
    print("[2/3] Khởi tạo Gemini 2.0 Flash AI Agent (Dùng API Key thực tế)...")
    try:
        ai_agent = RouterAIAgent(api_key=api_key)
    except Exception as e:
        print(f"[ERROR] Lỗi khởi tạo AI Agent: {e}")
        sys.exit(1)

    print("[3/3] Gửi dữ liệu tới Gemini AI và chờ quyết định Function Calling...")
    decision = ai_agent.analyze_and_decide(telemetry_data, anomaly_data)

    print("\n================ [ KẾT QUẢ AI DECISION THỰC TẾ (JSON FUNCTION CALL) ] ================")
    print(json.dumps(decision, indent=2, ensure_ascii=False))
    print("================================================---------------------------------------\n")

    if decision.get("action_type") == "FUNCTION_CALL":
        print(f"🎯 AI ĐÃ ĐƯA RA QUYẾT ĐỊNH HÀNH ĐỘNG THỰC TẾ: Call Tool -> '{decision['tool_name']}'")
        print(f"📌 Tham số (Arguments): {decision['arguments']}")
    else:
        print(f"💬 AI Phản hồi dạng văn bản: {decision.get('response_text')}")

    print("\n[SUCCESS] THỰC THI KIỂM THỬ THỰC TẾ GEMINI API THÀNH CÔNG 100%!")

if __name__ == "__main__":
    main()
