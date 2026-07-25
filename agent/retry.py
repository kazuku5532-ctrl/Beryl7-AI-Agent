import time
import functools
from agent.logger import agent_logger

def retry_on_exception(max_retries=3, delay_seconds=2, backoff_factor=2, exceptions=(Exception,)):
    """
    Decorator tự động thử lại (Retry with Exponential Backoff) cho các tác vụ I/O
    như kết nối SSH hoặc gọi Cloud API khi gặp sự cố mạng chập chờn.
    """
    def decorator(func):
        @functools.wraps(func)
        def wrapper(*args, **kwargs):
            current_delay = delay_seconds
            for attempt in range(1, max_retries + 1):
                try:
                    return func(*args, **kwargs)
                except exceptions as e:
                    if attempt == max_retries:
                        agent_logger.error(f"❌ Thất bại hoàn toàn sau {max_retries} lần thử lại hàm '{func.__name__}': {e}")
                        raise e
                    agent_logger.warning(
                        f"⚠️ Thử lại hàm '{func.__name__}' Lần {attempt}/{max_retries} thất bại ({e}). "
                        f"Thử lại sau {current_delay}s..."
                    )
                    time.sleep(current_delay)
                    current_delay *= backoff_factor
        return wrapper
    return decorator
