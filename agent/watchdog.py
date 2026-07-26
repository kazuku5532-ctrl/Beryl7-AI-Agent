import sys
import time
import socket
import paramiko

if sys.platform.startswith('win'):
    try:
        sys.stdout.reconfigure(encoding='utf-8')
    except AttributeError:
        pass

class GuardedWatchdog:
    """
    Module Watchdog & Auto-Rollback Guardrail (Phao cứu sinh an toàn).
    Đảm bảo 100% không bị mất mạng vĩnh viễn khi thi hành lệnh mới từ AI.
    
    Tích hợp tính năng +1 Evolution Step:
    Tạo script Fallback tự động đếm ngược trực tiếp TRÊN ROUTER (Local Hardware Watchdog).
    Dù Laptop có bị ngắt kết nối SSH/Wi-Fi hoàn toàn, Router vẫn tự khôi phục sau 30 giây!
    """
    def __init__(self, hostname="192.168.8.1", port=22, username="root", password=None, key_filename=None, timeout=5):
        self.hostname = hostname
        self.port = port
        self.username = username
        self.password = password
        self.key_filename = key_filename
        self.timeout = timeout

    def _exec_ssh_cmd(self, client, command):
        stdin, stdout, stderr = client.exec_command(command)
        out = stdout.read().decode('utf-8', errors='ignore').strip()
        err = stderr.read().decode('utf-8', errors='ignore').strip()
        return out, err

    def ping_check(self, target_host="192.168.8.1", port=22, timeout=2):
        """Kiểm tra độ trễ và khả năng sống sót của socket kết nối"""
        try:
            sock = socket.create_connection((target_host, port), timeout=timeout)
            sock.close()
            return True
        except Exception:
            return False

    def execute_with_guardrail(self, executor_func, *args, countdown_seconds=30, **kwargs):
        """
        Thực thi hàm Executor với cơ chế bảo vệ Guardrail 30s & Hardened Hardware Rollback.
        """
        checkpoint_file = "/tmp/agent_checkpoint.uci"
        fallback_script = "/tmp/watchdog_fallback.sh"

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
            self._exec_ssh_cmd(client, f"uci export > {checkpoint_file}")

            # +1 EVOLUTION STEP: Tạo Local Hardware Fallback Script chạy ngầm trên Router
            print(f"[WATCHDOG] 2. Kích hoạt Local Hardware Watchdog (Tự động Rollback sau {countdown_seconds}s trên Router)...")
            script_content = f"""#!/bin/sh
sleep {countdown_seconds}
if [ -f {checkpoint_file} ]; then
    uci import < {checkpoint_file}
    uci commit
    /etc/init.d/firewall reload >/dev/null 2>&1
    /etc/init.d/network reload >/dev/null 2>&1
    rm -f {checkpoint_file}
fi
"""
            self._exec_ssh_cmd(client, f"echo '{script_content}' > {fallback_script} && chmod +x {fallback_script}")
            # Chạy ngầm fallback script trên Router
            client.exec_command(f"nohup {fallback_script} >/dev/null 2>&1 &")

        except Exception as e:
            return {"success": False, "error": f"Lỗi khởi tạo Watchdog: {str(e)}", "rolled_back": False}
        finally:
            client.close()

        # 3. Thực thi lệnh thay đổi từ AI
        print(f"[WATCHDOG] 3. Đang thực thi lệnh thay đổi cấu hình mạng từ AI...")
        exec_result = executor_func(*args, **kwargs)

        if not exec_result.get("success", False):
            print("[WATCHDOG] Lệnh thi hành thất bại! Tiến hành hủy Watchdog...")
            self._cancel_fallback_script(checkpoint_file, fallback_script)
            return exec_result

        # 4. Vòng lặp đếm ngược 30 giây và kiểm tra sức khỏe mạng (Health Check)
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

        # 5. Đánh giá kết quả
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
        """Xóa file checkpoint và dừng script fallback khi lệnh chạy thành công"""
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
        """Khôi phục ngay lập tức cấu hình cũ"""
        try:
            client = paramiko.SSHClient()
            client.load_system_host_keys()
            client.set_missing_host_key_policy(paramiko.RejectPolicy())
            client.connect(hostname=self.hostname, port=self.port, username=self.username, password=self.password, timeout=3)
            self._exec_ssh_cmd(client, f"uci import < {checkpoint_file} && uci commit && /etc/init.d/firewall reload && /etc/init.d/network reload && rm -f {checkpoint_file}")
            client.close()
        except Exception:
            pass
