import re
import copy

class DataSanitizer:
    """
    Module Data Sanitizer: Lọc bỏ thông tin nhạy cảm (Mật khẩu, Key, Token)
    và bảo vệ quyền riêng tư trước khi dữ liệu được gửi lên Cloud AI API.
    """
    SENSITIVE_PATTERNS = [
        (re.compile(r"(password|passwd|key|psk|secret|token)\s*=\s*['\"]?[^'\"]+['\"]?", re.IGNORECASE), r"\1=***REDACTED***"),
        (re.compile(r"wpa_passphrase\s+['\"]?[^'\"]+['\"]?", re.IGNORECASE), r"wpa_passphrase ***REDACTED***")
    ]

    @classmethod
    def sanitize_string(cls, text):
        """Lọc các chuỗi văn bản thô"""
        if not isinstance(text, str):
            return text
        sanitized = text
        for pattern, replacement in cls.SENSITIVE_PATTERNS:
            sanitized = pattern.sub(replacement, sanitized)
        return sanitized

    @classmethod
    def sanitize_telemetry_dict(cls, data):
        """
        Lọc và chuẩn hóa dữ liệu dict Telemetry/Anomaly trước khi gửi tới AI.
        """
        if not isinstance(data, dict):
            return data
            
        clean_data = copy.deepcopy(data)

        # Trích xuất và xóa các thông tin nhạy cảm nếu có trong dict
        if "system" in clean_data:
            clean_data["system"].pop("password", None)
            clean_data["system"].pop("root_password", None)

        return clean_data
