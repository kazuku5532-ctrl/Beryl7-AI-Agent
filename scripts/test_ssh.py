import sys
import socket
import argparse
import paramiko

def test_ssh_connection(hostname, port, username, password=None, key_filename=None, timeout=5):
    """
    Kiểm thử kết nối SSH đến Router Beryl 7 với đầy đủ xử lý ngoại lệ (Exception Handling).
    """
    print(f"🔄 Đang kết nối tới SSH Server tại {username}@{hostname}:{port} (Timeout: {timeout}s)...")
    
    client = paramiko.SSHClient()
    # Tự động chấp nhận Host Key nếu lần đầu kết nối (Auto-Add Policy)
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    
    try:
        client.connect(
            hostname=hostname,
            port=port,
            username=username,
            password=password,
            key_filename=key_filename,
            timeout=timeout,
            banner_timeout=timeout
        )
        print("✅ KẾT NỐI SSH THÀNH CÔNG!")
        
        # Thử thực thi 1 lệnh đọc thông tin hệ thống OpenWrt
        cmd = "cat /etc/openwrt_release"
        print(f"📖 Đang chạy lệnh thử nghiệm: '{cmd}'...")
        stdin, stdout, stderr = client.exec_command(cmd)
        
        output = stdout.read().decode('utf-8').strip()
        error = stderr.read().decode('utf-8').strip()
        
        if output:
            print("\n--- [ THÔNG TIN HỆ ĐIỀU HÀNH OPENWRT ] ---")
            print(output)
            print("-------------------------------------------\n")
            return True
        elif error:
            print(f"⚠️ Lệnh chạy có lỗi: {error}")
            return False
            
    except paramiko.AuthenticationException:
        print("❌ LỖI XÁC THỰC (Authentication Error): Sai tên đăng nhập hoặc mật khẩu Root!")
    except paramiko.SSHException as ssh_err:
        print(f"❌ LỖI GIAO THỨC SSH (SSH Protocol Error): {ssh_err}")
    except socket.timeout:
        print(f"❌ LỖI TIMEOUT: Không thể phản hồi từ {hostname} sau {timeout}s. Vui lòng kiểm tra lại IP Router hoặc cáp mạng!")
    except socket.error as sock_err:
        print(f"❌ LỖI KẾT NỐI MẠNG (Socket Error): {sock_err}. Không thể tới được IP {hostname}.")
    except Exception as e:
        print(f"❌ LỖI KHÔNG XÁC ĐỊNH: {type(e).__name__} - {e}")
    finally:
        client.close()
        
    return False

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Tool kiểm thử kết nối SSH tới Router Beryl 7")
    parser.add_argument("--host", default="192.168.8.1", help="Địa chỉ IP Router (mặc định: 192.168.8.1)")
    parser.add_argument("--port", type=int, default=22, help="Cổng SSH (mặc định: 22)")
    parser.add_argument("--user", default="root", help="Username (mặc định: root)")
    parser.add_argument("--password", default=None, help="Mật khẩu root (nếu có)")
    parser.add_argument("--key", default=None, help="Đường dẫn file SSH Key (nếu dùng key)")
    parser.add_argument("--timeout", type=int, default=5, help="Thời gian chờ timeout (giây)")
    
    args = parser.parse_args()
    
    success = test_ssh_connection(
        hostname=args.host,
        port=args.port,
        username=args.user,
        password=args.password,
        key_filename=args.key,
        timeout=args.timeout
    )
    
    sys.exit(0 if success else 1)
