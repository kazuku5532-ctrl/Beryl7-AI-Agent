---
trigger: always_on
---

# 🧠 Mandatory Cognitive Bias & Reasoning Framework (Bộ khung Tư duy Phản biện Bắt buộc)

Quy tắc này áp dụng CHO MỌI phiên làm việc trước khi thiết kế, refactor, hoặc review
kiến trúc — kể cả khi chạy qua `/upgrade_pipeline`. Đây là lớp tư duy PHẢI thực hiện
TRƯỚC khi áp dụng 5-Point Security Checklist trong `engineering_rigor_and_integration.md`,
không thay thế nó.

Đây là bộ khung gồm 5 kỹ thuật tư duy ĐỘC LẬP, không phải một kỹ thuật duy nhất — mỗi
mục dưới đây là một lăng kính riêng, cần áp dụng phối hợp chứ không thay thế lẫn nhau:
(1) Multidimensional Thinking, (2) Survivorship Bias check, (3) First Principles
Thinking, (4) Systems Thinking, (5) Overconfidence Bias self-check.

## 1. Multidimensional Thinking — Cấm Tư duy Tuyến tính Một Chiều

Agent KHÔNG ĐƯỢC PHÉP đánh giá một thay đổi kiến trúc chỉ qua MỘT góc nhìn duy nhất
(ví dụ chỉ soi "có chạy được không" mà bỏ qua "có an toàn không", hoặc chỉ soi
"đã test pass" mà bỏ qua "có nhất quán với các module khác không").

Trước khi kết luận một thiết kế/thay đổi là ổn, BẮT BUỘC tự hỏi qua ít nhất các lăng
kính sau (chọn lăng kính phù hợp với thay đổi, không cần áp dụng máy móc toàn bộ):

- **An toàn vật lý:** Thay đổi này có thể làm hỏng/khóa router thật không?
- **Tính nhất quán dữ liệu:** Dữ liệu có mâu thuẫn giữa các nguồn/module không?
- **Bảo mật:** Bề mặt tấn công mới là gì? Ai có quyền gọi hành động này?
- **Vòng đời vận hành:** Điều gì xảy ra khi khởi động lại, mất điện, update?
- **Con người:** Người vận hành (chỉ 1 người, Zero) có cần nhớ/làm đúng điều gì không?
- **Kinh tế tài nguyên:** Chi phí CPU/RAM/flash dài hạn trên phần cứng nhúng RAM 512MB?

## 2. Survivorship Bias — Bẫy "Thiên kiến Kẻ sống sót"

KHÔNG được chỉ review phần code vừa sửa/vừa thêm — đó là phần đã "sống sót" qua nhiều
vòng test, giống máy bay quay về được. Phần NGUY HIỂM NHẤT thường là phần **CHƯA TỪNG
bị động vào, chưa từng có test fail, chưa từng bị soi** — vì im lặng không có nghĩa là
an toàn, có thể chỉ vì chưa ai từng thử.

Khi đánh giá tác động của MỘT thay đổi, bắt buộc tự hỏi: module nào KHÁC (chưa từng
đụng tới trong session này) có khả năng bị ảnh hưởng ngầm bởi thay đổi này? Ví dụ: thêm
threshold mới có được các module ra quyết định khác (không chỉ nơi vừa sửa) đọc đúng
không?

## 3. First Principles Thinking — Quay về Mục đích Gốc

Trước khi chấp nhận một pattern/kiến trúc hiện có là "đúng vì nó đã ở đó", hỏi lại:
mục đích gốc của Beryl7-AI-Agent là gì (tự động giữ mạng ổn định cho 1 router cá nhân,
ít can thiệp con người)? Thiết kế đang xét có PHỤC VỤ ĐÚNG mục đích đó không, hay chỉ
là thói quen/pattern lặp lại không cần thiết (ví dụ: logic bị copy-paste ở nhiều nơi
thay vì dùng chung 1 hàm — vi phạm chính nguyên lý "nhất quán, ít lỗi người")?

## 4. Systems Thinking — Soi Vòng phản hồi, không chỉ Module đơn lẻ

Không đánh giá một thay đổi bằng cách chỉ nhìn 1 module bị sửa. BẮT BUỘC truy theo
TOÀN BỘ vòng đời dữ liệu liên quan: Telemetry → Anomaly Detection → Q-Learning/AI
Decision → Executor → Verification → (quay lại) Q-Learning reward. Một thay đổi ở
một điểm trong vòng lặp có thể gây hậu quả ở điểm khác xa hơn trong cùng vòng lặp
(ví dụ: một endpoint test không xác thực có thể trực tiếp ghi vào cùng bảng Q-table
mà vòng lặp quyết định chính đang dùng — hậu quả không chỉ là 1 hành động sai, mà là
đầu độc dữ liệu học máy dài hạn).

## 5. Tự kiểm tra Overconfidence Bias — Vặn lại Chính Kết luận Của Mình

Sau khi đưa ra một kết luận "đã xác nhận", agent PHẢI tự hỏi lại ít nhất 1 lần: kết
luận này có dựa trên giả định chưa kiểm chứng không? Đọc lại đúng điều kiện kích hoạt
thực tế (không chỉ đọc code tồn tại, mà đọc ĐIỀU KIỆN nó được gọi) trước khi báo cáo
mức độ nghiêm trọng. Một lỗ hổng có thể nghiêm trọng HƠN đánh giá ban đầu nếu cơ chế
bảo vệ liên quan (rollback, validation) hóa ra có điều kiện kích hoạt hẹp hơn tưởng.

## 6. Áp dụng khi nào

- Khi thiết kế một tính năng mới (Technical Architect role).
- Khi review lại toàn bộ hoặc một phần kiến trúc theo yêu cầu của Zero.
- Khi một thay đổi động vào: executor (hành động vật lý), watchdog (rollback),
  bất kỳ endpoint HTTP nào, hoặc bất kỳ bảng dữ liệu nào Q-Learning/AI đọc-ghi.
- KHÔNG bắt buộc cho các thay đổi cực nhỏ, cô lập (ví dụ sửa 1 dòng comment, đổi
  tên biến nội bộ không ảnh hưởng hành vi).
