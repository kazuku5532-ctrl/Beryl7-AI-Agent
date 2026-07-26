import unittest
from unittest.mock import MagicMock, patch
from agent.watchdog import GuardedWatchdog

class TestGuardedWatchdog(unittest.TestCase):
    """
    Unit Tests cho GuardedWatchdog (Chạy local trên Laptop với Mocking).
    """

    def setUp(self):
        self.watchdog = GuardedWatchdog(hostname="192.168.8.1")

    def test_ping_check_success(self):
        with patch("socket.socket") as mock_sock_cls:
            mock_sock = MagicMock()
            mock_sock.connect_ex.return_value = 0
            mock_sock_cls.return_value = mock_sock
            self.assertTrue(self.watchdog.ping_check("192.168.8.1", 22))

    def test_ping_check_failure(self):
        with patch("socket.socket") as mock_sock_cls:
            mock_sock = MagicMock()
            mock_sock.connect_ex.return_value = 111
            mock_sock_cls.return_value = mock_sock
            self.assertFalse(self.watchdog.ping_check("192.168.8.1", 22))

    @patch("agent.watchdog.GuardedWatchdog._cancel_fallback_script")
    @patch("agent.watchdog.GuardedWatchdog.ping_check", return_value=True)
    @patch("paramiko.SSHClient")
    def test_execute_with_guardrail_success(self, mock_ssh_cls, mock_ping, mock_cancel):
        mock_client = MagicMock()
        mock_ssh_cls.return_value = mock_client
        mock_stdout = MagicMock()
        mock_stdout.read.return_value = b"OK"
        mock_stderr = MagicMock()
        mock_stderr.read.return_value = b""
        mock_client.exec_command.return_value = (None, mock_stdout, mock_stderr)

        mock_executor = MagicMock(return_value={"success": True, "action": "test_action"})
        
        res = self.watchdog.execute_with_guardrail(
            executor_func=mock_executor,
            countdown_seconds=10
        )

        self.assertTrue(res["success"])
        self.assertFalse(res.get("rolled_back", True))
        self.assertTrue(mock_executor.called)

    @patch("agent.watchdog.GuardedWatchdog._force_rollback_now")
    @patch("agent.watchdog.GuardedWatchdog.ping_check", return_value=False)
    @patch("paramiko.SSHClient")
    def test_execute_with_guardrail_triggers_rollback(self, mock_ssh_cls, mock_ping, mock_rollback):
        mock_client = MagicMock()
        mock_ssh_cls.return_value = mock_client
        mock_stdout = MagicMock()
        mock_stdout.read.return_value = b"OK"
        mock_stderr = MagicMock()
        mock_stderr.read.return_value = b""
        mock_client.exec_command.return_value = (None, mock_stdout, mock_stderr)

        mock_executor = MagicMock(return_value={"success": True, "action": "bad_action"})
        
        res = self.watchdog.execute_with_guardrail(
            executor_func=mock_executor,
            countdown_seconds=10
        )

        self.assertFalse(res["success"])
        self.assertTrue(res["rolled_back"])
        self.assertIn("Tự động Rollback", res["error"])
        self.assertTrue(mock_rollback.called)

if __name__ == "__main__":
    unittest.main()
