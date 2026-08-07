import urllib.request
import concurrent.futures
import time

TARGET_URL = "http://localhost:5000/api/health"
CONCURRENT_CLIENTS = 100
TOTAL_REQUESTS = 500

def fetch_health():
    start = time.time()
    try:
        req = urllib.request.urlopen(TARGET_URL, timeout=2.0)  # nosec B310
        code = req.getcode()
        duration = (time.time() - start) * 1000
        return code == 200, duration
    except Exception as e:  # nosec B110
        return False, (time.time() - start) * 1000

def run_stress_test():
    print(f"[Stress Test] Starting concurrency load test: {CONCURRENT_CLIENTS} clients, {TOTAL_REQUESTS} total requests...")
    start_total = time.time()
    success_count = 0
    latencies = []

    with concurrent.futures.ThreadPoolExecutor(max_workers=CONCURRENT_CLIENTS) as executor:
        futures = [executor.submit(fetch_health) for _ in range(TOTAL_REQUESTS)]
        for future in concurrent.futures.as_completed(futures):
            ok, dur = future.result()
            if ok:
                success_count += 1
                latencies.append(dur)

    total_time = time.time() - start_total
    rps = TOTAL_REQUESTS / total_time
    latencies.sort()
    avg_lat = sum(latencies) / len(latencies) if latencies else 0
    p95_lat = latencies[int(len(latencies) * 0.95)] if latencies else 0

    print(f"==================================================")
    print(f" Load Test Completed in {total_time:.2f}s!")
    print(f" Success Rate: {success_count}/{TOTAL_REQUESTS} ({success_count/TOTAL_REQUESTS*100:.1f}%)")
    print(f" Throughput: {rps:.1f} req/sec")
    print(f" Average Latency: {avg_lat:.2f} ms")
    print(f" P95 Latency: {p95_lat:.2f} ms")
    print(f"==================================================")

if __name__ == "__main__":
    run_stress_test()
