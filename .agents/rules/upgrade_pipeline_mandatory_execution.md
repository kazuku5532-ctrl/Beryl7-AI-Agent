---
trigger: always_on
---

# 🚀 Mandatory Upgrade Pipeline & Cognitive Review Enforcement (Quy tắc Kích hoạt Tuyệt đối)

Quy tắc này là **MỆNH LỆNH BẮT BUỘC BẤT KHẢ XÂM PHẠM** cho mọi phiên làm việc trên repository **Beryl7-AI-Agent**.

---

## 👑 Thứ Tự Ưu Tiên Tuyệt Đối (Supreme Hierarchy of Execution)

Khi nhận bất kỳ yêu cầu nào liên quan đến phân tích, sửa lỗi, nâng cấp kiến trúc hoặc khi có lệnh `/upgrade_pipeline`:

### 🥇 Ưu Tiên #1 (Bắt buộc Hàng đầu): Áp dụng 5 Lăng Kính `multidimensional_design_review.md`
Trước khi viết code hay đưa ra kết luận, Agent **BẮT BUỘC** phải rà soát qua 5 lăng kính:
1. **Multidimensional Thinking:** Soi 6 chiều an toàn (vật lý, dữ liệu, bảo mật, vòng đời vận hành, con người, tài nguyên phần cứng).
2. **Survivorship Bias Check:** Soi kỹ những vùng tối chưa từng có test fail (hàng đợi driver, cờ vô tuyến, log kernel/AdGuardHome).
3. **First Principles Thinking:** Đào sâu về bản chất vô tuyến & giao thức mạng thay vì chỉnh sửa bề mặt.
4. **Systems Thinking:** Truy vết toàn bộ vòng đời gói tin từ Client $\rightarrow$ Wi-Fi Driver $\rightarrow$ TC Queue $\rightarrow$ WAN $\rightarrow$ Gateway $\rightarrow$ ISP $\rightarrow$ Internet.
5. **Overconfidence Bias Self-Check:** Tự vặn lại kết luận, bắt buộc chứng minh bằng số liệu đo đạc thực chứng trên thiết bị thật.

---

### 🥈 Ưu Tiên #2 (Bắt buộc Kích hoạt): Thực thi Đầy Đủ 5 Giai Đoạn `/upgrade_pipeline`
Tuyệt đối không được nhảy cóc hay làm tắt. Mọi quy trình sửa đổi phải đi qua đủ 5 giai đoạn:
1. **🔍 Agent 1 (SAST & Security Audit):** Quét Go AST Data-Flow (CWE-290), Shannon Entropy $O(N)$ (>4.2), 5-Point Security Checklist $\rightarrow$ Lập vé Remediation Ticket.
2. **⚡ Agent 2 (Core Refactoring):** Tái cấu trúc tối thiểu, an toàn, tuân thủ 5 nhánh tự trị (Complete Self-Management Framework).
3. **🧪 Agent 3 (Test Suite & QA):** Chạy `go test ./...` (100% PASS trên toàn bộ Go packages), `go vet ./...` (100% Clean), Stress Test (20x20 ops), Soak Test (1000 cycles), `verify_rules.py` (100% PASS). Nếu lỗi: vòng lặp phản hồi tối đa 3 lần $\rightarrow$ Circuit Breaker.
4. **📦 Agent 4 (ARM64 Build & Sync):** Biên dịch nhị phân Linux ARM64 tĩnh `beryl7-agent` (< 16MB), kiểm tra Binary Sync Audit.
5. **🧹 Agent 5 (Cleaner, Security & Hardware Deploy):** Dọn dẹp sạch sẽ artifact tạm thời, nạp bản vá lên Router thật (`192.168.8.1`), kiểm tra trực tiếp qua socket/API (`/api/health`), dọn dẹp git branch thừa và giữ duy nhất nhánh `main`.

---

## 🛑 Điều Khoản Kỷ Luật Tuyệt Đối (Zero-Tolerance Invariants)
- **Không bao giờ được bỏ qua:** Trong 100% mọi trường hợp, 2 quy tắc trên KHÔNG ĐƯỢC PHÉP bị bỏ qua, bị lãng quên hay bị thay thế bởi câu trả lời phỏng đoán.
- **Bằng chứng thực chứng là tối thượng:** Không chấp nhận báo cáo "đã sửa xong" nếu chưa có số liệu đo đạc thực nghiệm từ cổng mạng/tiến trình thật trên Router.
