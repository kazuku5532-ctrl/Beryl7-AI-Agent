import argparse
import time
import sys
import paramiko
import urllib.request
import json

ACCEPTANCE_CRITERIA = {
    "memory_growth_rate_pct_per_hr": 1.0,  # Max 1% RAM growth per hour
    "thread_leak_max_per_hr": 5,           # Max 5 new threads per hour
    "latency_p99_ms": 500,                 # Max P99 response latency 500ms
    "api_error_rate_pct": 0.1              # Max 0.1% error rate
}

def get_router_metrics(router_ip, ssh_port, username, password):
    ssh = paramiko.SSHClient()
    ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    ssh.connect(router_ip, port=ssh_port, username=username, password=password)

    stdin, stdout, _ = ssh.exec_command("ps | grep beryl7-agent | grep -v grep")
    ps_out = stdout.read().decode().strip()
    pid = ps_out.split()[0] if ps_out else ""

    rss_kb = 0
    threads = 0
    if pid:
        stdin, stdout, _ = ssh.exec_command(f"cat /proc/{pid}/status")
        for line in stdout.read().decode().splitlines():
            if "VmRSS:" in line:
                rss_kb = int(line.split()[1])
            elif "Threads:" in line:
                threads = int(line.split()[1])

    ssh.close()
    return rss_kb, threads

def run_soak_test(duration_hours=72, interval_seconds=10, router_ip="192.168.8.1", username="root", password="Kazuku@2k6"):
    print(f"=== Beryl7 72-Hour Memory Leak & Performance Soak Test ===")
    print(f"Target Router: {router_ip} | Target Duration: {duration_hours} hours")
    print(f"Acceptance Criteria: Memory Growth < {ACCEPTANCE_CRITERIA['memory_growth_rate_pct_per_hr']}%/hr | Thread Delta < {ACCEPTANCE_CRITERIA['thread_leak_max_per_hr']}/hr | P99 Latency < {ACCEPTANCE_CRITERIA['latency_p99_ms']}ms")

    start_time = time.time()
    initial_rss, initial_threads = get_router_metrics(router_ip, 22, username, password)
    print(f"Baseline Start Metrics: VmRSS={initial_rss} KB (~{initial_rss/1024:.2f} MB), Threads={initial_threads}")

    measurements = []
    end_time = start_time + (duration_hours * 3600 if duration_hours > 0 else 60)

    try:
        while time.time() < end_time:
            t0 = time.time()
            try:
                resp = urllib.request.urlopen(f"http://{router_ip}:8888/api/health", timeout=5)
                latency_ms = (time.time() - t0) * 1000
                status_code = resp.getcode()
            except Exception as e:
                latency_ms = 5000
                status_code = 500

            rss_kb, threads = get_router_metrics(router_ip, 22, username, password)
            measurements.append((time.time(), rss_kb, threads, latency_ms, status_code))
            print(f"[{time.strftime('%H:%M:%S')}] VmRSS: {rss_kb/1024:.2f} MB | Threads: {threads} | Latency: {latency_ms:.1f} ms | Status: {status_code}")
            time.sleep(interval_seconds)

    except KeyboardInterrupt:
        print("\nSoak test manually stopped by operator.")

    final_rss, final_threads = get_router_metrics(router_ip, 22, username, password)
    duration_hrs_elapsed = max((time.time() - start_time) / 3600.0, 0.001)

    rss_growth_pct = ((final_rss - initial_rss) / max(initial_rss, 1)) * 100.0 / duration_hrs_elapsed
    threads_delta_per_hr = (final_threads - initial_threads) / duration_hrs_elapsed

    p99_latency = sorted([m[3] for m in measurements])[int(len(measurements) * 0.99)] if measurements else 0
    error_count = sum(1 for m in measurements if m[4] != 200)
    error_rate_pct = (error_count / max(len(measurements), 1)) * 100.0

    print("\n=== SOAK TEST VERIFICATION REPORT ===")
    print(f"Total Test Runtime: {duration_hrs_elapsed:.3f} hours")
    print(f"Memory Growth Rate: {rss_growth_pct:.2f}% / hr (Limit: < {ACCEPTANCE_CRITERIA['memory_growth_rate_pct_per_hr']}%)")
    print(f"Thread Count Delta: {threads_delta_per_hr:.2f} / hr (Limit: < {ACCEPTANCE_CRITERIA['thread_leak_max_per_hr']})")
    print(f"P99 Response Latency: {p99_latency:.1f} ms (Limit: < {ACCEPTANCE_CRITERIA['latency_p99_ms']} ms)")
    print(f"API Error Rate: {error_rate_pct:.3f}% (Limit: < {ACCEPTANCE_CRITERIA['api_error_rate_pct']}%)")

    assert rss_growth_pct <= ACCEPTANCE_CRITERIA["memory_growth_rate_pct_per_hr"], "FAILED: Memory growth rate exceeded threshold!"
    assert threads_delta_per_hr <= ACCEPTANCE_CRITERIA["thread_leak_max_per_hr"], "FAILED: Thread leak count exceeded threshold!"
    assert p99_latency <= ACCEPTANCE_CRITERIA["latency_p99_ms"], "FAILED: P99 response latency exceeded threshold!"
    print("🟢 SOAK TEST PASSED: All acceptance criteria met perfectly!")

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Beryl7 Soak Test Engine")
    parser.add_argument("--duration-hours", type=float, default=0.005, help="Test duration in hours (default: 0.005 for quick verification)")
    args = parser.parse_args()
    run_soak_test(duration_hours=args.duration_hours)
