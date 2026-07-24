import os
import shutil
import unittest
import tempfile
from agent.skill_store import SkillStore

class TestSkillStore(unittest.TestCase):
    """
    Unit Tests cho SQLite Skill Store và thuật toán Confidence Scoring (Chạy local 100%).
    """

    def setUp(self):
        self.temp_dir = tempfile.mkdtemp()
        self.temp_db_path = os.path.join(self.temp_dir, "skills_test.db")
        self.skill_store = SkillStore(db_path=self.temp_db_path)

    def tearDown(self):
        del self.skill_store
        shutil.rmtree(self.temp_dir, ignore_errors=True)

    def test_save_and_get_skill(self):
        sig = SkillStore.generate_signature("CRITICAL", "WAN", "WAN_INTERFACE_DROPPED")
        self.skill_store.save_or_update_skill(
            error_signature=sig,
            category="WAN",
            event_name="WAN_INTERFACE_DROPPED",
            tool_name="restart_interface",
            arguments={"interface_name": "wan", "reason": "Fix WAN drop"}
        )

        skill = self.skill_store.get_skill(sig)
        self.assertIsNotNone(skill)
        self.assertEqual(skill["tool_name"], "restart_interface")
        self.assertEqual(skill["arguments"]["interface_name"], "wan")
        self.assertEqual(skill["success_count"], 1)

    def test_skill_evolution_increments_success(self):
        sig = SkillStore.generate_signature("WARNING", "WIFI", "REPEATED_DISCONNECTS")
        
        # Lần 1: Học kỹ năng mới
        self.skill_store.save_or_update_skill(
            error_signature=sig, category="WIFI", event_name="REPEATED_DISCONNECTS",
            tool_name="optimize_wifi_channel", arguments={"band": "2.4G", "channel": 6}
        )
        
        # Lần 2: Tái sử dụng kỹ năng -> Success count tăng lên 2
        self.skill_store.save_or_update_skill(
            error_signature=sig, category="WIFI", event_name="REPEATED_DISCONNECTS",
            tool_name="optimize_wifi_channel", arguments={"band": "2.4G", "channel": 6}
        )

        skill = self.skill_store.get_skill(sig)
        self.assertEqual(skill["success_count"], 2)
        self.assertGreater(skill["confidence_score"], 1.0 - 0.01)

    def test_record_failure_reduces_confidence(self):
        sig = SkillStore.generate_signature("CRITICAL", "KERNEL", "OOM_ERROR")
        self.skill_store.save_or_update_skill(
            error_signature=sig, category="KERNEL", event_name="OOM_ERROR",
            tool_name="restart_interface", arguments={"interface_name": "lan"}
        )

        # Giảm confidence score do thực thi thất bại
        self.skill_store.record_failure(sig)
        
        # Với confidence score 0.6 >= 0.5 vẫn lấy được
        skill = self.skill_store.get_skill(sig, min_confidence=0.5)
        self.assertIsNotNone(skill)
        self.assertEqual(round(skill["confidence_score"], 1), 0.6)

        # Giảm tiếp lần 2 -> confidence score thành 0.2 < 0.5 (Đào thải kỹ năng hỏng)
        self.skill_store.record_failure(sig)
        skill_after = self.skill_store.get_skill(sig, min_confidence=0.5)
        self.assertIsNone(skill_after) # Đã bị loại khỏi cache!

if __name__ == "__main__":
    unittest.main()
