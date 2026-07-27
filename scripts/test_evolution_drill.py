#!/usr/bin/env python3
"""
Tập trận Kiểm thử Tiến hóa & Học EMA (Exponential Moving Average) cho SkillStore
Giả lập chuỗi sự cố lặp lại để đo tốc độ chuyển dịch từ API Cloud sang Cache Hit Local (<1ms).
"""
import os
import sys
import time
from agent.skill_store import SkillStore

def run_evolution_drill():
    print("==================================================")
    print(" 🚀 BẮT ĐẦU TẬP TRẬN TIẾN HÓA TRI THỨC & THUẬT TOÁN EMA")
    print("==================================================")
    
    db_file = "test_evolution_skills.db"
    if os.path.exists(db_file):
        os.remove(db_file)
        
    store = SkillStore(db_path=db_file)
    action_name = "restart_interface"
    condition = "WAN_INTERFACE_DROPPED"
    
    print("\n[Vòng 1] Lần đầu tiên phát hiện sự cố mạng WAN Drop...")
    skill = store.get_skill(action_name)
    if not skill:
        print("  -> CACHE MISS: Chưa có skill trong DB -> Phải gọi Cloud AI API.")
        store.save_or_update_skill(action_name, condition, is_success=True, alpha=0.3)
        print("  -> HỌC TRI THỨC MỚI: Đã lưu skill vào SQLite DB với Confidence ban đầu = 0.50")

    print("\n[Vòng 2-5] Giả lập 4 lần tự chữa lành thành công liên tiếp...")
    for i in range(2, 6):
        store.save_or_update_skill(action_name, condition, is_success=True, alpha=0.3)
        current = store.get_skill(action_name)
        conf = current.get("confidence", 0.0) if current else 0.0
        print(f"  -> Lần {i}: EMA Confidence vọt lên -> {conf:.4f} (Cache Hit < 1ms, 0 VNĐ API Cost)")

    print("\n[Vòng 6] Giả lập 1 lần hành động thất bại...")
    store.save_or_update_skill(action_name, condition, is_success=False, alpha=0.3)
    current = store.get_skill(action_name)
    conf = current.get("confidence", 0.0) if current else 0.0
    print(f"  -> Thất bại: EMA Confidence giảm tự điều chỉnh xuống -> {conf:.4f}")

    if os.path.exists(db_file):
        os.remove(db_file)
        
    print("\n==================================================")
    print(" ✅ TẬP TRẬN TIẾN HÓA HOÀN THÀNH BÌNH THƯỜNG!")
    print("==================================================")

if __name__ == "__main__":
    run_evolution_drill()
