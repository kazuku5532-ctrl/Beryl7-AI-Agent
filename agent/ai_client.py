import os
import json
from google import genai
from google.genai import types
from agent.sanitizer import DataSanitizer

class RouterAIAgent:
    """
    AI Agent cho Router Beryl 7, đóng vai trò Kỹ sư Chuyên gia Mạng OpenWrt.
    Sử dụng Gemini API với cơ chế Function Calling (Tool Use) để ra quyết định xử lý.
    """
    SYSTEM_INSTRUCTION = """
Bạn là "Beryl 7 AI Network Administrator", một kỹ sư chuyên gia về OpenWrt và mạng máy tính.
Nhiệm vụ của bạn là phân tích dữ liệu Telemetry và báo cáo sự cố (Anomaly Report) từ Router GL.iNet Beryl 7.
Dựa trên thông tin được cung cấp, bạn PHẢI chọn ra 1 Công cụ (Tool / Function Call) duy nhất phù hợp nhất để điều khiển hoặc tối ưu mạng.

Các nguyên tắc hoạt động:
1. Nếu phát hiện sự cố rớt WAN / Mất Internet -> Gọi tool `restart_interface` với interface 'wan'.
2. Nếu phát hiện thiết bị gây nghẽn mạng / CPU cao bất thường -> Gọi tool `set_qos_priority` hoặc `block_device`.
3. Nếu phát hiện nhiễu Wi-Fi hoặc ngắt kết nối lặp lại -> Gọi tool `optimize_wifi_channel`.
4. Nếu mạng bình thường không có lỗi -> Gọi tool `no_action_required`.
"""

    def __init__(self, api_key=None):
        self.api_key = api_key or os.environ.get("GEMINI_API_KEY")
        if not self.api_key:
            raise ValueError("Không tìm thấy GEMINI_API_KEY. Vui lòng cung cấp API Key qua biến môi trường hoặc tham số!")
            
        self.client = genai.Client(api_key=self.api_key)
        self.model_name = "gemini-2.0-flash"

    def _get_tools_definition(self):
        """
        Định nghĩa danh sách các Tools / Function Calling mà AI được phép sử dụng.
        """
        return [
            types.FunctionDeclaration(
                name="set_qos_priority",
                description="Thiết lập độ ưu tiên băng thông QoS cho một thiết bị mạng cụ thể.",
                parameters=types.Schema(
                    type=types.Type.OBJECT,
                    properties={
                        "target_mac": types.Schema(type=types.Type.STRING, description="Địa chỉ MAC của thiết bị"),
                        "priority": types.Schema(type=types.Type.STRING, description="Mức ưu tiên: HIGH, MEDIUM, LOW"),
                        "max_bandwidth_mbps": types.Schema(type=types.Type.INTEGER, description="Băng thông tối đa (Mbps)"),
                        "reason": types.Schema(type=types.Type.STRING, description="Lý do đưa ra quyết định này")
                    },
                    required=["target_mac", "priority", "reason"]
                )
            ),
            types.FunctionDeclaration(
                name="block_device",
                description="Cách ly hoặc chặn một thiết bị khỏi mạng do nghi ngờ độc hại hoặc vi phạm.",
                parameters=types.Schema(
                    type=types.Type.OBJECT,
                    properties={
                        "target_mac": types.Schema(type=types.Type.STRING, description="Địa chỉ MAC cần chặn"),
                        "reason": types.Schema(type=types.Type.STRING, description="Lý do chặn thiết bị")
                    },
                    required=["target_mac", "reason"]
                )
            ),
            types.FunctionDeclaration(
                name="restart_interface",
                description="Khởi động lại một card/interface mạng (e.g., 'wan', 'br-lan', 'ra0') để sửa lỗi mất kết nối.",
                parameters=types.Schema(
                    type=types.Type.OBJECT,
                    properties={
                        "interface_name": types.Schema(type=types.Type.STRING, description="Tên interface cần restart (e.g. wan)"),
                        "reason": types.Schema(type=types.Type.STRING, description="Lý do restart interface")
                    },
                    required=["interface_name", "reason"]
                )
            ),
            types.FunctionDeclaration(
                name="optimize_wifi_channel",
                description="Chuyển đổi kênh Wi-Fi sang kênh tối ưu để giảm nhiễu sóng.",
                parameters=types.Schema(
                    type=types.Type.OBJECT,
                    properties={
                        "band": types.Schema(type=types.Type.STRING, description="Băng tần: 2.4G hoặc 5G"),
                        "channel": types.Schema(type=types.Type.INTEGER, description="Số kênh Wi-Fi mới"),
                        "reason": types.Schema(type=types.Type.STRING, description="Lý do đổi kênh")
                    },
                    required=["band", "channel", "reason"]
                )
            ),
            types.FunctionDeclaration(
                name="no_action_required",
                description="Báo cáo hệ thống ổn định, không cần thực thi hành động nào.",
                parameters=types.Schema(
                    type=types.Type.OBJECT,
                    properties={
                        "reason": types.Schema(type=types.Type.STRING, description="Lý do không cần hành động")
                    },
                    required=["reason"]
                )
            )
        ]

    def analyze_and_decide(self, telemetry_data, anomaly_data):
        """
        Gửi dữ liệu Telemetry + Anomaly đã qua Sanitizer tới Gemini AI API.
        Nhận lại Function Call Decision từ AI.
        """
        # Lọc thông tin nhạy cảm trước khi gửi
        clean_telemetry = DataSanitizer.sanitize_telemetry_dict(telemetry_data)
        clean_anomaly = DataSanitizer.sanitize_telemetry_dict(anomaly_data)

        prompt = f"""
Hãy phân tích tình trạng hệ thống Router Beryl 7 dưới đây và đưa ra quyết định xử lý phù hợp nhất bằng Function Calling:

--- [ ROUTER TELEMETRY ] ---
{json.dumps(clean_telemetry, indent=2, ensure_ascii=False)}

--- [ ANOMALY REPORT ] ---
{json.dumps(clean_anomaly, indent=2, ensure_ascii=False)}
"""

        tools = [types.Tool(function_declarations=self._get_tools_definition())]
        
        config = types.GenerateContentConfig(
            system_instruction=self.SYSTEM_INSTRUCTION,
            tools=tools,
            temperature=0.1 # Nhiệt độ thấp giúp AI ra quyết định logic, nhất quán
        )

        response = self.client.models.generate_content(
            model=self.model_name,
            contents=prompt,
            config=config
        )

        # Trích xuất Function Call từ response
        function_calls = response.function_calls
        if function_calls:
            call = function_calls[0]
            return {
                "status": "success",
                "action_type": "FUNCTION_CALL",
                "tool_name": call.name,
                "arguments": dict(call.args),
                "raw_thought": response.text if hasattr(response, 'text') else None
            }
        else:
            return {
                "status": "success",
                "action_type": "TEXT_RESPONSE",
                "response_text": response.text
            }
