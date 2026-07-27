"""Unit tests for SkillStore module.
"""
import os
import unittest
from agent.skill_store import SkillStore


class TestSkillStore(unittest.TestCase):
    """Test suite verifying SkillStore SQLite caching, EMA scoring, and decay."""

    def setUp(self) -> None:
        self.test_db = "tests/unit_test_skills.db"
        if os.path.exists(self.test_db):
            os.remove(self.test_db)
        self.store = SkillStore(db_path=self.test_db)

    def tearDown(self) -> None:
        self.store.close()
        if os.path.exists(self.test_db):
            os.remove(self.test_db)

    def test_generate_signature_deterministic(self) -> None:
        sig1 = self.store.generate_signature("CRITICAL", "WAN", "WAN_DROP")
        sig2 = self.store.generate_signature("CRITICAL", "WAN", "WAN_DROP")
        self.assertEqual(sig1, sig2)
        self.assertEqual(len(sig1), 32)

    def test_save_and_retrieve_skill(self) -> None:
        sig = self.store.generate_signature("WARNING", "WIFI", "DISCONNECT")
        self.store.save_or_update_skill(
            sig, "WIFI", "DISCONNECT",
            "restart_interface", {"interface_name": "ra0"}
        )

        skill = self.store.get_skill(sig, min_confidence=0.5)
        self.assertIsNotNone(skill)
        if skill:
            self.assertEqual(skill["tool_name"], "restart_interface")
            self.assertGreaterEqual(skill["confidence_score"], 0.5)

    def test_record_failure_decay(self) -> None:
        sig = self.store.generate_signature("WARNING", "SYSTEM", "CPU_SPIKE")
        self.store.save_or_update_skill(sig, "SYSTEM", "CPU_SPIKE", "set_qos_priority", {})

        skill = self.store.get_skill(sig, min_confidence=0.1)
        self.assertIsNotNone(skill)
        initial_conf = skill["confidence_score"] if skill else 1.0

        self.store.record_failure(sig)
        decayed_skill = self.store.get_skill(sig, min_confidence=0.1)
        if decayed_skill:
            self.assertLess(decayed_skill["confidence_score"], initial_conf)


if __name__ == "__main__":
    unittest.main()
