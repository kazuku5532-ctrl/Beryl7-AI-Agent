import time
import json
from agent.telemetry import RouterTelemetry
from agent.log_parser import RouterLogParser
from agent.ai_client import RouterAIAgent
from agent.executor import RouterExecutor
from agent.watchdog import GuardedWatchdog
from agent.skill_store import SkillStore

class SelfEvolvingAgentOrchestrator:
    """
    Module Orchestrator trung tâm - Trái tim Đồ án:
    Điều phối Vòng lặp Tự sửa lỗi & Tự tiến hóa (Self-Healing & Self-Evolution Loop).
    Kết hợp Telemetry + Log Parser + SQLite Skill Store + Gemini AI + Executor + Watchdog Guardrail.
    """
    def __init__(self, hostname="192.168.8.1", port=22, username="root", password=None, key_filename=None, db_path="data/skills.db", dry_run=False):
        self.hostname = hostname
        self.port = port
        self.username = username
        self.password = password
        self.key_filename = key_filename
        self.dry_run = dry_run

        self.telemetry = RouterTelemetry(hostname, port, username, password, key_filename)
        self.log_parser = RouterLogParser(hostname, port, username, password, key_filename)
        self.executor = RouterExecutor(hostname, port, username, password, key_filename, dry_run=dry_run)
        self.watchdog = GuardedWatchdog(hostname, port, username, password, key_filename)
        self.skill_store = SkillStore(db_path=db_path)

    def run_self_healing_cycle(self, api_key=None, simulated_anomaly=None):
        """
        Thực thi 1 vòng lặp Self-Healing & Self-Evolution hoàn chỉnh.
        """
        print(f"\n================ [ BẮT ĐẦU VÒNG LẶP SELF-HEALING & SELF-EVOLUTION ] ================")
        
        # 1. Thu thập dữ liệu Telemetry & Anomaly
        print("[1/5] Thu thập Telemetry & Nhật ký hệ thống...")
        tele_data = self.telemetry.get_normalized_telemetry()
        anomaly_data = self.log_parser.detect_anomalies(telemetry_data=tele_data)

        # Hỗ trợ giả lập sự cố phục vụ test MVP
        if simulated_anomaly:
            print(f"⚠️ [SIMULATOR] Kích hoạt giả lập sự cố: '{simulated_anomaly.get('event_name')}'")
            anomaly_data["has_anomalies"] = True
            anomaly_data["max_severity"] = simulated_anomaly.get("severity", "CRITICAL")
            anomaly_data["anomalies"].append(simulated_anomaly)

        # 2. Kiểm tra nếu hệ thống bình thường
        if not anomaly_data.get("has_anomalies"):
            print("✅ Hệ thống mạng hoạt động bình thường! Không phát hiện sự cố.")
            return {
                "status": "healthy",
                "source": "NO_ACTION",
                "message": "Router ổn định."
            }

        # 3. Trích xuất sự cố hàng đầu (Top Anomaly)
        top_anomaly = anomaly_data["anomalies"][0]
        severity = top_anomaly.get("severity")
        category = top_anomaly.get("category")
        event_name = top_anomaly.get("event_name")
        
        signature = SkillStore.generate_signature(severity, category, event_name)
        print(f"🔍 [ANOMALY DETECTED] Sự cố: {severity}:{category}:{event_name} (Signature: {signature[:8]}...)")

        # 4. Kiểm tra SQLite Skill Store (Cache Lookup)
        print(f"[2/5] Truy vấn SQLite Skill Store tìm kỹ năng đã học...")
        learned_skill = self.skill_store.get_skill(signature, min_confidence=0.5)

        # ==================== TRƯỜNG HỢP 1: CACHE HIT (ĐÃ HỌC TỪ TRƯỚC) ====================
        if learned_skill:
            print(f"⚡ [CACHE HIT - SQLITE LOCAL] Tìm thấy Kỹ năng đã học! (Confidence: {learned_skill['confidence_score']}, Success: {learned_skill['success_count']})")
            print(f"  👉 Tool: '{learned_skill['tool_name']}' với tham số: {learned_skill['arguments']}")
            print(f"  💰 CHI PHÍ API: 0 VNĐ | THỜI GIAN PHẢN HỒI: ~0 GIÂY!")

            tool_name = learned_skill["tool_name"]
            args = learned_skill["arguments"]

            # Thực thi local qua Watchdog Guardrail
            exec_res = self.watchdog.execute_with_guardrail(
                executor_func=self.executor.dispatch_ai_decision,
                decision={
                    "status": "success",
                    "action_type": "FUNCTION_CALL",
                    "tool_name": tool_name,
                    "arguments": args
                },
                countdown_seconds=10
            )

            if exec_res.get("success"):
                # Cập nhật số lần thành công
                self.skill_store.save_or_update_skill(signature, category, event_name, tool_name, args)
                return {
                    "status": "success",
                    "source": "SQLITE_LOCAL_SKILL_STORE",
                    "api_cost_usd": 0.0,
                    "tool_used": tool_name,
                    "execution_result": exec_res
                }
            else:
                # Nếu chạy local thất bại -> Giảm điểm tin cậy để lần sau gọi AI sửa mới
                print("⚠️ [LOCAL FIX FAILED] Kỹ năng local thất bại! Giảm Confidence Score để AI tái huấn luyện...")
                self.skill_store.record_failure(signature)

        # ==================== TRƯỜNG HỢP 2: CACHE MISS (LỖI MỚI -> GỬI GEMINI AI) ====================
        print(f"🧠 [CACHE MISS - CLOUD AI] Lỗi mới chưa có trong tri thức! Gửi dữ liệu tới Gemini 2.0 Flash AI...")
        
        ai_agent = RouterAIAgent(api_key=api_key)
        ai_decision = ai_agent.analyze_and_decide(tele_data, anomaly_data)

        if ai_decision.get("action_type") != "FUNCTION_CALL":
            return {
                "status": "success",
                "source": "GEMINI_CLOUD_AI_TEXT",
                "response": ai_decision.get("response_text")
            }

        chosen_tool = ai_decision.get("tool_name")
        chosen_args = ai_decision.get("arguments", {})
        print(f"🎯 [GEMINI DECISION] AI đề xuất Tool: '{chosen_tool}' với tham số: {chosen_args}")

        # Thực thi kịch bản mới từ AI qua Watchdog Guardrail
        exec_res = self.watchdog.execute_with_guardrail(
            executor_func=self.executor.dispatch_ai_decision,
            decision=ai_decision,
            countdown_seconds=10
        )

        # 5. Tiến hóa & Lưu kỹ năng mới vào SQLite nếu thực thi thành công!
        if exec_res.get("success"):
            print(f"🚀 [SELF-EVOLUTION] Kịch bản sửa lỗi của AI THÀNH CÔNG! Lưu kỹ năng mới vào SQLite Skill Store...")
            self.skill_store.save_or_update_skill(
                error_signature=signature,
                category=category,
                event_name=event_name,
                tool_name=chosen_tool,
                arguments=chosen_args
            )
            return {
                "status": "success",
                "source": "GEMINI_CLOUD_AI_AND_EVOLVED",
                "evolved_new_skill": True,
                "tool_used": chosen_tool,
                "execution_result": exec_res
            }
        else:
            print("🚨 [EVOLUTION FAILED] Kịch bản AI thất bại và đã được Watchdog Rollback an toàn.")
            return {
                "status": "failed",
                "source": "GEMINI_CLOUD_AI",
                "evolved_new_skill": False,
                "execution_result": exec_res
            }
