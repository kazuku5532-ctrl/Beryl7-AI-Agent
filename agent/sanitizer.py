import re
import copy

class DataSanitizer:
    """
    Module Data Sanitizer: Lọc bỏ sâu thông tin nhạy cảm (Mật khẩu, Key, Token, JWT, Bearer)
    và bảo vệ quyền riêng tư trước khi dữ liệu được gửi lên Cloud AI API.
    """
    SENSITIVE_PATTERNS = [
        (re.compile(r"(password|passwd|key|psk|secret|token|auth)\s*=\s*['\"]?[^'\"]+['\"]?", re.IGNORECASE), r"\1=***REDACTED***"),
        (re.compile(r"wpa_passphrase\s+['\"]?[^'\"]+['\"]?", re.IGNORECASE), r"wpa_passphrase ***REDACTED***"),
        (re.compile(r"Bearer\s+[A-Za-z0-9\-\._~\+\/]+=*", re.IGNORECASE), r"Bearer ***REDACTED***"),
        (re.compile(r"eyJ[A-Za-z0-9\-_%]+\.eyJ[A-Za-z0-9\-_%]+\.[A-Za-z0-9\-_%]+"), r"***JWT_REDACTED***")
    ]

    SENSITIVE_KEY_NAMES = {"password", "passwd", "root_password", "key", "secret", "token", "auth_token", "api_key", "psk"}

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
        Lọc sâu (deep recursive traversal) dữ liệu dict/list Telemetry/Anomaly trước khi gửi tới AI.
        """
        if not isinstance(data, (dict, list)):
            if isinstance(data, str):
                return cls.sanitize_string(data)
            return data

        clean_data = copy.deepcopy(data)

        if isinstance(clean_data, dict):
            for k, v in list(clean_data.items()):
                if k.lower() in cls.SENSITIVE_KEY_NAMES:
                    clean_data[k] = "***REDACTED***"
                else:
                    clean_data[k] = cls.sanitize_telemetry_dict(v)
        elif isinstance(clean_data, list):
            clean_data = [cls.sanitize_telemetry_dict(item) for item in clean_data]

        return clean_data
