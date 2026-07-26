import re
import json
import paramiko

class RouterLogParser:
    """
    Module thu thập nhật ký logread, phân tích sự cố (Event Parsing) 
    và phát hiện bất thường (Anomaly Detection) trên Router Beryl 7.
    """
    # Các mẫu Regex nhận diện sự cố phổ biến trên OpenWrt
    WIFI_DISCONNECT_PATTERNS = [
        re.compile(r"deauthenticated", re.IGNORECASE),
        re.compile(r"disassociated", re.IGNORECASE),
        re.compile(r"AP-STA-DISCONNECTED", re.IGNORECASE),
        re.compile(r"leave", re.IGNORECASE)
    ]
    
    WAN_DOWN_PATTERNS = [
        re.compile(r"link down", re.IGNORECASE),
        re.compile(r"carrier lost", re.IGNORECASE),
        re.compile(r"interface.*wan.*down", re.IGNORECASE),
        re.compile(r"action.*down", re.IGNORECASE)
    ]

    KERNEL_ERROR_PATTERNS = [
        re.compile(r"out of memory", re.IGNORECASE),
        re.compile(r"kernel panic", re.IGNORECASE),
        re.compile(r"OOM-killer", re.IGNORECASE),
        re.compile(r"hardware error", re.IGNORECASE)
    ]

    DNS_DHCP_PATTERNS = [
        re.compile(r"dnsmasq.*failed", re.IGNORECASE),
        re.compile(r"no leases left", re.IGNORECASE),
        re.compile(r"DHCP NAK", re.IGNORECASE)
    ]

    def __init__(self, hostname="192.168.8.1", port=22, username="root", password=None, key_filename=None, timeout=5):
        self.hostname = hostname
        self.port = port
        self.username = username
        self.password = password
        self.key_filename = key_filename
        self.timeout = timeout

    def fetch_recent_logs(self, line_count=100):
        """
        Lấy 100 dòng log gần nhất từ logread qua SSH.
        """
        client = paramiko.SSHClient()
        client.load_system_host_keys()
        client.set_missing_host_key_policy(paramiko.RejectPolicy())
        
        logs = []
        try:
            client.connect(
                hostname=self.hostname,
                port=self.port,
                username=self.username,
                password=self.password,
                key_filename=self.key_filename,
                timeout=self.timeout
            )
            
            cmd = f"logread -l {line_count}"
            stdin, stdout, stderr = client.exec_command(cmd)
            out = stdout.read().decode('utf-8', errors='ignore').strip()
            
            if out:
                logs = out.splitlines()
        except Exception as e:
            logs = [f"ERROR: Cannot fetch logs - {str(e)}"]
        finally:
            client.close()
            
        return logs

    def parse_log_events(self, logs):
        """
        Phân loại các sự kiện log thành các danh mục sự cố.
        """
        events = {
            "wifi_disconnects": [],
            "wan_events": [],
            "kernel_errors": [],
            "dns_dhcp_errors": [],
            "general_warnings": []
        }

        for line in logs:
            line_str = line.strip()
            if not line_str:
                continue

            # Kiểm tra Wi-Fi Disconnect
            if any(pattern.search(line_str) for pattern in self.WIFI_DISCONNECT_PATTERNS):
                events["wifi_disconnects"].append(line_str)
                continue

            # Kiểm tra WAN / Interface Down
            if any(pattern.search(line_str) for pattern in self.WAN_DOWN_PATTERNS):
                events["wan_events"].append(line_str)
                continue

            # Kiểm tra Kernel Error / OOM
            if any(pattern.search(line_str) for pattern in self.KERNEL_ERROR_PATTERNS):
                events["kernel_errors"].append(line_str)
                continue

            # Kiểm tra DNS / DHCP Error
            if any(pattern.search(line_str) for pattern in self.DNS_DHCP_PATTERNS):
                events["dns_dhcp_errors"].append(line_str)
                continue

            # Các cảnh báo khác chứa từ khóa warn hoặc error
            if "warn" in line_str.lower() or "error" in line_str.lower() or "fail" in line_str.lower():
                events["general_warnings"].append(line_str)

        return events

    def detect_anomalies(self, telemetry_data=None):
        """
        Phát hiện bất thường (Anomaly Detection Engine) kết hợp Log & Telemetry.
        Trả về JSON Anomaly Schema với thứ tự ưu tiên (Severity Level).
        """
        logs = self.fetch_recent_logs(line_count=100)
        parsed_events = self.parse_log_events(logs)
        
        anomalies = []

        # 1. Bất thường Wi-Fi disconnect lặp lại nhiều lần
        wifi_disc_count = len(parsed_events["wifi_disconnects"])
        if wifi_disc_count > 3:
            anomalies.append({
                "severity": "WARNING",
                "category": "WIFI",
                "event_name": "REPEATED_WIFI_DISCONNECTS",
                "message": f"Phát hiện {wifi_disc_count} lượt ngắt kết nối Wi-Fi gần đây.",
                "sample_log": parsed_events["wifi_disconnects"][-1]
            })

        # 2. Bất thường WAN / Mất Internet
        if len(parsed_events["wan_events"]) > 0:
            anomalies.append({
                "severity": "CRITICAL",
                "category": "WAN",
                "event_name": "WAN_INTERFACE_DROPPED",
                "message": "Phát hiện sự cố rớt mạng WAN trong nhật ký hệ thống.",
                "sample_log": parsed_events["wan_events"][-1]
            })

        # 3. Bất thường Kernel / Tràn bộ nhớ OOM
        if len(parsed_events["kernel_errors"]) > 0:
            anomalies.append({
                "severity": "CRITICAL",
                "category": "KERNEL",
                "event_name": "KERNEL_MEMORY_OR_HARDWARE_ERROR",
                "message": "Phát hiện lỗi Kernel/OOM nghiêm trọng.",
                "sample_log": parsed_events["kernel_errors"][-1]
            })

        # 4. Tích hợp Metric Anomaly từ Telemetry (nếu được truyền vào)
        if telemetry_data and telemetry_data.get("status") == "success":
            sys_metrics = telemetry_data.get("system", {})
            wan_metrics = telemetry_data.get("wan", {})

            # CPU Spike Check
            cpu_load = sys_metrics.get("cpu_load_1m", 0.0)
            if cpu_load > 1.5:
                anomalies.append({
                    "severity": "WARNING",
                    "category": "SYSTEM",
                    "event_name": "HIGH_CPU_SPIKE",
                    "message": f"Mức tải CPU cao bất thường: {cpu_load} (ngưỡng an toàn < 1.5).",
                    "sample_log": None
                })

            # RAM Overload Check
            ram_pct = sys_metrics.get("ram_usage_percent", 0.0)
            if ram_pct > 85.0:
                anomalies.append({
                    "severity": "WARNING",
                    "category": "SYSTEM",
                    "event_name": "HIGH_RAM_CONSUMPTION",
                    "message": f"Dung lượng RAM đang bị chiếm dụng cao: {ram_pct}% (ngưỡng cảnh báo > 85%).",
                    "sample_log": None
                })

            # WAN Status Disconnected Check
            if not wan_metrics.get("is_connected", False):
                anomalies.append({
                    "severity": "CRITICAL",
                    "category": "WAN",
                    "event_name": "WAN_DISCONNECTED_METRIC",
                    "message": "Cổng WAN hiện tại không có địa chỉ IP / mất kết nối Internet.",
                    "sample_log": None
                })

        # Xác định mức độ nghiêm trọng cao nhất
        max_severity = "NORMAL"
        severities = [a["severity"] for a in anomalies]
        if "CRITICAL" in severities:
            max_severity = "CRITICAL"
        elif "WARNING" in severities:
            max_severity = "WARNING"
        elif "INFO" in severities:
            max_severity = "INFO"

        anomaly_schema = {
            "status": "success",
            "has_anomalies": len(anomalies) > 0,
            "max_severity": max_severity,
            "total_anomalies": len(anomalies),
            "anomalies": anomalies,
            "parsed_summary": {
                "total_logs_analyzed": len(logs),
                "wifi_disconnect_events": wifi_disc_count,
                "wan_events": len(parsed_events["wan_events"]),
                "kernel_errors": len(parsed_events["kernel_errors"]),
                "general_warnings": len(parsed_events["general_warnings"])
            }
        }

        return anomaly_schema
