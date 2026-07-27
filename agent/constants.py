"""Centralized configuration for all threshold values, timeouts, and default constants.
"""

# System Metrics Thresholds
CRITICAL_CPU_LOAD: float = 1.5
CRITICAL_RAM_USAGE_PERCENT: float = 85.0
CRITICAL_HARDWARE_TEMP_C: float = 80.0

# Wi-Fi Thresholds
CRITICAL_WIFI_DISCONNECT_COUNT: int = 3
ALLOWED_WIFI_CHANNELS: range = range(1, 166)  # 1-165
DEFAULT_24G_CHANNEL: int = 6
DEFAULT_5G_CHANNEL: int = 36

# Watchdog Guardrails
WATCHDOG_COUNTDOWN_SECONDS: int = 30
WATCHDOG_HEALTH_CHECK_INTERVAL: int = 2  # seconds
WATCHDOG_INITIAL_MONITOR_DURATION: int = 15  # seconds
SAFE_MODE_SUCCESS_THRESHOLD: int = 3

# Skill Store & EMA Parameters
SKILL_CONFIDENCE_THRESHOLD: float = 0.5
SKILL_LOCAL_EXECUTION_THRESHOLD: float = 0.85
SKILL_SUCCESS_ALPHA: float = 0.2  # EMA growth rate
SKILL_FAILURE_DECAY: float = 0.5  # 50% penalty on failure

# SSH Connectivity
SSH_CONNECT_TIMEOUT: int = 5
SSH_COMMAND_TIMEOUT: int = 10
SSH_KEEPALIVE_INTERVAL: int = 5

# Cloud AI (Gemini) API
GEMINI_API_TIMEOUT: int = 30
GEMINI_RETRY_MAX: int = 3
GEMINI_RETRY_DELAY: int = 2
GEMINI_FALLBACK_MODELS: list[str] = [
    "gemini-2.5-flash",
    "gemini-2.0-flash-lite",
    "gemini-2.0-flash",
]

# Health Check Server
HEALTH_SERVER_PORT: int = 8888
HTTP_RATE_LIMIT_PER_MIN: int = 30
PENDING_APPROVAL_EXPIRY_MINUTES: int = 10
