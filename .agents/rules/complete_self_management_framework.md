# 🏛️ Complete Self-Management Framework Rule (Triết lý Mạng Tự Trị Toàn Diện)

## 1. Nguyên lý Chỉ đạo Cao nhất (Ultimate Supreme Goal)
Mạng Tự trị Toàn diện (Complete Self-Management Framework) chính là **gốc rễ, mục tiêu tối thượng và yếu tố đứng đầu chuỗi kiến trúc Beryl 7**. Mọi tính năng, module hay tối ưu hóa đều phải phục vụ cho khung tự trị này.

## 2. 5 Nhánh Kiến trúc Cốt lõi Hợp nhất
Khung Mạng Tự trị Toàn diện của Beryl 7 hợp nhất 5 nhánh tự trị bắt buộc:

1. **Self-Optimizing (Tự Tối Ưu Mạng):**
   - Tự động nhảy băng thông 5GHz Wi-Fi 80MHz <-> 160MHz (EHT160) theo lưu lượng WAN.
   - Làm mịn chỉ số telemetry bằng bộ lọc EWMA và phát hiện bất thường Z-Score.
   - Dọn dẹp các phiên NAT Conntrack rác.

2. **Self-Securing / Self-Protecting (Tự Bảo Mật & Tự Phòng Vệ):**
   - Rate Limiting 60 req/min per IP chống Brute-force/DoS.
   - Phân quyền RBAC Token 256-bit (`AUTH_TOKEN` & `APPROVE_TOKEN`).
   - Bọc tham số shell bằng `shlex.quote()` chống OS Command Injection.
   - Tự động xóa vết credential (`***REDACTED***`) trước khi ghi log.
   - Giới hạn CORS origin chỉ cho LAN subnets và `null`.

3. **Self-Smoothing (Tự Tối Ưu Tiến Trình Chạy Mượt Mà):**
   - Khóa bất đồng bộ không tranh chấp (`cfgAtomic.Load` / `cfgAtomic.Store`) cho độ trễ 0ms.
   - CSDL SQLite WAL mode đọc/ghi song song không lock.
   - Kiểm tra xác minh kết quả async non-blocking trên goroutine ngầm.
   - Quản lý RAM chủ động giữ footprint luôn cực nhẹ (< 16MB).

4. **Self-Healing (Tự Phục Hồi Sự Cố):**
   - Hardware Watchdog 30s Dead Man's Switch & 15s health check loop.
   - Sao lưu điểm khôi phục `/tmp/agent_checkpoint.uci` và tự động rollback.
   - Khôi phục CSDL tự động khi bị hỏng (SQLite Auto-Dump Recovery).

5. **Self-Configuring (Tự Cấu Hình Thích Nghi):**
   - Multi-Firmware Compatibility Matrix (OpenWrt / GL.iNet 4.9.0, 5.0+, 24.10 ubus RPC).
   - Tự bảo tồn cấu hình qua `/etc/sysupgrade.conf`.

## 3. Gắn chặt với các Quy tắc Kỷ luật Kỹ thuật & Khiêm tốn

- **Đồng bộ Bánh răng Hệ thống (Clockwork Alignment):**
  Mỗi khi thay đổi hay nâng cấp bất kỳ thành phần nào, phải kiểm tra toàn bộ hệ thống (Toàn bộ 10 package Go, ARM64 build, nạp router thực tế). Không bao giờ hạ thấp cảnh giác.

- **Phòng ngừa Lỗi Gián tiếp & Lỗi "Không ngờ tới":**
  Lỗi có thể sinh ra từ ngay chính nơi vừa sửa đổi, hoặc từ các thành phần không liên quan trực tiếp nhưng liên quan gián tiếp. Phải thực hiện kiểm thử diện rộng, không bỏ qua bất kỳ chi tiết nhỏ nào để bảo đảm không một lỗi "không ngờ tới" nào xuất hiện.

- **Ngôn từ Khiêm tốn & Chuyên nghiệp (Modesty & Professionalism):**
  Trình bày trung thực, ngắn gọn, dựa trên số liệu thực tế đo đạc được. Tuyệt đối không dùng từ ngữ phô trương, tâng bốc hay khoa trương.
