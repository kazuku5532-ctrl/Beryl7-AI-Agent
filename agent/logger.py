"""Production-grade structured logging module with JSON formatting and redaction.
"""
import os
import re
import sys
import json
import logging
from datetime import datetime
from logging.handlers import RotatingFileHandler
from typing import Any, Dict


class SensitiveRedactFilter(logging.Filter):
    """Redaction filter for sensitive tokens, passwords, and API keys."""

    PASS_REGEX = re.compile(r"(?i)(password|passwd|key|psk|secret|token|auth)\s*=\s*['\"]?[^'\"]+['\"]?")
    BEARER_REGEX = re.compile(r"(?i)Bearer\s+[A-Za-z0-9\-\._~\+\/]+=*")
    JWT_REGEX = re.compile(r"eyJ[A-Za-z0-9\-_%]+\.eyJ[A-Za-z0-9\-_%]+\.[A-Za-z0-9\-_%]+")

    def filter(self, record: logging.LogRecord) -> bool:
        if isinstance(record.msg, str):
            record.msg = self.PASS_REGEX.sub(r"\1=***REDACTED***", record.msg)
            record.msg = self.BEARER_REGEX.sub("Bearer ***REDACTED***", record.msg)
            record.msg = self.JWT_REGEX.sub("***JWT_REDACTED***", record.msg)
        return True


class JSONFormatter(logging.Formatter):
    """Structured JSON Formatter for log management pipelines."""

    def format(self, record: logging.LogRecord) -> str:
        log_data: Dict[str, Any] = {
            "timestamp": datetime.utcnow().isoformat() + "Z",
            "level": record.levelname,
            "logger": record.name,
            "message": record.getMessage(),
            "module": record.module,
            "function": record.funcName,
            "line": record.lineno,
        }
        if record.exc_info:
            log_data["exception"] = self.formatException(record.exc_info)
        return json.dumps(log_data, ensure_ascii=False)


def setup_logger(name: str = "beryl7_agent", log_file: str = "logs/beryl7_agent.log", level: int = logging.INFO) -> logging.Logger:
    """Initialize structured logging system with file rotation and redaction filters."""
    os.makedirs(os.path.dirname(log_file), exist_ok=True)

    logger = logging.getLogger(name)
    logger.setLevel(level)

    if logger.hasHandlers():
        return logger

    if sys.platform.startswith("win"):
        try:
            if hasattr(sys.stdout, "reconfigure"):
                sys.stdout.reconfigure(encoding="utf-8")  # type: ignore
        except AttributeError:
            pass

    formatter = logging.Formatter(
        "[%(asctime)s] [%(levelname)s] [%(name)s]: %(message)s",
        datefmt="%Y-%m-%d %H:%M:%S"
    )

    redact_filter = SensitiveRedactFilter()

    file_handler = RotatingFileHandler(log_file, maxBytes=5 * 1024 * 1024, backupCount=3, encoding="utf-8")
    file_handler.setFormatter(formatter)
    file_handler.setLevel(level)
    file_handler.addFilter(redact_filter)
    logger.addHandler(file_handler)

    console_handler = logging.StreamHandler(sys.stdout)
    console_handler.setFormatter(formatter)
    console_handler.setLevel(level)
    console_handler.addFilter(redact_filter)
    logger.addHandler(console_handler)

    return logger


agent_logger: logging.Logger = setup_logger()
