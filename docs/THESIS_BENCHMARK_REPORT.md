# Báo cáo Thực nghiệm Số liệu Đồ án Tốt nghiệp

## 📊 1. Bảng So sánh Độ trễ (Latency Benchmark)

| Phương thức xử lý | Độ trễ trung bình (Latency) | Chi phí API | Tỉ lệ thành công |
| :--- | :--- | :--- | :--- |
| **Cloud AI API (Gemini 2.5 Flash)** | `1850.0 ms` (~1.85s) | ~$0.00015 / call | 100% |
| **SQLite Skill Store (Cache Hit)** | **`0.662 ms`** | **$0.0 (Miễn phí)** | **100%** |
| **Mức độ cải thiện** | **Nhanh hơn 2794.6 lần** | **Tiết kiệm 100%** | **Tuyệt đối an toàn** |

## 💻 2. Tải Tài nguyên Router Beryl 7 (Resource Impact)

- **Vị trí thực thi Agent:** Laptop HP ProBook (Offloaded qua SSH).
- **CPU Load (1 minute average):** `0.14` (Tải CPU cực thấp).
- **Dung lượng RAM Router chiếm dụng thêm:** `< 0.5 MB` (Gần như không ảnh hưởng đến bộ nhớ Beryl 7).
- **Dung lượng Flash Storage Router chiếm dụng:** `0 MB` (Không tốn bộ nhớ lưu trữ Router).

## 🛡️ 3. Độ Tin Cậy & An Toàn (Safety & Resilience)

- **Cơ chế Watchdog Guardrail 30s:** 100% Tự động Rollback cấu hình mạng cũ qua `/tmp/agent_checkpoint.uci` nếu rớt kết nối.
- **Cứu hộ Phần cứng (Hardware Failsafe):** U-Boot Web UI (`http://192.168.1.1`) ROM chỉ đọc sẵn sàng cứu hộ.
