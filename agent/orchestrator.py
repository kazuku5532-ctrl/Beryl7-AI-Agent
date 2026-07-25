import os
import json
import time
from agent.logger import agent_logger
from agent.telemetry import RouterTelemetry
from agent.log_parser import RouterLogParser
from agent.ai_client import RouterAIAgent
from agent.executor import RouterExecutor
from agent.watchdog import GuardedWatchdog
from agent.skill_store import SkillStore

class SelfEvolvingAgentOrchestrator:
    """
    Self-Evolving Agent Orchestrator: Bộ não trung tâm điều phối toàn bộ vòng lặp
    Tự phát hiện -> Tìm kiếm Skill -> Gọi Gemini AI -> Kiểm thử với Watchdog -> Tiến hóa SQLite.
    
    Cập nhật v3.5:
    - Sửa triệt để lỗi Defensive Error Handling khi anomalies rỗng.
    - Tích hợp Logging tập trung và Retry Logic.
    """
    def __init__(self, hostname=None, port=None, username=None, password=None, db_path="data/skills.db", dry_run=True):
        self.hostname = hostname or os.environ.get("ROUTER_HOST", "192.168.8.1")
        self.port = int(port or os.environ.get("ROUTER_PORT", 22))
        self.username = username or os.environ.get("ROUTER_USER", "root")
        self.password = password or os.environ.get("ROUTER_PASSWORD")
        self.db_path = db_path
        self.dry_run = dry_run

        self.telemetry = RouterTelemetry(self.hostname, self.port, self.username, self.password)
        self.log_parser = RouterLogParser(self.hostname, self.port, self.username, self.password)
        self.executor = RouterExecutor(self.hostname, self.port, self.username, self.password, dry_run=self.dry_run)
        self.watchdog = GuardedWatchdog(self.hostname, self.port, self.username, self.password)
        self.skill_store = SkillStore(self.db_path)

    def run_self_healing_cycle(self, api_key=None, simulated_anomaly=None):
        """
        Thực thi trọn vẹn 1 vòng lặp Self-Healing & Self-Evolution.
        """
        agent_logger.info("\n================ [ BẮT ĐẦU VÒNG LẶP SELF-HEALING & SELF-EVOLUTION ] ================")
        
        # 1. Thu thập Telemetry & Nhận diện Bất thường
        agent_logger.info("[1/5] Thu thập Telemetry & Nhật ký hệ thống...")
        telemetry_data = self.telemetry.get_normalized_telemetry()

        if simulated_anomaly:
            agent_logger.warning(f"⚠️ [SIMULATOR] Kích hoạt giả lập sự cố: '{simulated_anomaly.get('event_name')}'")
            anomaly_data = {
                "has_anomalies": True,
                "max_severity": simulated_anomaly.get("severity", "WARNING"),
                "anomalies": [simulated_anomaly]
            }
        else:
            anomaly_data = self.log_parser.detect_anomalies(telemetry_data)

        # Defensive Error Check: Đảm bảo không bị crash nếu anomalies rỗng
        anomalies_list = anomaly_data.get("anomalies", [])
        if not anomaly_data.get("has_anomalies") or not anomalies_list:
            agent_logger.info("✅ Hệ thống mạng hoạt động bình thường! Không phát hiện sự cố.")
            return {
                "status": "healthy",
                "message": "No anomalies detected.",
                "api_cost_usd": 0.0
            }

        top_anomaly = anomalies_list[0]
        severity = top_anomaly.get("severity", "WARNING")
        category = top_anomaly.get("category", "UNKNOWN")
        event_name = top_anomaly.get("event_name", "UNKNOWN_EVENT")

        error_signature = self.skill_store.generate_signature(severity, category, event_name)
        agent_logger.warning(f"🔍 [ANOMALY DETECTED] Sự cố: {severity}:{category}:{event_name} (Signature: {error_signature[:8]}...)")

        # 2. Kiểm tra Cache trong SQLite Skill Store
        agent_logger.info("[2/5] Truy vấn SQLite Skill Store tìm kỹ năng đã học...")
        learned_skill = self.skill_store.get_skill(error_signature, min_confidence=0.5)

        if learned_skill:
            tool_name = learned_skill["tool_name"]
            arguments = learned_skill["arguments"]
            agent_logger.info(f"⚡ [CACHE HIT - SQLITE LOCAL] Tìm thấy Kỹ năng đã học! (Confidence: {learned_skill['confidence_score']}, Success: {learned_skill['success_count']})")
            agent_logger.info(f"  👉 Tool: '{tool_name}' với tham số: {arguments}")
            agent_logger.info(f"  💰 CHI PHÍ API: 0 VNĐ | THỜI GIAN PHẢN HỒI: ~0 GIÂY!")
            
            # Thực thi lại lệnh đã học với Watchdog Guardrail
            exec_res = self.watchdog.execute_with_guardrail(self.executor.execute_tool, tool_name, arguments)
            
            if exec_res.get("success"):
                self.skill_store.save_or_update_skill(error_signature, category, event_name, tool_name, arguments)
            else:
                self.skill_store.record_failure(error_signature)

            return {
                "status": "success",
                "source": "SQLITE_LOCAL_SKILL_STORE",
                "api_cost_usd": 0.0,
                "tool_used": tool_name,
                "execution_result": exec_res
            }

        # 3. Cache Miss -> Gửi lên Cloud Gemini AI API
        agent_logger.info("🧠 [CACHE MISS] Chưa có kỹ năng trong SQLite local. Gửi yêu cầu tới Cloud Gemini AI API...")
        try:
            ai_agent = RouterAIAgent(api_key=api_key)
            ai_decision = ai_agent.analyze_and_decide(telemetry_data, anomaly_data)
        except Exception as e:
            agent_logger.error(f"❌ Lỗi khi gọi Gemini AI API: {e}")
            return {"status": "error", "message": f"Gemini API error: {e}"}

        if ai_decision.get("action_type") != "FUNCTION_CALL":
            agent_logger.info(f"💬 AI Phản hồi văn bản (Không yêu cầu hành động): {ai_decision.get('response_text')}")
            return {
                "status": "success",
                "source": "GEMINI_CLOUD_AI",
                "api_cost_usd": 0.0001,
                "tool_used": "none",
                "response_text": ai_decision.get("response_text")
            }

        tool_name = ai_decision["tool_name"]
        arguments = ai_decision["arguments"]
        agent_logger.info(f"🎯 AI Quyết định gọi Tool: '{tool_name}' với tham số: {arguments}")

        # 4. Kiểm thử với Watchdog Guardrail
        agent_logger.info("[4/5] Thực thi lệnh AI qua Watchdog Rollback Guardrail 30s...")
        exec_res = self.watchdog.execute_with_guardrail(self.executor.execute_tool, tool_name, arguments)

        # 5. Tiến hóa SQLite Skill Store nếu thành công
        if exec_res.get("success"):
            agent_logger.info("🌱 [EVOLUTION] Kiểm thử thành công! Lưu kỹ năng mới vào SQLite Skill Store...")
            self.skill_store.save_or_update_skill(error_signature, category, event_name, tool_name, arguments)
        else:
            agent_logger.error("❌ Kiểm thử thất bại hoặc bị Rollback! Không lưu kỹ năng này vào SQLite.")

        return {
            "status": "success",
            "source": "GEMINI_CLOUD_AI",
            "api_cost_usd": 0.0001,
            "tool_used": tool_name,
            "execution_result": exec_res
        }
