import unittest
from agent.skill_store import SkillStore

class TestSkillStore(unittest.TestCase):

    def setUp(self):
        # Dùng :memory: database cho Unit Test an toàn trên Windows
        self.store = SkillStore(db_path=":memory:")

    def test_save_and_get_skill(self):
        sig = self.store.generate_signature("CRITICAL", "WAN", "WAN_INTERFACE_DROPPED")
        self.store.save_or_update_skill(
            error_signature=sig,
            category="WAN",
            event_name="WAN_INTERFACE_DROPPED",
            tool_name="restart_interface",
            arguments={"interface_name": "wan"}
        )

        skill = self.store.get_skill(sig)
        self.assertIsNotNone(skill)
        self.assertEqual(skill["tool_name"], "restart_interface")
        self.assertEqual(skill["arguments"]["interface_name"], "wan")
        self.assertEqual(skill["confidence_score"], 1.0)

    def test_ema_confidence_score_growth(self):
        """Test thuật toán EMA giúp điểm tin cậy cập nhật mượt mà khi học lặp lại"""
        sig = self.store.generate_signature("WARNING", "WIFI", "REPEATED_DISCONNECTS")
        self.store.save_or_update_skill(sig, "WIFI", "REPEATED_DISCONNECTS", "optimize_wifi_channel", {"band": "5G", "channel": 36})
        self.store.save_or_update_skill(sig, "WIFI", "REPEATED_DISCONNECTS", "optimize_wifi_channel", {"band": "5G", "channel": 36})

        skill = self.store.get_skill(sig)
        self.assertEqual(skill["success_count"], 2)
        self.assertEqual(skill["confidence_score"], 1.0)

    def test_record_failure_reduces_confidence(self):
        """Test cơ chế giảm điểm suy giảm (Decay 50%) khi thất bại"""
        sig = self.store.generate_signature("CRITICAL", "SYSTEM", "OOM_ERROR")
        self.store.save_or_update_skill(sig, "SYSTEM", "OOM_ERROR", "restart_interface", {"interface_name": "br-lan"})
        
        # Thất bại làm confidence giảm từ 1.0 xuống 0.5 (Decay factor 0.5)
        self.store.record_failure(sig)
        
        skill = self.store.get_skill(sig, min_confidence=0.1)
        self.assertIsNotNone(skill)
        self.assertEqual(round(skill["confidence_score"], 1), 0.5)

    def test_skill_pruning_low_confidence(self):
        """Test tự động ẩn/đào thải kỹ năng khi confidence_score < 0.5"""
        sig = self.store.generate_signature("CRITICAL", "SYSTEM", "OOM_ERROR")
        self.store.save_or_update_skill(sig, "SYSTEM", "OOM_ERROR", "restart_interface", {"interface_name": "br-lan"})
        
        # Lần 1 thất bại: 1.0 -> 0.5
        self.store.record_failure(sig)
        # Lần 2 thất bại: 0.5 -> 0.25 (< 0.5)
        self.store.record_failure(sig)

        skill = self.store.get_skill(sig, min_confidence=0.5)
        self.assertIsNone(skill) # Đã bị đào thải không lấy ra nữa!

if __name__ == "__main__":
    unittest.main()
