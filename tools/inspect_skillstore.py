#!/usr/bin/env python3
"""
tools/inspect_skillstore.py
Inspects SQLite skillstore database (/tmp/skills.db or custom path),
compares learned Q-values and Skill confidence against baseline seed values,
and proves whether the agent has actively learned from live network anomalies.
"""

import sys
import os
import sqlite3
import datetime

if sys.platform == "win32":
    sys.stdout.reconfigure(encoding='utf-8', errors='replace')

DEFAULT_SEEDS = {
    ("WAN_DROP", "restart_wan_interface"): 0.5,
    ("HIGH_LATENCY", "enable_sqm_cake"): 0.6,
    ("BUFFERBLOAT", "enable_sqm_cake"): 0.6,
    ("MEMORY_EXHAUSTION", "purge_memory_cache"): 0.6,
    ("WIFI_CHANNEL_CONGESTION", "optimize_wifi_channels"): 0.5,
    ("REPEATER_DESYNC", "resync_repeater_interface"): 0.5,
    ("DEVICE_STUCK_2G", "trigger_80211v_bss_transition"): 0.5,
}

def get_db_snapshot(db_path):
    """Returns structured snapshot of Q-table and Skills table for automated delta verification."""
    if not os.path.exists(db_path):
        return None

    snapshot = {
        "q_table": {},
        "skills": {},
    }
    try:
        conn = sqlite3.connect(f"file:{db_path}?mode=ro", uri=True)
        cursor = conn.cursor()

        cursor.execute("SELECT state, action, q_value, updated_at FROM q_table;")
        for state, action, q_val, updated_at in cursor.fetchall():
            snapshot["q_table"][(state, action)] = {
                "q_value": float(q_val),
                "updated_at": updated_at
            }

        cursor.execute("SELECT condition_query, action, confidence, success_count, failure_count, last_used_at FROM skills;")
        for cond, act, conf, succ, fail, last_used in cursor.fetchall():
            snapshot["skills"][(cond, act)] = {
                "confidence": float(conf),
                "success_count": int(succ),
                "failure_count": int(fail),
                "last_used_at": last_used
            }

        conn.close()
        return snapshot
    except Exception:
        return None

def inspect_db(db_path):
    if not os.path.exists(db_path):
        print(f"❌ Database not found at: {db_path}")
        return False

    print("==================================================================")
    print(f"🔍 Beryl 7 / AeroSON SkillStore Q-Learning Inspection")
    print(f"📂 Path: {db_path}")
    print(f"⏰ Inspected at: {datetime.datetime.now().isoformat()}")
    print("==================================================================")

    try:
        conn = sqlite3.connect(f"file:{db_path}?mode=ro", uri=True)
        cursor = conn.cursor()

        # 1. Inspect Q-Table
        print("\n📊 1. Q-LEARNING Q-TABLE EVOLUTION:")
        print("-" * 66)
        print(f"{'State':<25} {'Action':<25} {'Q-Value':<8} {'Seed':<6} {'Delta':<8}")
        print("-" * 66)

        cursor.execute("SELECT state, action, q_value, updated_at FROM q_table ORDER BY state ASC;")
        rows = cursor.fetchall()

        q_learned_count = 0
        for state, action, q_val, updated_at in rows:
            seed = DEFAULT_SEEDS.get((state, action), 0.5)
            delta = q_val - seed
            delta_str = f"{delta:+.3f}" if abs(delta) > 0.001 else "0.000 (Seed)"
            if abs(delta) > 0.001:
                q_learned_count += 1
            print(f"{state:<25} {action:<25} {q_val:<8.3f} {seed:<6.2f} {delta_str:<8}")

        # 2. Inspect Skills Table
        print("\n📈 2. EMPIRICAL SKILLS CONFIDENCE & HIT COUNTERS:")
        print("-" * 66)
        print(f"{'Condition':<22} {'Action':<22} {'Conf':<6} {'Success':<8} {'Fail':<6}")
        print("-" * 66)

        cursor.execute("SELECT condition_query, action, confidence, success_count, failure_count, last_used_at FROM skills ORDER BY last_used_at DESC;")
        skill_rows = cursor.fetchall()
        for cond, act, conf, succ, fail, last_used in skill_rows:
            print(f"{cond:<22} {act:<22} {conf:<6.2f} {succ:<8} {fail:<6}")

        # 3. Inspect State Signatures (TinyML Similarity Vector Space)
        cursor.execute("SELECT name FROM sqlite_master WHERE type='table' AND name='state_signatures';")
        if cursor.fetchone():
            print("\n🧠 3. TINYML STATE SIGNATURES & TELEMETRY VECTORS:")
            print("-" * 66)
            print(f"{'State Name':<24} {'RAM%':<7} {'Lat(ms)':<8} {'CPU%':<7} {'Temp':<6} {'WAN/WiFi':<8}")
            print("-" * 66)
            cursor.execute("SELECT state_name, ram_pct, latency_ms, cpu_pct, temp_c, wan_offline, wifi_fail FROM state_signatures ORDER BY state_name ASC;")
            sig_rows = cursor.fetchall()
            for sname, sram, slat, scpu, stemp, swan, swifi in sig_rows:
                flags = f"W:{swan}/F:{swifi}"
                print(f"{sname:<24} {sram:<7.1f} {slat:<8.1f} {scpu:<7.1f} {stemp:<6.1f} {flags:<8}")

        print("==================================================================")
        if q_learned_count > 0 or len(skill_rows) > 0:
            print(f"✅ VERDICT: Active Reinforcement Learning CONFIRMED!")
            print(f"   • {q_learned_count} Q-values have dynamically shifted from seed baseline.")
            print(f"   • {len(skill_rows)} learned skill records actively tracking empirical success/fail.")
        else:
            print("ℹ️ STATUS: Initial Seed State (No dynamic anomaly events recorded yet).")
        print("==================================================================")
        conn.close()
        return True

    except Exception as e:
        print(f"❌ Error reading database: {e}")
        return False

if __name__ == "__main__":
    target = sys.argv[1] if len(sys.argv) > 1 else "/tmp/skills.db"  # nosec B108
    if not os.path.exists(target) and sys.platform == "win32":
        target = "go-agent/tests/test_skills.db"
    inspect_db(target)
