import os
import json
import time
import socket
import paramiko
from agent.logger import agent_logger

class RouterTelemetry:
    """
    Module thu thập dữ liệu mạng và trạng thái hệ thống từ Router Beryl 7 qua SSH.
    """
    def __init__(self, hostname=None, port=None, username=None, password=None, key_filename=None, timeout=5):
        self.hostname = hostname or os.environ.get("ROUTER_HOST", "192.168.8.1")
        self.port = int(port or os.environ.get("ROUTER_PORT", 22))
        self.username = username or os.environ.get("ROUTER_USER", "root")
        self.password = password or os.environ.get("ROUTER_PASSWORD")
        self.key_filename = key_filename
        self.timeout = timeout
        self.last_sample_time = None
        self.last_device_stats = {}

    def _exec_ssh_cmd(self, client, command):
        """Hàm bổ trợ thực thi câu lệnh SSH và trả về output/error"""
        stdin, stdout, stderr = client.exec_command(command)
        out = stdout.read().decode('utf-8', errors='ignore').strip()
        err = stderr.read().decode('utf-8', errors='ignore').strip()
        return out, err

    def collect_raw_telemetry(self):
        """
        Gửi các câu lệnh ubus & system lệnh xuống Beryl 7 để lấy dữ liệu thô.
        """
        client = paramiko.SSHClient()
        client.load_system_host_keys()
        client.set_missing_host_key_policy(paramiko.RejectPolicy())
        
        raw_data = {
            "system_info": {},
            "dhcp_leases": [],
            "wifi_clients_2g": {},
            "wifi_clients_5g": {},
            "network_devices": {},
            "wan_status": {}
        }
        
        try:
            client.connect(
                hostname=self.hostname,
                port=self.port,
                username=self.username,
                password=self.password,
                key_filename=self.key_filename,
                timeout=self.timeout
            )
            
            # 1. System Info (CPU Load, RAM, Uptime)
            out, _ = self._exec_ssh_cmd(client, "ubus call system info")
            if out:
                try:
                    raw_data["system_info"] = json.loads(out)
                except (json.JSONDecodeError, TypeError, ValueError):
                    pass

            # 2. DHCP Leases (MAC -> IP -> Hostname)
            out, _ = self._exec_ssh_cmd(client, "cat /tmp/dhcp.leases")
            if out:
                leases = []
                for line in out.splitlines():
                    parts = line.split()
                    if len(parts) >= 4:
                        leases.append({
                            "timestamp": parts[0],
                            "mac": parts[1].upper(),
                            "ip": parts[2],
                            "hostname": parts[3] if parts[3] != "*" else "Unknown"
                        })
                raw_data["dhcp_leases"] = leases

            # 3. Wi-Fi Clients
            out, _ = self._exec_ssh_cmd(client, "ubus call hostapd.wlan0 get_clients")
            if out:
                try:
                    raw_data["wifi_clients_2g"] = json.loads(out).get("clients", {})
                except (json.JSONDecodeError, AttributeError, TypeError, ValueError):
                    pass
                    
            out, _ = self._exec_ssh_cmd(client, "ubus call hostapd.wlan1 get_clients")
            if out:
                try:
                    raw_data["wifi_clients_5g"] = json.loads(out).get("clients", {})
                except (json.JSONDecodeError, AttributeError, TypeError, ValueError):
                    pass

            # 4. Network Devices (RX/TX bytes, packets, errors, drops)
            out, _ = self._exec_ssh_cmd(client, "ubus call network.device status")
            if out:
                try:
                    raw_data["network_devices"] = json.loads(out)
                except (json.JSONDecodeError, TypeError, ValueError):
                    pass

            # 5. WAN Interface Status (Public IP, Gateway, DNS)
            out, _ = self._exec_ssh_cmd(client, "ubus call network.interface.wan status")
            if out:
                try:
                    raw_data["wan_status"] = json.loads(out)
                except (json.JSONDecodeError, TypeError, ValueError):
                    pass

        except (paramiko.SSHException, socket.error, socket.timeout, OSError) as e:
            agent_logger.error(f"Lỗi thu thập Telemetry từ Router: {e}")
            raw_data["error"] = str(e)
        finally:
            client.close()
            
        return raw_data

    def get_normalized_telemetry(self):
        """
        Chuẩn hóa dữ liệu thô thành Telemetry Schema hoàn chỉnh cho AI đọc.
        """
        now = time.time()
        raw = self.collect_raw_telemetry()
        
        if "error" in raw:
            return {"status": "error", "message": raw["error"]}

        # Parse System Info
        sys_info = raw.get("system_info", {})
        memory = sys_info.get("memory", {})
        total_ram = memory.get("total", 1)
        free_ram = memory.get("free", 0) + memory.get("buffered", 0) + memory.get("cached", 0)
        ram_usage_pct = round(((total_ram - free_ram) / total_ram) * 100, 2)
        
        load = sys_info.get("load", [0, 0, 0])
        cpu_load_1m = round(load[0] / 65535.0, 2) if len(load) > 0 else 0.0

        # Parse DHCP & Clients
        dhcp_map = {item["mac"]: item for item in raw.get("dhcp_leases", [])}
        
        clients_list = []
        all_wifi_clients = {}
        all_wifi_clients.update(raw.get("wifi_clients_2g", {}))
        all_wifi_clients.update(raw.get("wifi_clients_5g", {}))
        
        for mac, info in all_wifi_clients.items():
            mac_upper = mac.upper()
            dhcp_info = dhcp_map.get(mac_upper, {})
            clients_list.append({
                "mac": mac_upper,
                "ip": dhcp_info.get("ip", "Unknown"),
                "hostname": dhcp_info.get("hostname", "Unknown"),
                "signal_rssi": info.get("signal", 0),
                "rx_rate_mbps": round(info.get("rx", {}).get("rate", 0) / 1000.0, 1),
                "tx_rate_mbps": round(info.get("tx", {}).get("rate", 0) / 1000.0, 1),
                "connected_sec": info.get("connected_time", 0)
            })

        # Calculate Realtime Interface Bandwidth
        devices_status = raw.get("network_devices", {})
        interfaces_summary = {}
        dt = (now - self.last_sample_time) if self.last_sample_time else 0
        
        for dev_name, dev_info in devices_status.items():
            rx_bytes = dev_info.get("stats", {}).get("rx_bytes", 0)
            tx_bytes = dev_info.get("stats", {}).get("tx_bytes", 0)
            
            rx_speed_kbs = 0.0
            tx_speed_kbs = 0.0
            
            if dt > 0 and dev_name in self.last_device_stats:
                prev_rx = self.last_device_stats[dev_name].get("rx_bytes", rx_bytes)
                prev_tx = self.last_device_stats[dev_name].get("tx_bytes", tx_bytes)
                rx_speed_kbs = round(max(0, rx_bytes - prev_rx) / (dt * 1024.0), 2)
                tx_speed_kbs = round(max(0, tx_bytes - prev_tx) / (dt * 1024.0), 2)
                
            self.last_device_stats[dev_name] = {"rx_bytes": rx_bytes, "tx_bytes": tx_bytes}
            
            if dev_info.get("up", False):
                interfaces_summary[dev_name] = {
                    "is_up": True,
                    "rx_speed_kbs": rx_speed_kbs,
                    "tx_speed_kbs": tx_speed_kbs,
                    "rx_errors": dev_info.get("stats", {}).get("rx_errors", 0),
                    "tx_errors": dev_info.get("stats", {}).get("tx_errors", 0)
                }

        self.last_sample_time = now

        # Parse WAN Status
        wan_info = raw.get("wan_status", {})
        wan_up = wan_info.get("up", False)
        wan_ip = "Disconnected"
        if wan_up and wan_info.get("ipv4-address"):
            wan_ip = wan_info["ipv4-address"][0].get("address", "Unknown")

        normalized_schema = {
            "status": "success",
            "timestamp": int(now),
            "system": {
                "uptime_seconds": sys_info.get("uptime", 0),
                "cpu_load_1m": cpu_load_1m,
                "ram_usage_percent": ram_usage_pct
            },
            "wan": {
                "is_connected": wan_up,
                "ip_address": wan_ip
            },
            "connected_clients_count": len(clients_list),
            "clients": clients_list,
            "active_interfaces": interfaces_summary
        }
        
        return normalized_schema
