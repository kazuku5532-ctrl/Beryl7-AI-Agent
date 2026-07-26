import os
import re
import sys
import logging
from logging.handlers import RotatingFileHandler

class SensitiveRedactFilter(logging.Filter):
    """
    Bộ lọc Redaction nhạy cảm cho Python Logger: Ẩn Bearer, JWT, Key, Token, IP, Passwords
    """
    PASS_REGEX = re.compile(r"(?i)(password|passwd|key|psk|secret|token|auth)\s*=\s*['\"]?[^'\"]+['\"]?")
    BEARER_REGEX = re.compile(r"(?i)Bearer\s+[A-Za-z0-9\-\._~\+\/]+=*")
    JWT_REGEX = re.compile(r"eyJ[A-Za-z0-9\-_%]+\.eyJ[A-Za-z0-9\-_%]+\.[A-Za-z0-9\-_%]+")

    def filter(self, record):
        if isinstance(record.msg, str):
            record.msg = self.PASS_REGEX.sub(r"\1=***REDACTED***", record.msg)
            record.msg = self.BEARER_REGEX.sub("Bearer ***REDACTED***", record.msg)
            record.msg = self.JWT_REGEX.sub("***JWT_REDACTED***", record.msg)
        return True

def setup_logger(name="beryl7_agent", log_file="logs/beryl7_agent.log", level=logging.INFO):
    """
    Hệ thống Logging tập trung cấp Production cho Beryl 7 Agent.
    Hỗ trợ xuất log ra Console có màu và lưu File log với cơ chế xoay vòng và Redaction nhạy cảm.
    """
    os.makedirs(os.path.dirname(log_file), exist_ok=True)

    logger = logging.getLogger(name)
    logger.setLevel(level)

    if logger.hasHandlers():
        return logger

    if sys.platform.startswith('win'):
        try:
            sys.stdout.reconfigure(encoding='utf-8')
        except AttributeError:
            pass

    formatter = logging.Formatter(
        "[%(asctime)s] [%(levelname)s] [%(name)s]: %(message)s",
        datefmt="%Y-%m-%d %H:%M:%S"
    )

    redact_filter = SensitiveRedactFilter()

    file_handler = RotatingFileHandler(log_file, maxBytes=5*1024*1024, backupCount=3, encoding="utf-8")
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

agent_logger = setup_logger()
