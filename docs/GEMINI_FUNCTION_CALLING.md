# Tài liệu Chi tiết Gemini Function Calling Schemas & Actions

> **Tài liệu quy định các Tool / Function Declarations mà Google Gemini 2.5 Flash API sử dụng để điều khiển Router GL.iNet Beryl 7.**

---

## 🛠️ Danh sách 5 Tools / Functions Chuẩn mực

### 1. `restart_interface` (Khởi động lại Interface Mạng)
- **Mô tả:** Khởi động lại card mạng WAN/LAN/Wi-Fi khi phát hiện mất kết nối hoặc rớt gói tin.
- **Parameters Schema:**
  ```json
  {
    "type": "OBJECT",
    "properties": {
      "interface_name": { "type": "STRING", "description": "Tên interface (e.g. 'wan', 'br-lan', 'ra0', 'rai0')" },
      "reason": { "type": "STRING", "description": "Lý do khởi động lại interface" }
    },
    "required": ["interface_name", "reason"]
  }
  ```
- **Ví dụ AI Return JSON:**
  ```json
  {
    "tool_name": "restart_interface",
    "arguments": {
      "interface_name": "wan",
      "reason": "Cổng WAN ngắt kết nối rớt gói 100%."
    }
  }
  ```

---

### 2. `optimize_wifi_channel` (Tối ưu Kênh Wi-Fi 7)
- **Mô tả:** Chuyển đổi kênh Wi-Fi sang kênh tối ưu khi phát hiện nhiễu sóng hoặc ngắt kết nối lặp lại.
- **Parameters Schema:**
  ```json
  {
    "type": "OBJECT",
    "properties": {
      "band": { "type": "STRING", "description": "Băng tần: '2.4G' hoặc '5G'" },
      "channel": { "type": "INTEGER", "description": "Số kênh Wi-Fi mới (1-165)" },
      "reason": { "type": "STRING", "description": "Lý do điều chỉnh kênh Wi-Fi" }
    },
    "required": ["band", "channel", "reason"]
  }
  ```
- **Ví dụ AI Return JSON:**
  ```json
  {
    "tool_name": "optimize_wifi_channel",
    "arguments": {
      "band": "5G",
      "channel": 36,
      "reason": "Phát hiện ngắt kết nối Wi-Fi 5G lặp lại do nhiễu sóng."
    }
  }
  ```

---

### 3. `set_qos_priority` (Giới hạn / Ưu tiên Băng thông)
- **Mô tả:** Thiết lập độ ưu tiên QoS hoặc giới hạn băng thông cho thiết bị mạng.
- **Parameters Schema:**
  ```json
  {
    "type": "OBJECT",
    "properties": {
      "target_mac": { "type": "STRING", "description": "Địa chỉ MAC thiết bị" },
      "priority": { "type": "STRING", "description": "HIGH, MEDIUM, LOW" },
      "max_bandwidth_mbps": { "type": "INTEGER", "description": "Băng thông tối đa (Mbps)" },
      "reason": { "type": "STRING", "description": "Lý do phân bổ băng thông" }
    },
    "required": ["target_mac", "priority", "reason"]
  }
  ```

---

### 4. `block_device` (Cách ly / Chặn Thiết bị)
- **Mô tả:** Cách ly thiết bị khỏi mạng khi phát hiện nghi ngờ xâm nhập hoặc vi phạm.
- **Parameters Schema:**
  ```json
  {
    "type": "OBJECT",
    "properties": {
      "target_mac": { "type": "STRING", "description": "Địa chỉ MAC cần chặn" },
      "reason": { "type": "STRING", "description": "Lý do chặn thiết bị" }
    },
    "required": ["target_mac", "reason"]
  }
  ```

---

### 5. `no_action_required` (Mạng Ổn định)
- **Mô tả:** Báo cáo hệ thống ổn định, không cần can thiệp.
- **Parameters Schema:**
  ```json
  {
    "type": "OBJECT",
    "properties": {
      "reason": { "type": "STRING", "description": "Lý do không cần hành động" }
    },
    "required": ["reason"]
  }
  ```
