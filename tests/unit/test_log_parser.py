"""Unit tests for LogParser module parsing syslog lines and sanitizing inputs.
"""
import unittest
from agent.log_parser import RouterLogParser


class TestLogParser(unittest.TestCase):
    """Test suite verifying RouterLogParser anomaly regex pattern matching."""

    def setUp(self) -> None:
        self.parser = RouterLogParser()

    def test_wan_drop_parsing(self) -> None:
        sample_log = ["kernel: eth1 link down (WAN disconnected)"]
        events = self.parser.parse_log_events(sample_log)
        self.assertGreater(len(events["wan_events"]), 0)

    def test_wifi_failure_parsing(self) -> None:
        sample_log = ["hostapd: wlan0: STA 00:11:22:33:44:55 IEEE 802.11: deauthenticated due to beacon loss"]
        events = self.parser.parse_log_events(sample_log)
        self.assertGreater(len(events["wifi_disconnects"]), 0)

    def test_oom_parsing(self) -> None:
        sample_log = ["kernel: Out of memory: Kill process 123 (bad_proc) score 900"]
        events = self.parser.parse_log_events(sample_log)
        self.assertGreater(len(events["kernel_errors"]), 0)


if __name__ == "__main__":
    unittest.main()
