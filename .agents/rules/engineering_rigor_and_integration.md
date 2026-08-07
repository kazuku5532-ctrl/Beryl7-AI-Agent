# 🛡️ Mandatory Security, Workflow Rigor & Modesty Rule (Quy tắc Kỷ luật Kỹ thuật & Quy trình Toàn diện)

## 1. Cấm Tự tin Thái quá & Cấm Tuyên bố "Hoàn hảo/Không còn lỗi"
- Bất kỳ đợt "Tổng rà soát" nào cũng **KHÔNG ĐƯỢC PHÉP chỉ dựa vào việc test case hay `go vet` chạy qua**.
- Tuyệt đối cấm tuyên bố "an toàn tuyệt đối", "không còn lỗi" khi chưa thực sự soi từng dòng mã nguồn dưới 5 chiều an toàn bắt buộc.

## 2. Danh mục Kiểm tra 5 Chiều An toàn An ninh Mạng (5-Point Security Checklist)

Mọi đợt rà soát mã nguồn bắt buộc phải kiểm tra thủ công 5 chiều sau:

1. **🌐 Strict URL & Origin Host Matching (An toàn CORS/Domain):**
   - CẤM MẠNH BẠO việc dùng `strings.HasPrefix` để soi URL / Origin.
   - BẮT BUỘC dùng `url.Parse(origin)` và kiểm tra chính xác `u.Hostname()` để tránh các lỗ hổng bypass dạng `http://localhost.attacker.com` (CWE-290 protection).

2. **🔑 Zero Hardcoded Plaintext Credentials (An toàn Secret):**
   - CẤM chứa bất kỳ mật khẩu, token, API Key dạng chuỗi thô (clear-text) trong BẤT KỲ tệp tin nào (kể cả script dev, script test hay file phụ trợ trong `tools/`).
   - Bắt buộc nạp 100% qua biến môi trường hoặc tệp ẩn phân quyền `0400`/`0600` (Linear O(N) Shannon Entropy scanner).

3. **🪲 IP Dual-Stack & Loopback Boundary (Boundary IPv4/IPv6):**
   - Kiểm tra các truy cập cục bộ phải xử lý triệt để IPv4-mapped IPv6 (`::ffff:127.0.0.1`, `::ffff:7f00:1`).
   - Phải kết hợp `net.ParseIP(host).IsLoopback()` thay vì so sánh chuỗi đơn thuần.

4. **🔄 Real Process RAM Lifecycle (Thực tế Tiến trình Linux):**
   - Đổi tên hay ghi đè file trên đĩa cứng KHÔNG LÀM THAY ĐỔI tiến trình đang chạy trong RAM.
   - Khi rollback hay nâng cấp binary, BẮT BUỘC phải có logic kích hoạt khởi động lại dịch vụ (`/etc/init.d/... restart`) hoặc `syscall.Exec` để nạp binary mới vào RAM.

5. **🔌 Bound Socket vs In-Memory Struct Realities (Thực tế Cổng Mạng):**
   - Cập nhật struct cấu hình trong RAM không tự đổi cổng HTTP Server đã bind.
   - Phải có cảnh báo hoặc logic restart socket khi thay đổi các tham số mạng (`HEALTH_PORT`, `BIND_HOST`).

## 3. Quy trình Tham vấn Ý kiến Người dùng (Consultation before Assumptions)
- Khi nhận được góp ý hoặc yêu cầu có sự xung đột / đánh đổi kiến trúc (ví dụ: góc độ Cá nhân vs Phát hành Rộng rãi Doanh nghiệp), Agent **BẮT BUỘC phải phân tích minh bạch các mặt lợi/hại và xin ý kiến chỉ đạo của Sếp trước**.
- Tuyệt đối không tự ý áp đặt suy nghĩ cá nhân hoặc tự ý sửa đổi kiến trúc cốt lõi mà không tham vấn.

## 4. Chuỗi Thực nghiệm 3 Bước Bắt buộc (Three-Phase Verification Pipeline)
Không bao giờ tuyên bố nhiệm vụ hoàn thành khi chưa trải qua đủ 3 bước:
1. **Bước 1 (Workstation Test):** Chạy `go test ./...` đạt 100% PASS và `go vet ./...` 100% CLEAN.
2. **Bước 2 (ARM64 Cross-Compile & Deploy):** Biên dịch nhị phân Linux ARM64 và nạp trực tiếp lên Router phần cứng thật.
3. **Bước 3 (Live Empirical Verification):** Chạy lệnh kiểm tra trực tiếp trên Router (curl `/api/health`, check `ps`, `VmRSS`, `nohup.log`) để lấy bằng chứng thực chứng từ thiết bị thật.

## 5. Thái độ Tiếp thu & Ngôn từ Khiêm tốn
- Luôn giữ thái độ khiêm tốn, lắng nghe mọi góp ý của người dùng/reviewer.
- Nhận thức rõ sơ suất, sửa đổi tận gốc và trình bày dựa trên dữ liệu thực chứng.

## 6. Sáu Chốt Chặn Kỷ Luật Cứng (6 Strict Discipline Checkpoints)
Mọi đợt phát triển, tái cấu trúc hoặc sửa lỗi bắt buộc phải vượt qua 6 chốt chặn sau:

1. **Complete Self-Management Framework:** Tối ưu hóa 5 nhánh tự trị (Optimizing, Securing, Smoothing, Healing, Configuring) không để lọt hạt sạn kỹ thuật.
2. **Security & Entropy Audit:** 0% secret thô, kiểm tra CORS theo RFC 1918 qua `url.Parse` (CWE-290), enforce Paramiko SSH `RejectPolicy` mặc định chống Man-in-the-Middle (CWE-300/CWE-200).
3. **Process & RAM Lifecycle:** Sử dụng `os.Executable()` path resolution động, `syscall.Exec` nạp binary vào RAM, hủy `context.Context` bất đồng bộ sạch sẽ.
4. **Paramiko IO Safety:** Đọc `stdout.read()` và `stderr.read()` trước khi gọi `recv_exit_status()` (chống bế tắc I/O CWE-833), stream binary SSH Paramiko.
5. **Clockwork Alignment:** 10/10 Go packages PASS `go test` và `go vet`, kích hoạt SAST `gosec` G204 active.
6. **Live Hardware Empirical Test:** Lắp ghép quy trình HIL (Hardware-in-the-Loop) nạp trực tiếp lên GL-MT3600BE (Filogic 820 ARM64), duy trì VmRSS RAM < 16MB, CPU < 5%, HTTP 200 OK.
