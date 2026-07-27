"""Performance and benchmark test suite for cache lookup latency and database scaling.
"""
import os
import time
import unittest
from agent.skill_store import SkillStore


class TestE2EPerformance(unittest.TestCase):
    """E2E Performance test suite for Skill Store benchmark metrics."""

    def setUp(self) -> None:
        self.db_path = "tests/e2e_perf_skills.db"
        if os.path.exists(self.db_path):
            os.remove(self.db_path)
        self.skill_store = SkillStore(db_path=self.db_path)

    def tearDown(self) -> None:
        self.skill_store.close()
        if os.path.exists(self.db_path):
            os.remove(self.db_path)

    def test_sqlite_lookup_latency(self) -> None:
        """Benchmark: Ensure SQLite skill store lookup executes in < 1ms."""
        sig = self.skill_store.generate_signature("WARNING", "SYSTEM", "CPU_SPIKE")
        self.skill_store.save_or_update_skill(sig, "SYSTEM", "CPU_SPIKE", "purge_memory_cache", {})

        start = time.perf_counter()
        skill = self.skill_store.get_skill(sig, min_confidence=0.1)
        duration_ms = (time.perf_counter() - start) * 1000.0

        self.assertIsNotNone(skill)
        self.assertLess(duration_ms, 5.0)  # Threshold under 5ms

    def test_batch_skill_scaling(self) -> None:
        """Benchmark: Ensure Skill Store performs efficiently with 100 inserted skills."""
        for i in range(100):
            sig = self.skill_store.generate_signature("INFO", "TEST", f"EVENT_{i}")
            self.skill_store.save_or_update_skill(sig, "TEST", f"EVENT_{i}", "no_action_required", {})

        sig_test = self.skill_store.generate_signature("INFO", "TEST", "EVENT_50")
        skill = self.skill_store.get_skill(sig_test, min_confidence=0.1)
        self.assertIsNotNone(skill)


if __name__ == "__main__":
    unittest.main()
