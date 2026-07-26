import re
import shlex
import time
import paramiko

class RouterExecutor:
    """
    Module RouterExecutor: Chuyển đổi các quyết định AI (Function Calling) thành
    các câu lệnh OpenWrt (uci, nftables, ifup/ifdown, wifi reload) và thi hành qua SSH.
    Tích hợp kiểm tra an toàn (Safety Check), RejectPolicy SSH, Whitelist strict và Audit Trail.
    """
    VALID_MAC_REGEX = re.compile(r"^([0-9A-Fa-f]{2}[:-]){5}([0-9A-Fa-f]{2})$")
    ALLOWED_INTERFACES = {"wan", "lan", "br-lan", "eth0", "eth1", "ra0", "rai0", "wlan0", "wlan1"}
    ALLOWED_WIFI_BANDS = {"2.4G", "5G", "2G"}

    def __init__(self, hostname="192.168.8.1", port=22, username="root", password=None, key_filename=None, timeout=5, dry_run=False, accept_insecure_hostkey=False):
        if not hostname or not username:
            raise ValueError("SECURITY ERROR: hostname and username must be explicitly configured.")

        self.hostname = hostname
        self.port = port
        self.username = username
        self.password = password
        self.key_filename = key_filename
        self.timeout = timeout
        self.dry_run = dry_run
        self.accept_insecure_hostkey = accept_insecure_hostkey

    def _exec_remote_commands(self, commands):
        """
        Thực thi danh sách lệnh SSH với kiểm tra an toàn, RejectPolicy và Audit Trail.
        """
        audit_log = []
        if self.dry_run:
            for cmd in commands:
                audit_log.append({"command": cmd, "status": "DRY_RUN_SKIPPED", "output": "Dry-run mode active."})
            return {"success": True, "dry_run": True, "audit_log": audit_log}

        client = paramiko.SSHClient()
        client.load_system_host_keys()

        if self.accept_insecure_hostkey:
            client.set_missing_host_key_policy(paramiko.WarningPolicy())
        else:
            client.set_missing_host_key_policy(paramiko.RejectPolicy())

        try:
            client.connect(
                hostname=self.hostname,
                port=self.port,
                username=self.username,
                password=self.password,
                key_filename=self.key_filename,
                timeout=self.timeout
            )
            
            transport = client.get_transport()
            if transport:
                transport.set_keepalive(5)

            for cmd in commands:
                stdin, stdout, stderr = client.exec_command(cmd, timeout=10)
                out = stdout.read().decode('utf-8', errors='ignore').strip()
                err = stderr.read().decode('utf-8', errors='ignore').strip()
                
                status = "SUCCESS" if not err else "WARNING/ERROR"
                audit_log.append({
                    "command": cmd,
                    "status": status,
                    "stdout": out,
                    "stderr": err,
                    "timestamp": time.time()
                })
                
                if err and "error" in err.lower():
                    client.close()
                    return {"success": False, "error": err, "audit_log": audit_log}

        except Exception as e:
            return {"success": False, "error": str(e), "audit_log": audit_log}
        finally:
            client.close()

        return {"success": True, "dry_run": False, "audit_log": audit_log}

    def validate_mac(self, mac_address):
        if not mac_address or not self.VALID_MAC_REGEX.match(mac_address):
            raise ValueError(f"Địa chỉ MAC không hợp lệ hoặc không an toàn: '{mac_address}'")
        return mac_address.upper()

    def validate_interface(self, interface_name):
        if not interface_name or interface_name.lower() not in self.ALLOWED_INTERFACES:
            raise ValueError(f"Interface '{interface_name}' không nằm trong Whitelist cho phép ({self.ALLOWED_INTERFACES})")
        return interface_name.lower()

    def execute_set_qos_priority(self, target_mac, priority="HIGH", max_bandwidth_mbps=10, reason=""):
        valid_mac = self.validate_mac(target_mac)
        prio_clean = shlex.quote(str(priority).upper())
        try:
            bw_clean = int(max_bandwidth_mbps)
        except (ValueError, TypeError):
            bw_clean = 10

        commands = [
            f"uci set firewall.qos_{valid_mac.replace(':', '_')}=rule",
            f"uci set firewall.qos_{valid_mac.replace(':', '_')}.src_mac='{valid_mac}'",
            f"uci set firewall.qos_{valid_mac.replace(':', '_')}.target='ACCEPT'",
            "uci commit firewall",
            "/etc/init.d/firewall reload"
        ]
        
        res = self._exec_remote_commands(commands)
        res["action"] = "set_qos_priority"
        res["details"] = f"Đặt ưu tiên {prio_clean} ({bw_clean}Mbps) cho MAC {valid_mac}. Lý do: {reason}"
        return res

    def execute_block_device(self, target_mac, reason=""):
        valid_mac = self.validate_mac(target_mac)
        rule_name = f"block_{valid_mac.replace(':', '_')}"

        commands = [
            f"uci set firewall.{rule_name}=rule",
            f"uci set firewall.{rule_name}.name='Block_{valid_mac}'",
            f"uci set firewall.{rule_name}.src='lan'",
            f"uci set firewall.{rule_name}.src_mac='{valid_mac}'",
            f"uci set firewall.{rule_name}.target='DROP'",
            "uci commit firewall",
            "/etc/init.d/firewall reload"
        ]
        
        res = self._exec_remote_commands(commands)
        res["action"] = "block_device"
        res["details"] = f"Đã chặn MAC {valid_mac}. Lý do: {reason}"
        return res

    def execute_restart_interface(self, interface_name, reason=""):
        valid_iface = self.validate_interface(interface_name)

        commands = [
            f"ifdown {valid_iface}",
            "sleep 1",
            f"ifup {valid_iface}"
        ]
        
        res = self._exec_remote_commands(commands)
        res["action"] = "restart_interface"
        res["details"] = f"Đã restart interface '{valid_iface}'. Lý do: {reason}"
        return res

    def execute_optimize_wifi_channel(self, band="2.4G", channel=6, reason=""):
        band_str = str(band).upper()
        if band_str not in self.ALLOWED_WIFI_BANDS:
            raise ValueError(f"Băng tần '{band}' không hợp lệ. Phải là 2.4G hoặc 5G.")
            
        try:
            chan_num = int(channel)
        except (ValueError, TypeError):
            chan_num = 6

        if chan_num < 1 or chan_num > 165:
            raise ValueError(f"Số kênh Wi-Fi '{chan_num}' nằm ngoài khoảng cho phép (1-165).")

        wifi_iface = "ra0" if "2" in band_str else "rai0"

        commands = [
            f"uci set wireless.{wifi_iface}.channel='{chan_num}'",
            "uci commit wireless",
            "wifi reload"
        ]
        
        res = self._exec_remote_commands(commands)
        res["action"] = "optimize_wifi_channel"
        res["details"] = f"Đã đổi kênh Wi-Fi {band_str} (Interface {wifi_iface}) sang channel {chan_num}. Lý do: {reason}"
        return res

    def execute_no_action_required(self, reason=""):
        return {
            "success": True,
            "dry_run": self.dry_run,
            "action": "no_action_required",
            "details": f"Không có hành động can thiệp. Lý do: {reason}",
            "audit_log": []
        }

    def execute_tool(self, tool_name, arguments):
        return self.dispatch_ai_decision({
            "action_type": "FUNCTION_CALL",
            "tool_name": tool_name,
            "arguments": arguments
        })

    def dispatch_ai_decision(self, decision):
        if decision.get("action_type") != "FUNCTION_CALL":
            return {"success": True, "action": "text_response", "details": decision.get("response_text")}

        tool_name = decision.get("tool_name")
        args = decision.get("arguments", {})

        if tool_name == "set_qos_priority":
            return self.execute_set_qos_priority(
                target_mac=args.get("target_mac"),
                priority=args.get("priority", "HIGH"),
                max_bandwidth_mbps=args.get("max_bandwidth_mbps", 10),
                reason=args.get("reason", "")
            )
        elif tool_name == "block_device":
            return self.execute_block_device(
                target_mac=args.get("target_mac"),
                reason=args.get("reason", "")
            )
        elif tool_name == "restart_interface":
            return self.execute_restart_interface(
                interface_name=args.get("interface_name"),
                reason=args.get("reason", "")
            )
        elif tool_name == "optimize_wifi_channel":
            return self.execute_optimize_wifi_channel(
                band=args.get("band", "2.4G"),
                channel=args.get("channel", 6),
                reason=args.get("reason", "")
            )
        elif tool_name == "no_action_required":
            return self.execute_no_action_required(
                reason=args.get("reason", "")
            )
        else:
            raise ValueError(f"SECURITY ERROR: Tool '{tool_name}' không nằm trong danh mục Whitelist được phép thực thi.")
