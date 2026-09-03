#!/usr/bin/env python3
"""
tests/chaos/live_chaos_test.py
Active Chaos Engineering & Empirical Q-Learning Verification Harness.

RULES & INTEGRITY GUARANTEES:
1. ZERO SQL WRITE STATEMENTS: This script NEVER modifies SQLite directly. All learning and
   database updates MUST be executed autonomously by the compiled Go agent (main.go).
2. REAL HTTP & SYSTEM INJECTION: Sends live requests to the Go agent's REST API (/api/chaos/inject).
3. READ-ONLY VERIFICATION: Uses inspect_skillstore.py in READ-ONLY mode to assert that the Go
   agent's 3-second post-action verification loop actually updated Q-values and skill confidence.
"""

import sys
import os
import time
import json
import gc
import urllib.request
import urllib.error

# Add tools directory to path
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), "..", "..", "tools")))
try:
    import inspect_skillstore
except ImportError:
    inspect_skillstore = None

if sys.platform == "win32":
    sys.stdout.reconfigure(encoding='utf-8', errors='replace')

def check_agent_health(endpoint="http://127.0.0.1:8888"):
    """Checks if the Go agent daemon is running and reachable."""
    try:
        url = f"{endpoint}/api/health"
        req = urllib.request.Request(url, headers={"User-Agent": "AeroSON-LiveChaos/2.0"})
        with urllib.request.urlopen(req, timeout=3) as resp:  # nosec B310
            if resp.status == 200:
                data = json.loads(resp.read().decode('utf-8'))
                return True, data
    except Exception as e:
        return False, str(e)
    return False, "Offline"

def inject_chaos_via_api(endpoint="http://127.0.0.1:8888", anomaly="MEMORY_EXHAUSTION", action="purge_memory_cache"):
    """Sends a real HTTP POST request to trigger closed-loop remediation in the Go agent."""
    url = f"{endpoint}/api/chaos/inject"
    payload = json.dumps({"anomaly": anomaly, "action": action}).encode('utf-8')
    req = urllib.request.Request(
        url,
        data=payload,
        headers={"Content-Type": "application/json", "User-Agent": "AeroSON-LiveChaos/2.0"},
        method="POST"
    )
    with urllib.request.urlopen(req, timeout=5) as resp:  # nosec B310
        if resp.status == 200:
            return json.loads(resp.read().decode('utf-8'))
        raise RuntimeError(f"API returned non-200 status: {resp.status}")

