"""Unit tests for Executor module validation and dry-run safety.
"""
import unittest
from agent.executor import RouterExecutor


class TestRouterExecutor(unittest.TestCase):
    """Test suite verifying RouterExecutor parameter validation and whitelist enforcement."""

    def setUp(self) -> None:
        self.executor = RouterExecutor(dry_run=True)

    def test_mac_validation(self) -> None:
        valid_mac = "00:11:22:33:44:55"
        invalid_mac = "invalid-mac-address"

        self.assertEqual(self.executor.validate_mac(valid_mac), "00:11:22:33:44:55")
        with self.assertRaises(ValueError):
            self.executor.validate_mac(invalid_mac)

    def test_interface_validation(self) -> None:
        self.assertEqual(self.executor.validate_interface("wan"), "wan")
        with self.assertRaises(ValueError):
            self.executor.validate_interface("invalid_interface_name")

    def test_dry_run_execution(self) -> None:
        result = self.executor.execute_no_action_required(reason="test dry run")
        self.assertTrue(result["success"])
        self.assertTrue(result["dry_run"])


if __name__ == "__main__":
    unittest.main()
