import base64
import re
import shlex
import time
import socket
import paramiko

class RouterWatchdog:
    """
    Module RouterWatchdog: Quản lý tính năng tự động Rollback (Khôi phục) cấu hình cũ
    nếu lệnh AI làm rớt mạng hoặc gây sự cố.
    """
    def __init__(self, hostname="192.168.8.1", port=22, username="root", password=None, key_filename=None, timeout=5):
        self.hostname = hostname
        self.port = port
        self.username = username
        self.password = password
        self.key_filename = key_filename
        self.timeout = timeout

    def _exec_ssh_cmd(self, client, cmd):
        stdin, stdout, stderr = client.exec_command(cmd, timeout=10)
        out = stdout.read().decode('utf-8', errors='ignore').strip()
        err = stderr.read().decode('utf-8', errors='ignore').strip()
        return out, err

    def ping_check(self, target_host="192.168.8.1", port=22, timeout=2):
        """Kiểm tra Socket TCP port 22 tới Beryl 7"""
        try:
            sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
            sock.settimeout(timeout)
            result = sock.connect_ex((target_host, port))
            sock.close()
            if result == 0:
                return True
            return False
        except Exception:
            return False

    def execute_with_guardrail(self, executor_func, *args, countdown_seconds=30, **kwargs):
        """
        Thực thi hàm Executor với cơ chế bảo vệ Guardrail 30s & Hardened Hardware Rollback.
        Khắc phục Hạng mục B & E: Base64-encode safe script upload + chmod 700 an toàn.
        """
        checkpoint_file = "/tmp/agent_checkpoint.uci"
        fallback_script = "/tmp/watchdog_fallback.sh"

        safe_checkpoint = shlex.quote(checkpoint_file)
        safe_fallback = shlex.quote(fallback_script)

        client = paramiko.SSHClient()
        client.load_system_host_keys()
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

            print(f"[WATCHDOG] 1. Khởi tạo điểm sao lưu Checkpoint tại '{checkpoint_file}'...")
            self._exec_ssh_cmd(client, f"uci export > {safe_checkpoint}")

            print(f"[WATCHDOG] 2. Kích hoạt Local Hardware Watchdog (Tự động Rollback sau {countdown_seconds}s trên Router)...")
            script_content = f"""#!/bin/sh
sleep {countdown_seconds}
if [ -f {safe_checkpoint} ]; then
    uci import < {safe_checkpoint}
    uci commit
    /etc/init.d/firewall reload >/dev/null 2>&1
    /etc/init.d/network reload >/dev/null 2>&1
    rm -f {safe_checkpoint}
fi
"""
            # Mã hóa Base64 đảm bảo 100% an toàn không bị lỗi shell quoting
            b64_script = base64.b64encode(script_content.encode('utf-8')).decode('utf-8')
            self._exec_ssh_cmd(client, f"echo '{b64_script}' | base64 -d > {safe_fallback} && chmod 700 {safe_fallback}")
            
            client.exec_command(f"nohup {safe_fallback} >/dev/null 2>&1 &")

        except Exception as e:
            return {"success": False, "error": f"Lỗi khởi tạo Watchdog: {str(e)}", "rolled_back": False}
        finally:
            client.close()

        print(f"[WATCHDOG] 3. Đang thực thi lệnh thay đổi cấu hình mạng từ AI...")
        exec_result = executor_func(*args, **kwargs)

        if not exec_result.get("success", False):
            print("[WATCHDOG] Lệnh thi hành thất bại! Tiến hành hủy Watchdog...")
            self._cancel_fallback_script(checkpoint_file, fallback_script)
            return exec_result

        print(f"[WATCHDOG] 4. Vòng lặp đếm ngược {countdown_seconds}s & Kiểm tra sức khỏe kết nối (PING mỗi 2s)...")
        start_time = time.time()
        health_passed = True

        while (time.time() - start_time) < min(countdown_seconds - 5, 15):
            time.sleep(2)
            is_alive = self.ping_check(target_host=self.hostname, port=self.port, timeout=2)
            elapsed = int(time.time() - start_time)
            
            if is_alive:
                print(f"  [Health Check {elapsed}s] Socket SSH OK (Mạng hoạt động bình thường).")
            else:
                print(f"  [Health Check {elapsed}s] KHÔNG PHẢN HỒI! Phát hiện rớt mạng.")
                health_passed = False
                break

        if health_passed:
            print("[WATCHDOG] Sức khỏe mạng ĐẠT CHUẨN! Hủy đếm ngược Rollback và xác nhận cấu hình mới.")
            self._cancel_fallback_script(checkpoint_file, fallback_script)
            exec_result["rolled_back"] = False
            return exec_result
        else:
            print("[WATCHDOG] RỚT MẠNG KHI THỬ LỆNH MỚI! Kích hoạt Rollback về cấu hình cũ...")
            self._force_rollback_now(checkpoint_file)
            exec_result["success"] = False
            exec_result["rolled_back"] = True
            exec_result["error"] = "Tự động Rollback do lệnh mới làm rớt kết nối mạng!"
            return exec_result

    def _cancel_fallback_script(self, checkpoint_file, fallback_script):
        try:
            client = paramiko.SSHClient()
            client.load_system_host_keys()
            client.set_missing_host_key_policy(paramiko.RejectPolicy())
            client.connect(hostname=self.hostname, port=self.port, username=self.username, password=self.password, timeout=3)
            self._exec_ssh_cmd(client, f"rm -f {checkpoint_file} {fallback_script} && pkill -f watchdog_fallback.sh")
            client.close()
        except Exception:
            pass

    def _force_rollback_now(self, checkpoint_file):
        try:
            client = paramiko.SSHClient()
            client.load_system_host_keys()
            client.set_missing_host_key_policy(paramiko.RejectPolicy())
            client.connect(hostname=self.hostname, port=self.port, username=self.username, password=self.password, timeout=3)
            self._exec_ssh_cmd(client, f"uci import < {checkpoint_file} && uci commit && /etc/init.d/firewall reload && /etc/init.d/network reload && rm -f {checkpoint_file}")
            client.close()
        except Exception:
            pass

# Export alias cho unit tests compatibility
GuardedWatchdog = RouterWatchdog
