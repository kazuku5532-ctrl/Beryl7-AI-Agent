import unittest
from unittest.mock import MagicMock, patch
from agent.orchestrator import SelfEvolvingAgentOrchestrator

class TestSelfEvolvingAgentOrchestrator(unittest.TestCase):
    """
    Unit test suite cho SelfEvolvingAgentOrchestrator (Bộ não trung tâm).
    Kiểm thử Cache Hit/Miss, Xử lý sự cố rỗng (Defensive Check) và AI API Integration.
    """

    @patch("agent.orchestrator.RouterTelemetry")
    @patch("agent.orchestrator.RouterLogParser")
    @patch("agent.orchestrator.RouterExecutor")
    @patch("agent.orchestrator.GuardedWatchdog")
    @patch("agent.orchestrator.SkillStore")
    def setUp(self, MockSkillStore, MockWatchdog, MockExecutor, MockLogParser, MockTelemetry):
        self.orchestrator = SelfEvolvingAgentOrchestrator(
            hostname="192.168.8.1",
            username="root",
            password="placeholder_test_password",  # nosec B106
            db_path=":memory:",
            dry_run=True
        )

    def test_healthy_system_no_anomalies(self):
        """Test khi hệ thống mạng hoàn toàn bình thường (No anomalies)"""
        self.orchestrator.telemetry.get_normalized_telemetry.return_value = {"status": "success"}
        self.orchestrator.log_parser.detect_anomalies.return_value = {
            "has_anomalies": False,
            "anomalies": []
        }

        res = self.orchestrator.run_self_healing_cycle(api_key="fake_key")
        self.assertEqual(res["status"], "healthy")
        self.assertEqual(res["api_cost_usd"], 0.0)

    def test_defensive_empty_anomalies_list(self):
        """Test trường hợp edge case has_anomalies=True nhưng anomalies list rỗng (Chống crash)"""
        self.orchestrator.telemetry.get_normalized_telemetry.return_value = {"status": "success"}
        self.orchestrator.log_parser.detect_anomalies.return_value = {
            "has_anomalies": True,
            "anomalies": [] # Empty list
        }

        res = self.orchestrator.run_self_healing_cycle(api_key="fake_key")
        self.assertEqual(res["status"], "healthy")

    def test_cache_hit_local_sqlite_execution(self):
        """Test khi phát hiện sự cố đã có sẵn trong SQLite Skill Store (Cache Hit)"""
        self.orchestrator.telemetry.get_normalized_telemetry.return_value = {"status": "success"}
        self.orchestrator.log_parser.detect_anomalies.return_value = {
            "has_anomalies": True,
            "max_severity": "CRITICAL",
            "anomalies": [{
                "severity": "CRITICAL",
                "category": "WAN",
                "event_name": "WAN_INTERFACE_DROPPED"
            }]
        }

        self.orchestrator.skill_store.generate_signature.return_value = "fake_sig_123"
        self.orchestrator.skill_store.get_skill.return_value = {
            "id": 1,
            "tool_name": "restart_interface",
            "arguments": {"interface_name": "wan"},
            "confidence_score": 1.0,
            "success_count": 5
        }

        self.orchestrator.watchdog.execute_with_guardrail.return_value = {
            "success": True,
            "action": "restart_interface"
        }

        res = self.orchestrator.run_self_healing_cycle(api_key="fake_key")

        self.assertEqual(res["status"], "success")
        self.assertEqual(res["source"], "SQLITE_LOCAL_SKILL_STORE")
        self.assertEqual(res["api_cost_usd"], 0.0)
        self.assertEqual(res["tool_used"], "restart_interface")

if __name__ == "__main__":
    unittest.main()
