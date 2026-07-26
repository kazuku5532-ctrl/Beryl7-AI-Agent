import re
import copy

class DataSanitizer:
    """
    Module Data Sanitizer: Deep recursive redaction cho dicts, lists, JWTs, Base64 keys,
    passwords và tokens nhạy cảm trước khi dữ liệu được gửi lên Cloud AI API.
    """
    SECRET_KEY_PATTERN = re.compile(r"(password|passwd|key|psk|secret|token|auth|bearer)", re.IGNORECASE)
    JWT_REGEX = re.compile(r"eyJ[A-Za-z0-9\-_%]+\.eyJ[A-Za-z0-9\-_%]+\.[A-Za-z0-9\-_%]+")
    BASE64_LONG_REGEX = re.compile(r"[A-Za-z0-9+/]{40,}={0,2}")
    WPA_REGEX = re.compile(r"wpa_passphrase\s+['\"]?[^'\"]+['\"]?", re.IGNORECASE)

    @classmethod
    def deep_sanitize(cls, obj):
        """
        Khắc phục Lỗ hổng 6: Deep recursive redaction theo đúng mã mẫu đề xuất
        """
        if isinstance(obj, dict):
            out = {}
            for k, v in obj.items():
                if cls.SECRET_KEY_PATTERN.search(str(k)):
                    out[k] = "***REDACTED***"
                else:
                    out[k] = cls.deep_sanitize(v)
            return out
        if isinstance(obj, list):
            return [cls.deep_sanitize(x) for x in obj]
        if isinstance(obj, str):
            if cls.JWT_REGEX.search(obj) or cls.BASE64_LONG_REGEX.search(obj):
                return "***REDACTED***"
            if cls.WPA_REGEX.search(obj):
                return "***WPA_REDACTED***"
            if len(obj) > 1000:
                return obj[:1000] + "...TRUNCATED"
            return obj
        return obj

    @classmethod
    def sanitize_string(cls, text):
        return cls.deep_sanitize(text)

    @classmethod
    def sanitize_telemetry_dict(cls, data):
        return cls.deep_sanitize(data)
