import os
import json
import sqlite3
import hashlib
from datetime import datetime
from agent.logger import agent_logger

class SkillStore:
    """
    Module SQLite Skill Store: Bộ nhớ tri thức lưu trữ các kịch bản tự sửa lỗi (Skills)
    đã được kiểm chứng thành công.
    
    Cập nhật v3.5 (+1 Evolution Step):
    Thuật toán Trọng số Bình quân gia quyền (Exponential Moving Average - EMA)
    để điều chỉnh điểm tin cậy (Confidence Score 0.0 - 1.0) một cách mịn màng,
    tự động đào thải kỹ năng lỗi thời khi confidence_score < 0.5.
    """
    ALPHA_SUCCESS = 0.2  # Hệ số tăng trưởng khi thành công
    DECAY_FAILURE = 0.5   # Hệ số giảm điểm khi thất bại (Giảm 50% mỗi lần lỗi)

    def __init__(self, db_path="data/skills.db"):
        self.db_path = db_path
        self._shared_conn = None
        if self.db_path == ":memory:":
            self._shared_conn = sqlite3.connect(":memory:")
        else:
            dir_name = os.path.dirname(self.db_path)
            if dir_name:
                os.makedirs(dir_name, exist_ok=True)
        self._init_db()

    def _get_connection(self):
        if self._shared_conn:
            return self._shared_conn
        return sqlite3.connect(self.db_path)

    def _init_db(self):
        """Khởi tạo bảng cơ sở dữ liệu skills và tự động Migration nếu cần"""
        conn = self._get_connection()
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
        if not self._shared_conn:
            conn.close()

    @staticmethod
    def generate_signature(severity, category, event_name):
        """Tạo chữ ký lỗi duy nhất (Error Signature)"""
        raw_key = f"{severity}:{category}:{event_name}".upper()
        return hashlib.md5(raw_key.encode('utf-8')).hexdigest()

    def get_skill(self, error_signature, min_confidence=0.5):
        """
        Truy vấn kỹ năng đã học theo Error Signature với ngưỡng Confidence tối thiểu.
        """
        conn = self._get_connection()
        cursor = conn.cursor()
        cursor.execute("""
            SELECT id, tool_name, arguments_json, success_count, confidence_score
            FROM skills
            WHERE error_signature = ? AND confidence_score >= ?
        """, (error_signature, min_confidence))
        
        row = cursor.fetchone()
        if not self._shared_conn:
            conn.close()

        if row:
            skill_id, tool_name, args_json, success_count, confidence = row
            return {
                "id": skill_id,
                "error_signature": error_signature,
                "tool_name": tool_name,
                "arguments": json.loads(args_json),
                "success_count": success_count,
                "confidence_score": round(confidence, 3)
            }
        return None

    def save_or_update_skill(self, error_signature, category, event_name, tool_name, arguments):
        """
        Lưu kỹ năng mới hoặc cập nhật điểm số EMA thành công cho kỹ năng cũ (Self-Evolution).
        """
        args_json = json.dumps(arguments, ensure_ascii=False)
        now_str = datetime.now().strftime("%Y-%m-%d %H:%M:%S")

        conn = self._get_connection()
        cursor = conn.cursor()
        cursor.execute("SELECT id, success_count, confidence_score FROM skills WHERE error_signature = ?", (error_signature,))
        row = cursor.fetchone()

        if row:
            skill_id, current_success, current_conf = row
            new_success = current_success + 1
            # Công thức EMA tiệm cận 1.0: new_conf = current + alpha * (1.0 - current)
            new_conf = min(1.0, current_conf + self.ALPHA_SUCCESS * (1.0 - current_conf))
            cursor.execute("""
                UPDATE skills 
                SET success_count = ?, confidence_score = ?, arguments_json = ?, last_used_at = ?
                WHERE id = ?
            """, (new_success, new_conf, args_json, now_str, skill_id))
            agent_logger.info(f"🧠 [SKILL STORE] Cập nhật kỹ năng '{event_name}': Success={new_success}, Confidence={round(new_conf, 3)}")
        else:
            cursor.execute("""
                INSERT INTO skills (error_signature, category, event_name, tool_name, arguments_json, success_count, confidence_score, created_at, last_used_at)
                VALUES (?, ?, ?, ?, ?, 1, 1.0, ?, ?)
            """, (error_signature, category, event_name, tool_name, args_json, now_str, now_str))
            agent_logger.info(f"🚀 [SKILL STORE] Học và lưu Kỹ năng mới '{event_name}' vào SQLite Skill Store!")

        conn.commit()
        if not self._shared_conn:
            conn.close()

    def record_failure(self, error_signature):
        """Giảm điểm Confidence Score bằng tỷ lệ suy giảm (Decay) để tự đào thải kỹ năng lỗi thời"""
        conn = self._get_connection()
        cursor = conn.cursor()
        cursor.execute("SELECT id, failure_count, confidence_score FROM skills WHERE error_signature = ?", (error_signature,))
        row = cursor.fetchone()

        if row:
            skill_id, current_fail, current_conf = row
            new_fail = current_fail + 1
            new_conf = max(0.0, current_conf * self.DECAY_FAILURE) # Giảm 50% điểm tin cậy
            cursor.execute("""
                UPDATE skills 
                SET failure_count = ?, confidence_score = ?
                WHERE id = ?
            """, (new_fail, new_conf, skill_id))
            conn.commit()
            agent_logger.warning(f"⚠️ [SKILL STORE] Kỹ năng thất bại! Confidence giảm từ {round(current_conf, 3)} xuống {round(new_conf, 3)}")

        if not self._shared_conn:
            conn.close()

    def list_all_skills(self):
        """Liệt kê toàn bộ các kỹ năng đã học được trong SQLite"""
        conn = self._get_connection()
        cursor = conn.cursor()
        cursor.execute("SELECT id, error_signature, category, event_name, tool_name, success_count, confidence_score, last_used_at FROM skills ORDER BY last_used_at DESC")
        rows = cursor.fetchall()
        if not self._shared_conn:
            conn.close()
        
        skills = []
        for r in rows:
            skills.append({
                "id": r[0],
                "signature": r[1],
                "category": r[2],
                "event_name": r[3],
                "tool_name": r[4],
                "success_count": r[5],
                "confidence_score": round(r[6], 3),
                "last_used_at": r[7]
            })
        return skills

    def close(self):
        if self._shared_conn:
            self._shared_conn.close()
            self._shared_conn = None
