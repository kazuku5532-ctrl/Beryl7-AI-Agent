"""Prometheus metrics factory and thread-safe metric counters, gauges, and histograms.
"""
import time
import threading
from typing import Dict, Any, List, Optional


class MetricsFactory:
    """Thread-safe factory for defining and exporting Prometheus metrics in text format."""

    def __init__(self) -> None:
        self._lock = threading.Lock()

        # Counters
        self.anomalies_detected: Dict[str, int] = {}  # key: "category:severity"
        self.cache_hits_total: int = 0
        self.cache_misses_total: int = 0
        self.actions_executed: Dict[str, int] = {}  # key: "tool_name:success"

        # Gauges
        self.skills_in_store: int = 0
        self.system_cpu_load_1m: float = 0.0
        self.system_ram_usage_percent: float = 0.0
        self.router_reachable: int = 1
        self.cache_learning_rate: float = 0.0

        # Histograms (recorded durations)
        self.execution_durations: List[float] = []
        self.api_latencies: List[float] = []
        self.cache_hit_response_times_ms: List[float] = []

    def increment_anomaly(self, category: str, severity: str) -> None:
        key = f"{category}:{severity}"
        with self._lock:
            self.anomalies_detected[key] = self.anomalies_detected.get(key, 0) + 1

    def increment_cache_hit(self) -> None:
        with self._lock:
            self.cache_hits_total += 1

    def increment_cache_miss(self) -> None:
        with self._lock:
            self.cache_misses_total += 1

    def increment_action_executed(self, tool_name: str, success: bool) -> None:
        key = f"{tool_name}:{str(success).lower()}"
        with self._lock:
            self.actions_executed[key] = self.actions_executed.get(key, 0) + 1

    def update_system_gauges(self, cpu_load: float, ram_percent: float, skills_count: int, router_reachable: bool = True) -> None:
        with self._lock:
            self.system_cpu_load_1m = cpu_load
            self.system_ram_usage_percent = ram_percent
            self.skills_in_store = skills_count
            self.router_reachable = 1 if router_reachable else 0

    def record_execution_duration(self, duration_sec: float) -> None:
        with self._lock:
            self.execution_durations.append(duration_sec)
            if len(self.execution_durations) > 1000:
                self.execution_durations.pop(0)

    def record_api_latency(self, latency_sec: float) -> None:
        with self._lock:
            self.api_latencies.append(latency_sec)
            if len(self.api_latencies) > 1000:
                self.api_latencies.pop(0)

    def record_cache_hit_response_ms(self, response_ms: float) -> None:
        with self._lock:
            self.cache_hit_response_times_ms.append(response_ms)
            if len(self.cache_hit_response_times_ms) > 1000:
                self.cache_hit_response_times_ms.pop(0)

    def export_prometheus_text(self) -> str:
        with self._lock:
            lines = [
                "# HELP beryl7_anomalies_detected_total Total anomalies detected by category and severity",
                "# TYPE beryl7_anomalies_detected_total counter",
            ]
            for key, val in self.anomalies_detected.items():
                parts = key.split(":")
                cat = parts[0] if len(parts) > 0 else "UNKNOWN"
                sev = parts[1] if len(parts) > 1 else "INFO"
                lines.append(f'beryl7_anomalies_detected_total{{category="{cat}",severity="{sev}"}} {val}')

            lines.extend([
                "# HELP beryl7_cache_hits_total Total SQLite cache hits",
                "# TYPE beryl7_cache_hits_total counter",
                f"beryl7_cache_hits_total {self.cache_hits_total}",
                "# HELP beryl7_cache_misses_total Total SQLite cache misses requiring Cloud AI",
                "# TYPE beryl7_cache_misses_total counter",
                f"beryl7_cache_misses_total {self.cache_misses_total}",
                "# HELP beryl7_skills_in_store Current number of learned skills in SQLite store",
                "# TYPE beryl7_skills_in_store gauge",
                f"beryl7_skills_in_store {self.skills_in_store}",
                "# HELP beryl7_system_cpu_load_1m Current 1-minute system CPU load",
                "# TYPE beryl7_system_cpu_load_1m gauge",
                f"beryl7_system_cpu_load_1m {self.system_cpu_load_1m:.2f}",
                "# HELP beryl7_system_ram_usage_percent Current system RAM usage percentage",
                "# TYPE beryl7_system_ram_usage_percent gauge",
                f"beryl7_system_ram_usage_percent {self.system_ram_usage_percent:.2f}",
                "# HELP beryl7_router_reachable SSH connectivity state to router (1 reachable, 0 down)",
                "# TYPE beryl7_router_reachable gauge",
                f"beryl7_router_reachable {self.router_reachable}",
            ])

            return "\n".join(lines) + "\n"

    def get_summary_dict(self) -> Dict[str, Any]:
        with self._lock:
            total_lookups = self.cache_hits_total + self.cache_misses_total
            hit_rate = (self.cache_hits_total / total_lookups * 100.0) if total_lookups > 0 else 100.0
            return {
                "total_cache_hits": self.cache_hits_total,
                "total_cache_misses": self.cache_misses_total,
                "cache_hit_rate_pct": round(hit_rate, 2),
                "skills_in_store": self.skills_in_store,
                "router_reachable": bool(self.router_reachable),
                "cpu_load_1m": self.system_cpu_load_1m,
                "ram_usage_pct": self.system_ram_usage_percent,
            }


# Singleton instance
metrics_factory: MetricsFactory = MetricsFactory()