def run_live_chaos_test(db_path=None, endpoint="http://127.0.0.1:8888"):
    print("==================================================================")
    print("🧪 Beryl 7 / AeroSON Live Chaos & Empirical Learning Test")
    print(f"🎯 Target Endpoint: {endpoint}")
    print("==================================================================")

    # 1. Resolve Active SQLite Database Path (Read-Only)
    if not db_path:
        if os.path.exists("/tmp/skills.db"):  # nosec B108
            db_path = "/tmp/skills.db"  # nosec B108
        else:
            db_path = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", "..", "go-agent", "skills.db"))

    print(f"📂 Read-Only Database Target: {db_path}")

    # 2. Verify Go Agent is Online
    is_live, health_info = check_agent_health(endpoint)
    if not is_live:
        print(f"\n❌ ERROR: Go Agent is NOT running at {endpoint} ({health_info}).")
        print("💡 The Go daemon must be active so it can execute real anomaly detection,")
        print("   real remediation, and real SQLite TD-updates.")
        print("\n👉 To start the agent: ./beryl7-agent or (cd go-agent && go run ./cmd)")
        sys.exit(1)

    print(f"✅ Connected to Active Go Agent Daemon:")
    print(f"   • Active Profile: {health_info.get('active_profile')}")
    print(f"   • Active Driver:  {health_info.get('active_driver')}")
    print(f"   • Uptime:         {health_info.get('uptime_seconds')}s")
    print(f"   • Telemetry Token:{health_info.get('network_token')}")

    # 3. Take PRE-CHAOS Snapshot via inspect_skillstore (READ-ONLY)
    print("\n------------------------------------------------------------------")
    print("📸 [STEP 1/4] Recording Pre-Chaos Baseline State from SQLite (Read-Only)")
    print("------------------------------------------------------------------")
    snap_before = inspect_skillstore.get_db_snapshot(db_path) if inspect_skillstore else None
    
    q_key = ("MEMORY_EXHAUSTION", "purge_memory_cache")
    q_before = 0.60
    conf_before = 0.60
    succ_before = 0
    if snap_before and q_key in snap_before["q_table"]:
        q_before = snap_before["q_table"][q_key]["q_value"]
    if snap_before and q_key in snap_before["skills"]:
        conf_before = snap_before["skills"][q_key]["confidence"]
        succ_before = snap_before["skills"][q_key]["success_count"]

    print(f"📊 Baseline Q-Learning State before Chaos:")
    print(f"   • Q-Value(MEMORY_EXHAUSTION, purge_memory_cache) = {q_before:.4f}")
    print(f"   • Skills Confidence = {conf_before:.4f}, Hits = {succ_before}")

    # 4. Inject Real RAM Memory Stress & Call Real Go Agent API
    print("\n------------------------------------------------------------------")
    print("⚡ [STEP 2/4] Triggering Real Memory Stress & Live API Chaos Injection")
    print("------------------------------------------------------------------")
    
    # Real memory allocation to simulate RAM pressure
    ram_buffer = []
    try:
        print(" • Allocating 128 MB real RAM bytearray buffer...")
        for _ in range(128):
            ram_buffer.append(bytearray(1024 * 1024))
        print(" • Real RAM buffer active (simulating memory pressure spike).")

        # Call real Go Agent REST API
        print(f" • Sending POST {endpoint}/api/chaos/inject ...")
        api_resp = inject_chaos_via_api(endpoint, anomaly="MEMORY_EXHAUSTION", action="purge_memory_cache")
        print(f" • Go Agent Response: {api_resp.get('status')} (Execution: {api_resp.get('execution_success')})")
        print(f" • Go Agent Message:  {api_resp.get('message')}")

    finally:
        # Free memory buffer
        del ram_buffer[:]
        gc.collect()
        print(" • Real RAM buffer released.")

    # 5. Wait for the Go Agent's 3-Second Async Verification Goroutine
    print("\n------------------------------------------------------------------")
    print("⏳ [STEP 3/4] Awaiting Go Agent 3.0s Autonomous Verification & Learning Loop")
    print("------------------------------------------------------------------")
    wait_sec = 4.2
    print(f" • Waiting {wait_sec}s for Go goroutine to complete verifyActionSuccess() and write to SQLite...")
    time.sleep(wait_sec)

    # 6. Take POST-CHAOS Snapshot via inspect_skillstore (READ-ONLY) & Assert Delta Shift
    print("\n------------------------------------------------------------------")
    print("📈 [STEP 4/4] Reading Post-Chaos State & Asserting Autonomous Learning")
    print("------------------------------------------------------------------")
    snap_after = inspect_skillstore.get_db_snapshot(db_path) if inspect_skillstore else None
    if not snap_after:
        raise RuntimeError(f"❌ Failed to read SQLite database at {db_path}")

    q_after = snap_after["q_table"].get(q_key, {}).get("q_value", q_before)
    conf_after = snap_after["skills"].get(q_key, {}).get("confidence", conf_before)
    succ_after = snap_after["skills"].get(q_key, {}).get("success_count", succ_before)

    delta_q = q_after - q_before
    delta_conf = conf_after - conf_before

    print(f"📊 Post-Chaos Verification Results (Written by Go Agent):")
    print(f"   • Q-Value:    {q_before:.4f} -> {q_after:.4f} (Delta: {delta_q:+.4f})")
    print(f"   • Confidence: {conf_before:.4f} -> {conf_after:.4f} (Delta: {delta_conf:+.4f})")
    print(f"   • Hits:       {succ_before} -> {succ_after} (+{succ_after - succ_before} Hit)")

    # STRICT MATHEMATICAL ASSERTION: The Go Agent MUST have updated the values
    if delta_q <= 0.0001:
        raise AssertionError(
            f"❌ Q-Value did NOT shift! Expected delta > 0, got delta_q={delta_q:.6f}. "
            f"The Go agent failed to learn from the live anomaly."
        )
    if succ_after <= succ_before:
        raise AssertionError(
            f"❌ Success count did NOT increment! succ_before={succ_before}, succ_after={succ_after}. "
            f"The Go agent verification loop did not record outcome."
        )

    print("\n==================================================================")
    print("🏆 SUCCESS: Live Go Agent Closed-Loop Learning EMPIRICALLY CONFIRMED!")
    print("   • Real HTTP Anomaly Triggered -> Go Agent Closed-Loop Executed")
    print("   • 3-Second Telemetry Measured -> Go Agent Wrote to SQLite")
    print("   • Zero Fake Test Injections   -> 100% Autonomous Reinforcement Learning")
    print("==================================================================")

if __name__ == "__main__":
    target_db = sys.argv[1] if len(sys.argv) > 1 else None
    target_endpoint = sys.argv[2] if len(sys.argv) > 2 else "http://127.0.0.1:8888"
    run_live_chaos_test(target_db, target_endpoint)
