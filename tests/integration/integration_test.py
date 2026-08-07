import urllib.request
import json

ENDPOINTS = [
    "http://localhost:5000/api/health",
    "http://localhost:5000/api/modules/status",
    "http://localhost:5000/api/logs",
    "http://localhost:5000/api/metrics/history",
    "http://localhost:5000/api/cache/stats",
    "http://localhost:5000/api/system/info"
]

def run_integration_test():
    print("[Integration Test] Testing REST API endpoints...")
    passed = 0
    for url in ENDPOINTS:
        if not (url.startswith("http://") or url.startswith("https://")):
            raise ValueError(f"Disallowed URL scheme in test endpoint: {url}")
        try:
            req = urllib.request.urlopen(url)
            if req.getcode() == 200:
                print(f"  [PASS] {url}")
                passed += 1
            else:
                print(f"  [FAIL] {url} status {req.getcode()}")
        except (urllib.error.URLError, OSError) as e:
            print(f"  [WARN] {url} offline ({e})")

    print(f"Integration Test Result: {passed}/{len(ENDPOINTS)} endpoints responding OK")

if __name__ == "__main__":
    run_integration_test()
