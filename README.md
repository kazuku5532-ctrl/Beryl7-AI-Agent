# Beryl 7 (GL-MT3600BE) Self-Evolving AI Edge Router Agent

## 📌 Tổng quan đồ án
Đồ án tốt nghiệp xây dựng hệ thống **Zero-Touch Network Automation & Self-Evolving AI Agent** tích hợp trên Router du lịch **GL.iNet Beryl 7 (GL-MT3600BE)** chạy hệ điều hành **OpenWrt**.

## 🏗️ Kiến trúc Hệ thống
- **Edge Agent (OpenWrt - Golang):** Lắng nghe ubus, logread, thu thập telemetry và thực thi lệnh qua `uci`/`ubus`/`nftables`.
- **Cloud AI (Google Gemini API):** Phân tích ngữ cảnh, xử lý lỗi chưa biết và đưa ra quyết định hành động qua cơ chế **Function Calling**.
- **Skill Store (SQLite):** Lưu trữ các kịch bản tự điều chỉnh/sửa lỗi đã qua kiểm chứng (Self-Healing) để tái sử dụng mà không cần truy vấn lại AI.
- **Safety Rollback Guardrail:** Tự động khôi phục cấu hình sau 30 giây nếu thay đổi mạng làm mất kết nối.

## 🚀 Cấu trúc dự án
```text
Beryl7-AI-Agent/
├── .gitignore
├── README.md
├── docs/               # Tài liệu thiết kế & sơ đồ kiến trúc
├── cmd/                # Entrypoints ứng dụng Go
│   └── agent/
├── pkg/                # Các package module chính (router, ai, skillstore)
└── scripts/            # Kịch bản deployment & test
```

## 🛡️ An toàn Phần cứng
Thiết bị sử dụng cơ chế cứu hộ phần cứng **U-Boot Web UI** (`http://192.168.1.1`), đảm bảo an toàn tuyệt đối 100% không lo rủi ro phần cứng trong quá trình phát triển.
