import os
import json
import sqlite3
import hashlib
from datetime import datetime

class SkillStore:
    """
    Module SQLite Skill Store: Bộ nhớ tri thức lưu trữ các kịch bản tự sửa lỗi (Skills)
    đã được kiểm chứng thành công. Giúp Router Agent tự tiến hóa và giải quyết sự cố 
    trùng lặp mà KHÔNG CẦN gọi lại AI API.
    
    Tích hợp +1 Evolution Step:
    Thêm chỉ số Confidence Score (Độ tin cậy 0.0 - 1.0) tự động tăng khi chạy tốt 
    và tự giảm khi thất bại để tự đào thải các kỹ năng lỗi thời.
    """
    def __init__(self, db_path="data/skills.db"):
        self.db_path = db_path
        os.makedirs(os.path.dirname(self.db_path), exist_ok=True)
        self._init_db()

    def _get_connection(self):
        return sqlite3.connect(self.db_path)

    def _init_db(self):
        """Khởi tạo bảng cơ sở dữ liệu skills"""
        with self._get_connection() as conn:
            cursor = conn.cursor()
            cursor.execute("""
                CREATE TABLE IF NOT EXISTS skills (
                    id INTEGER PRIMARY KEY AUTOINCREMENT,
                    error_signature TEXT UNIQUE NOT NULL,
                    category TEXT NOT NULL,
                    event_name TEXT NOT NULL,
                    tool_name TEXT NOT NULL,
                    arguments_json TEXT NOT NULL,
                    success_count INTEGER DEFAULT 1,
                    failure_count INTEGER DEFAULT 0,
                    confidence_score REAL DEFAULT 1.0,
                    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                    last_used_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
                )
            """)
            conn.commit()

    @staticmethod
    def generate_signature(severity, category, event_name):
        """Tạo chữ ký lỗi duy nhất (Error Signature)"""
        raw_key = f"{severity}:{category}:{event_name}".upper()
        return hashlib.md5(raw_key.encode('utf-8')).hexdigest()

    def get_skill(self, error_signature, min_confidence=0.5):
        """
        Truy vấn kỹ năng đã học theo Error Signature với ngưỡng Confidence tối thiểu.
        """
        with self._get_connection() as conn:
            cursor = conn.cursor()
            cursor.execute("""
                SELECT id, tool_name, arguments_json, success_count, confidence_score
                FROM skills
                WHERE error_signature = ? AND confidence_score >= ?
            """, (error_signature, min_confidence))
            
            row = cursor.fetchone()
            if row:
                skill_id, tool_name, args_json, success_count, confidence = row
                return {
                    "id": skill_id,
                    "error_signature": error_signature,
                    "tool_name": tool_name,
                    "arguments": json.loads(args_json),
                    "success_count": success_count,
                    "confidence_score": confidence
                }
        return None

    def save_or_update_skill(self, error_signature, category, event_name, tool_name, arguments):
        """
        Lưu kỹ năng mới hoặc cập nhật điểm số thành công cho kỹ năng cũ (Self-Evolution).
        """
        args_json = json.dumps(arguments, ensure_ascii=False)
        now_str = datetime.now().strftime("%Y-%m-%d %H:%M:%S")

        with self._get_connection() as conn:
            cursor = conn.cursor()
            # Kiểm tra xem kỹ năng đã tồn tại chưa
            cursor.execute("SELECT id, success_count, confidence_score FROM skills WHERE error_signature = ?", (error_signature,))
            row = cursor.fetchone()

            if row:
                skill_id, current_success, current_conf = row
                new_success = current_success + 1
                new_conf = min(1.0, current_conf + 0.1) # Tăng độ tin cậy khi thành công
                cursor.execute("""
                    UPDATE skills 
                    SET success_count = ?, confidence_score = ?, arguments_json = ?, last_used_at = ?
                    WHERE id = ?
                """, (new_success, new_conf, args_json, now_str, skill_id))
            else:
                cursor.execute("""
                    INSERT INTO skills (error_signature, category, event_name, tool_name, arguments_json, success_count, confidence_score, created_at, last_used_at)
                    VALUES (?, ?, ?, ?, ?, 1, 1.0, ?, ?)
                """, (error_signature, category, event_name, tool_name, args_json, now_str, now_str))

            conn.commit()

    def record_failure(self, error_signature):
        """Giảm điểm Confidence Score khi kỹ năng gặp thất bại để tự đào thải kỹ năng lỗi thời"""
        with self._get_connection() as conn:
            cursor = conn.cursor()
            cursor.execute("SELECT id, failure_count, confidence_score FROM skills WHERE error_signature = ?", (error_signature,))
            row = cursor.fetchone()

            if row:
                skill_id, current_fail, current_conf = row
                new_fail = current_fail + 1
                new_conf = max(0.0, current_conf - 0.4) # Giảm mạnh độ tin cậy khi xảy ra lỗi
                cursor.execute("""
                    UPDATE skills 
                    SET failure_count = ?, confidence_score = ?
                    WHERE id = ?
                """, (new_fail, new_conf, skill_id))
                conn.commit()

    def list_all_skills(self):
        """Liệt kê toàn bộ các kỹ năng đã học được trong SQLite"""
        with self._get_connection() as conn:
            cursor = conn.cursor()
            cursor.execute("SELECT id, error_signature, category, event_name, tool_name, success_count, confidence_score, last_used_at FROM skills ORDER BY last_used_at DESC")
            rows = cursor.fetchall()
            
            skills = []
            for r in rows:
                skills.append({
                    "id": r[0],
                    "signature": r[1],
                    "category": r[2],
                    "event_name": r[3],
                    "tool_name": r[4],
                    "success_count": r[5],
                    "confidence_score": r[6],
                    "last_used_at": r[7]
                })
            return skills
