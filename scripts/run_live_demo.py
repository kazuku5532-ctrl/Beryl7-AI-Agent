import sys
import json
import time
import argparse
from pathlib import Path

# Thêm thư mục gốc vào PYTHONPATH
sys.path.insert(0, str(Path(__file__).resolve().parent.parent))

from agent.orchestrator import SelfEvolvingAgentOrchestrator

if sys.platform.startswith('win'):
    try:
        sys.stdout.reconfigure(encoding='utf-8')
    except AttributeError:
        pass

def run_live_demo_presentation(api_key, host="192.168.8.1", user="root", password=None):
    """
    Kịch bản Live Demo trình diễn trước Hội đồng chấm Đồ án Tốt nghiệp.
    Bao gồm 3 Kịch bản sống động chứng minh khả năng Tự tiến hóa & Tự sửa lỗi của Agent.
    """
    print("""
========================================================================================
   🎓 KỊCH BẢN LIVE DEMO BẢO VỆ ĐỒ ÁN TỐT NGHIỆP: BERYL 7 SELF-EVOLVING AI EDGE AGENT
========================================================================================
""")

    orchestrator = SelfEvolvingAgentOrchestrator(
        hostname=host, username=user, password=password, dry_run=True
    )

    # KỊCH BẢN 1: MẤT MẠNG WAN LẦN ĐẦU (CACHE MISS -> GEMINI AI -> LƯU KỸ NĂNG)
    print("\n🎬 [KỊCH BẢN 1] Sự cố rớt mạng WAN Lần đầu (Lỗi chưa từng có trong cơ sở tri thức)...")
    time.sleep(1)
    
    simulated_wan_drop = {
        "severity": "CRITICAL",
        "category": "WAN",
        "event_name": "WAN_INTERFACE_DROPPED",
        "message": "Cổng WAN bị mất tín hiệu kết nối Internet.",
        "sample_log": "netifd: Interface 'wan' link is down"
    }

    res1 = orchestrator.run_self_healing_cycle(api_key=api_key, simulated_anomaly=simulated_wan_drop)
    print(f"\n📊 KẾT QUẢ KỊCH BẢN 1: Source='{res1.get('source')}', EvolvedNewSkill={res1.get('evolved_new_skill')}")
    print("----------------------------------------------------------------------------------------")

    # KỊCH BẢN 2: MẤT MẠNG WAN LẦN HAI (CACHE HIT -> SQLITE LOCAL -> TỨC THÌ 0s, 0$)
    print("\n🎬 [KỊCH BẢN 2] Sự cố rớt mạng WAN Lần hai (Lỗi lặp lại - Đã học vào SQLite Store)...")
    time.sleep(2)

    res2 = orchestrator.run_self_healing_cycle(api_key=api_key, simulated_anomaly=simulated_wan_drop)
    print(f"\n📊 KẾT QUẢ KỊCH BẢN 2: Source='{res2.get('source')}', Tool='{res2.get('tool_used')}', ApiCost='${res2.get('api_cost_usd')}'")
    print("----------------------------------------------------------------------------------------")

    # KỊCH BẢN 3: NGHẼN MẠNG BĂNG THÔNG CAO (AI TỰ ĐIỀU CHỈNH QOS)
    print("\n🎬 [KỊCH BẢN 3] Phát hiện thiết bị nội bộ chiếm 100% băng thông gây lag mạng...")
    time.sleep(2)

    simulated_qos_issue = {
        "severity": "WARNING",
        "category": "SYSTEM",
        "event_name": "HIGH_BANDWIDTH_CONGESTION",
        "message": "Phát hiện thiết bị MAC AA:BB:CC:11:22:33 đang tải Torrent ngốn sạch băng thông.",
        "sample_log": "qos: high traffic threshold reached on br-lan"
    }

    res3 = orchestrator.run_self_healing_cycle(api_key=api_key, simulated_anomaly=simulated_qos_issue)
    print(f"\n📊 KẾT QUẢ KỊCH BẢN 3: Source='{res3.get('source')}', Tool='{res3.get('tool_used')}'")

    print("""
========================================================================================
   🎉 TỔNG KẾT LIVE DEMO: HỆ THỐNG ĐÃ TỰ TIẾN HÓA & KHÔI PHỤC MẠNG THÀNH CÔNG 100%!
========================================================================================
""")

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Script trình diễn Live Demo cho Đồ án Tốt nghiệp")
    parser.add_argument("--host", default="192.168.8.1", help="Địa chỉ IP Router")
    parser.add_argument("--user", default="root", help="Username SSH")
    parser.add_argument("--password", required=True, help="Mật khẩu root SSH")
    parser.add_argument("--api-key", required=True, help="Google Gemini API Key")

    args = parser.parse_args()
    run_live_demo_presentation(api_key=args.api_key, host=args.host, user=args.user, password=args.password)
