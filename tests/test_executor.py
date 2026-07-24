import unittest
from unittest.mock import MagicMock, patch
from agent.executor import RouterExecutor

class TestRouterExecutor(unittest.TestCase):
    """
    Unit Tests cho RouterExecutor module (Chạy 100% Local trên Laptop không cần Router thật).
    """

    def setUp(self):
        self.executor_dry = RouterExecutor(dry_run=True)

    # 1. Test Parameter Validation & Safety Checks
    def test_validate_mac_valid(self):
        mac = "AA:BB:CC:11:22:33"
        self.assertEqual(self.executor_dry.validate_mac(mac), "AA:BB:CC:11:22:33")

    def test_validate_mac_invalid_format(self):
        with self.assertRaises(ValueError):
            self.executor_dry.validate_mac("INVALID-MAC-FORMAT")

    def test_validate_mac_injection_attempt(self):
        with self.assertRaises(ValueError):
            self.executor_dry.validate_mac("AA:BB:CC:11:22:33; rm -rf /")

    def test_validate_interface_valid(self):
        self.assertEqual(self.executor_dry.validate_interface("wan"), "wan")
        self.assertEqual(self.executor_dry.validate_interface("br-lan"), "br-lan")

    def test_validate_interface_not_in_whitelist(self):
        with self.assertRaises(ValueError):
            self.executor_dry.validate_interface("unauthorized_iface")

    # 2. Test Dry-Run Mode
    def test_dry_run_mode_restart_interface(self):
        res = self.executor_dry.execute_restart_interface(interface_name="wan", reason="Test WAN restart")
        self.assertTrue(res["success"])
        self.assertTrue(res["dry_run"])
        self.assertEqual(res["action"], "restart_interface")
        self.assertEqual(len(res["audit_log"]), 3)

    def test_dry_run_mode_block_device(self):
        res = self.executor_dry.execute_block_device(target_mac="11:22:33:44:55:66", reason="Malware suspicious")
        self.assertTrue(res["success"])
        self.assertTrue(res["dry_run"])
        self.assertEqual(res["action"], "block_device")

    # 3. Test AI Decision Dispatcher
    def test_dispatch_ai_decision_function_call(self):
        decision = {
            "status": "success",
            "action_type": "FUNCTION_CALL",
            "tool_name": "restart_interface",
            "arguments": {
                "interface_name": "wan",
                "reason": "WAN link down anomaly detected"
            }
        }
        res = self.executor_dry.dispatch_ai_decision(decision)
        self.assertTrue(res["success"])
        self.assertEqual(res["action"], "restart_interface")

    def test_dispatch_ai_decision_no_action(self):
        decision = {
            "status": "success",
            "action_type": "FUNCTION_CALL",
            "tool_name": "no_action_required",
            "arguments": {
                "reason": "System healthy"
            }
        }
        res = self.executor_dry.dispatch_ai_decision(decision)
        self.assertTrue(res["success"])
        self.assertEqual(res["action"], "no_action_required")

    # 4. Test Mocked SSH Execution
    @patch("paramiko.SSHClient")
    def test_mocked_ssh_execution(self, mock_ssh_cls):
        mock_client = MagicMock()
        mock_ssh_cls.return_value = mock_client
        
        mock_stdout = MagicMock()
        mock_stdout.read.return_value = b"OK"
        mock_stderr = MagicMock()
        mock_stderr.read.return_value = b""
        
        mock_client.exec_command.return_value = (None, mock_stdout, mock_stderr)

        real_executor = RouterExecutor(hostname="192.168.8.1", dry_run=False)
        res = real_executor.execute_restart_interface("wan", reason="Test Mock SSH")

        self.assertTrue(res["success"])
        self.assertFalse(res["dry_run"])
        self.assertTrue(mock_client.connect.called)
        self.assertTrue(mock_client.exec_command.called)

if __name__ == "__main__":
    unittest.main()
