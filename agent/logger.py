import os
import sys
import logging
from logging.handlers import RotatingFileHandler

def setup_logger(name="beryl7_agent", log_file="logs/beryl7_agent.log", level=logging.INFO):
    """
    Hệ thống Logging tập trung cấp Production cho Beryl 7 Agent.
    Hỗ trợ xuất log ra Console có màu và lưu File log với cơ chế xoay vòng (Rotating Log).
    """
    os.makedirs(os.path.dirname(log_file), exist_ok=True)

    logger = logging.getLogger(name)
    logger.setLevel(level)

    # Đảm bảo không bị thêm handler lặp lại
    if logger.hasHandlers():
        return logger

    # Windows Console UTF-8 Format Fix
    if sys.platform.startswith('win'):
        try:
            sys.stdout.reconfigure(encoding='utf-8')
        except AttributeError:
            pass

    # Formatter chuẩn
    formatter = logging.Formatter(
        "[%(asctime)s] [%(levelname)s] [%(name)s]: %(message)s",
        datefmt="%Y-%m-%d %H:%M:%S"
    )

    # File Handler (Ghi log ra file, tối đa 5MB/file, giữ lại 3 file cũ)
    file_handler = RotatingFileHandler(log_file, maxBytes=5*1024*1024, backupCount=3, encoding="utf-8")
    file_handler.setFormatter(formatter)
    file_handler.setLevel(level)
    logger.addHandler(file_handler)

    # Console Handler (Xuất ra màn hình terminal)
    console_handler = logging.StreamHandler(sys.stdout)
    console_handler.setFormatter(formatter)
    console_handler.setLevel(level)
    logger.addHandler(console_handler)

    return logger

# Single instance logger mặc định
agent_logger = setup_logger()
