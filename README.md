# 🚀 GL.iNet Beryl 7 (GL-MT3600BE) Self-Evolving AI Edge Router Agent

> **Hệ thống AI Edge Router Tự khắc phục Sự cố & Tự tiến hóa Tri thức cho GL.iNet Beryl 7 (Wi-Fi 7 / Filogic 850 / OpenWrt 21.02)**

![Python Version](https://img.shields.io/badge/python-3.14%2B-blue)
![OpenWrt Version](https://img.shields.io/badge/OpenWrt-21.02-green)
![AI Model](https://img.shields.io/badge/AI-Google%20Gemini%202.5%20Flash-orange)
![Security Guardrail](https://img.shields.io/badge/Watchdog-30s%20Auto--Rollback-red)
![License](https://img.shields.io/badge/license-MIT-blue)

---

## 💡 Tổng quan Hệ thống (System Architecture)

**Beryl 7 AI Edge Router Agent** là giải pháp quản trị mạng thế hệ mới, biến chiếc Router du lịch cao cấp GL.iNet Beryl 7 thành một **"Siêu Router Tự chữa lành (Self-Healing Router)"**. 

Hệ thống hoạt động theo mô hình **Laptop-Offloaded Architecture** (chạy trên Laptop kết nối SSH tới Router), giúp Router tiêu tốn **0 MB Flash** và **< 0.5 MB RAM**, đảm bảo an toàn phần cứng 100%.

```text
       ┌────────────────────────────────────────────────────────┐
       │   GL.iNet Beryl 7 (Filogic 850 / OpenWrt 21.02)         │
       └───────────────────────────┬────────────────────────────┘
                                   │ (Telemetry / Logread / ubus)
                                   ▼
       ┌────────────────────────────────────────────────────────┐
       │             Python Self-Evolving Orchestrator          │
       └─────┬────────────────────────────────────────────┬─────┘
             │                                            │
   (Cache Hit: ~0.73ms)                         (Cache Miss)
             ▼                                            ▼
┌─────────────────────────┐                  ┌─────────────────────────┐
│ SQLite Skill Store      │                  │ Google Gemini 2.5 Flash │
│ (Exponential Decay EMA) │                  │ Function Calling API    │
└────────────┬────────────┘                  └────────────┬────────────┘
             │                                            │
             └─────────────────────┬──────────────────────┘
                                   ▼
       ┌────────────────────────────────────────────────────────┐
       │ Guarded Watchdog (30s Local Hardware Fallback Script)   │
       └────────────────────────────────────────────────────────┘
```

---

## 🔥 Tính năng Nổi bật (Core Features)

1. **Self-Healing & Anomaly Detection:** Tự động phát hiện rớt kết nối WAN, tràn bộ nhớ (OOM), nghẽn CPU, ngắt kết nối Wi-Fi lặp lại theo thời gian thực.
2. **AI Function Calling (Gemini 2.5 Flash):** AI đọc Telemetry & Anomaly Report đã được mã hóa an toàn qua `DataSanitizer` để đưa ra quyết định hành động dạng JSON Function Call (`restart_interface`, `set_qos_priority`, `block_device`, `optimize_wifi_channel`).
3. **SQLite Self-Evolution Skill Store:** 
   * **Cache Miss:** Hỏi AI Cloud -> Kiểm thử an toàn -> Lưu tri thức vào SQLite.
   * **Cache Hit:** Lần sau gặp lại lỗi cũ -> Rút kỹ năng từ SQLite Local xử lý trong **0.73ms (Nhanh hơn 2520 lần, 0 VNĐ API Cost)**!
   * Thuật toán **EMA (Exponential Moving Average)** tự động điều chỉnh điểm tin cậy `confidence_score` và tự đào thải kỹ năng hỏng khi `< 0.5`.
4. **Bảo hiểm An toàn phần cứng 100% (Guarded Watchdog):** 
   * Trước khi thi hành lệnh thay đổi cấu hình mạng (`uci`), hệ thống tạo điểm sao lưu `uci export` và kích hoạt script tự khôi phục ngầm trên Router (`/tmp/watchdog_fallback.sh`).
   * Dù đứt SSH hay sập Wi-Fi, Router phần cứng tự động khôi phục mạng sau 30 giây.
   * Hỗ trợ nạp Firmware nguyên bản qua **U-Boot Web UI (`192.168.1.1`)** cứu hộ tuyệt đối.

---

## 🛠️ Hướng dẫn Cài đặt & Khởi chạy (Getting Started)

### 1. Requirements (Yêu cầu Môi trường)
- **Laptop:** Python 3.10+ (Đã thử nghiệm hoàn hảo trên Python 3.14).
- **Router:** GL.iNet MT3600BE (Beryl 7) đã bật SSH (IP mặc định: `192.168.8.1`).
- **Google Gemini API Key:** Lấy miễn phí tại [Google AI Studio](https://aistudio.google.com/).

---

### 2. Thiết lập Environment File (`.env`)
Tạo file `.env` từ file mẫu `.env.example` tại thư mục gốc dự án (Tệp `.env` được bảo vệ bởi `.gitignore` không bao giờ bị đẩy lên Git):

```bash
cp .env.example .env
```

Nội dung file `.env`:
```ini
# Router Credentials
ROUTER_HOST=192.168.8.1
ROUTER_PORT=22
ROUTER_USER=root
ROUTER_PASSWORD=Mật_khẩu_root_Router_của_bạn

# Google Gemini API Key
GEMINI_API_KEY=AIzaSy...
```

---

### 3. Cài đặt Môi trường ảo Python & Thư viện
```powershell
# Tạo môi trường ảo venv
python -m venv venv

# Kích hoạt venv (Windows PowerShell)
.\venv\Scripts\Activate.ps1

# Cài đặt các thư viện phụ thuộc
pip install -r requirements.txt
```

---

### 4. Chạy Unit Tests Local (Kiểm thử An toàn)
Chạy toàn bộ 21 Unit Tests trong thư mục `tests/` để đảm bảo hệ thống không có bất kỳ lỗi logic nào trước khi kết nối tới Router:

```powershell
.\venv\Scripts\python -m unittest discover -s tests
```

---

### 5. Khởi chạy Beryl 7 AI Agent

#### 🔹 Chế độ Dry-Run An toàn (Khuyên dùng thử nghiệm):
```powershell
.\venv\Scripts\python scripts/start_agent.py
```

#### 🔹 Chế độ Live (Cho phép AI ra lệnh thật xuống Router):
```powershell
.\venv\Scripts\python scripts/start_agent.py --live
```

---

## 📁 Cấu trúc Thư mục Dự án (Project Structure)

```text
Beryl7-AI-Agent/
├── agent/
│   ├── telemetry.py       # Thu thập dữ liệu Ubus, CPU, RAM, Leases, RX/TX
│   ├── log_parser.py      # Phân tích Logread & Phát hiện Anomaly
│   ├── sanitizer.py       # Lọc mật khẩu, WPA keys, root secrets
│   ├── ai_client.py       # Gemini 2.5 Flash Function Calling API
│   ├── executor.py        # Động cơ thi hành lệnh UCI / OpenWrt
│   ├── watchdog.py        # Backup UCI Checkpoint & Fallback 30s
│   ├── skill_store.py     # Bộ nhớ SQLite Skill Store & Thuật toán EMA
│   ├── orchestrator.py    # Bộ não trung tâm điều phối toàn bộ hệ thống
│   ├── logger.py          # Hệ thống Logging tập trung chuẩn Production
│   └── retry.py           # Retry logic tự động thử lại khi SSH/API chập chờn
├── tests/
│   ├── test_executor.py   # Unit tests cho Executor
│   ├── test_watchdog.py   # Unit tests cho Watchdog
│   ├── test_skill_store.py# Unit tests cho SQLite Skill Store
│   └── test_orchestrator.py# Unit tests cho Orchestrator
├── scripts/
│   ├── start_agent.py     # Launcher chính thức khởi chạy Agent
│   ├── benchmark_metrics.py# Đo đạc thông số thực nghiệm (Latency, Cost)
│   └── run_live_demo.py   # Kịch bản trình diễn Live Demo
├── docs/
│   └── THESIS_BENCHMARK_REPORT.md # Báo cáo đo đạc thực nghiệm chi tiết
├── .env.example           # File mẫu cấu hình biến môi trường
├── requirements.txt       # Danh sách thư viện Python
└── README.md              # Tài liệu hướng dẫn sử dụng
```

---

## 🔒 Security & Safety Disclaimers (Bảo mật & An toàn)

- **Zero-Credential Exposure:** Mọi credential (IP, Mật khẩu SSH, API Key) đều được quản lý tập trung qua biến môi trường `.env` và tuyệt đối không bao giờ được hard-code vào mã nguồn.
- **Dry-Run Default:** Hệ thống mặc định khởi chạy ở chế độ Dry-Run (chỉ in câu lệnh giả lập) trừ khi có cờ `--live`.
- **Failsafe Hardware Recovery:** Router Beryl 7 trang bị phân vùng U-Boot Web UI (`http://192.168.1.1`) ở ROM chỉ đọc, đảm bảo khả năng nạp lại firmware cứu hộ trong mọi tình huống rủi ro.

---

## 📄 License
Dự án được phân phối dưới giấy phép [MIT License](LICENSE).
