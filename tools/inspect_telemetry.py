
import urllib.request
import json
import argparse
import sys

def main():
    parser = argparse.ArgumentParser(description="Inspect Beryl7 Telemetry")
    parser.add_argument("--url", default="http://127.0.0.1:8888/api/v1/metrics", help="Target URL (e.g. /api/v1/metrics or /api/health)")
    args = parser.parse_args()

    try:
        req = urllib.request.Request(args.url, method="GET")
        with urllib.request.urlopen(req, timeout=5) as resp:
            data = json.loads(resp.read().decode("utf-8"))
    except Exception as e:
        print(f"Error fetching data from {args.url}: {e}")
        sys.exit(1)

    print("="*60)
    print(f"Beryl7 Telemetry Dashboard ({args.url})")
    print("="*60)

    # If it is /api/v1/metrics
    if "process" in data:
        p = data["process"]
        rss = p.get("rss_mb", 0)
        heap = p.get("heap_alloc_mb", 0)
        goroutines = p.get("goroutines", 0)
        print("PROCESS STATS:")
        print(f"  RSS (MB):      {rss:.2f} " + (" [WARNING: >16MB]" if rss > 16 else ""))
        print(f"  Heap (MB):     {heap:.2f}")
        print(f"  Goroutines:    {goroutines}" + (" [WARNING: >25]" if goroutines > 25 else ""))
        print()

        if "telemetry" in data:
            t = data["telemetry"]
            print("HARDWARE TELEMETRY:")
            print(f"  CPU Usage:     {t.get('cpu_usage_pct', 0):.1f}%")
            print(f"  RAM Usage:     {t.get('ram_usage_pct', 0):.1f}%")
            print(f"  Temperature:   {t.get('hardware_temp_c', 0):.1f}°C")
            print()
            
        if "operational" in data:
            o = data["operational"]
            print("SKILLSTORE OPERATIONAL METRICS:")
            print(f"  Q-Updates:            {o.get('total_q_updates', 0)}")
            print(f"  Interpolations:       {o.get('interpolations', 0)}")
            print(f"  Exact Matches:        {o.get('exact_match_count', 0)}")
            print(f"  Fallback Defaults:    {o.get('fallback_default_count', 0)}")
            print(f"  Verified Successes:   {o.get('verified_successes', 0)}")
            print(f"  Verified Failures:    {o.get('verified_failures', 0)}")
            print()

        if "collector_counters" in data:
            cc = data["collector_counters"]
            print("TELEMETRY COUNTERS:")
            for k, v in cc.items():
                print(f"  {k}: {v}")
            print()

    # If it is /api/health
    elif "status" in data:
        print("HEALTH STATUS:")
        print(f"  Status:        {data.get('status', '')}")
        print(f"  Uptime:        {data.get('uptime_seconds', 0)}s")
        rss = data.get('rss_mb', 0)
        goroutines = data.get('goroutines', 0)
        print(f"  RSS (MB):      {rss:.2f} " + (" [WARNING: >16MB]" if rss > 16 else ""))
        print(f"  Goroutines:    {goroutines}" + (" [WARNING: >25]" if goroutines > 25 else ""))

if __name__ == "__main__":
    main()

