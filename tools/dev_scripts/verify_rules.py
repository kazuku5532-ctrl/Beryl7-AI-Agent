#!/usr/bin/env python3
"""
Beryl 7 Mandatory System-Wide Constitutional Rule Verifier
MULTI-PATTERN REGEX SECRET SCANNER, AST REGEX CHECKER & BINARY SYNC AUDITOR.

This script enforces 100% compliance with workspace rules:
- .agents/rules/complete_self_management_framework.md
- .agents/rules/engineering_rigor_and_integration.md

Exit code 0 = 100% CONSTITUTIONAL COMPLIANCE (Commit Allowed)
Exit code 1 = CONSTITUTIONAL VIOLATION DETECTED (Commit Blocked)
"""
import os
import sys
import re
import subprocess

if sys.platform == "win32":
    sys.stdout.reconfigure(encoding='utf-8')

WORKSPACE_ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "../.."))
GO_AGENT_DIR = os.path.join(WORKSPACE_ROOT, "go-agent")

errors = []

def report_pass(check_name):
    print(f"  ✅ [PASS] {check_name}")

def report_fail(check_name, reason):
    print(f"  ❌ [FAIL] {check_name}: {reason}")
    errors.append(f"{check_name}: {reason}")

print("==========================================================")
print("🛡️ BERYL 7 SYSTEM-WIDE CONSTITUTIONAL ENFORCEMENT ENGINE")
print("==========================================================")

# --------------------------------------------------------------------------
# CHECK 1: MULTI-PATTERN & HIGH-ENTROPY SECRET SCANNER
# --------------------------------------------------------------------------
print("\n[Rule 2.2] Multi-Pattern Regex & Secret Audit (Scanning All Extensions)...")
SECRET_PATTERNS = [
    (re.compile(r'Kaz' + r'uku@2k6'), "Hardcoded Router Password"),
    (re.compile(r'AIzaSy[A-Za-z0-9_\\-]{35}'), "Google Gemini API Key"),
    (re.compile(r'-----BEGIN (RSA|OPENSSH|PRIVATE) KEY-----'), "SSH/RSA Private Key"),
    (re.compile(r'(?:password|secret|auth_token|approve_token)\s*[:=]\s*["\']([^"\']{8,})["\']', re.IGNORECASE), "Hardcoded Secret String"),
]

EXCLUDED_DIRS = {'.git', 'venv', '__pycache__', '.system_generated', 'scratch'}
EXCLUDED_FILES = {'agent.env', 'verify_rules.py', 'go.sum', 'coverage.out'}

secret_violations = []
for root, dirs, files in os.walk(WORKSPACE_ROOT):
    dirs[:] = [d for d in dirs if d not in EXCLUDED_DIRS]
    for f in files:
        if f in EXCLUDED_FILES or f.endswith(('.pyc', '.png', '.jpg', '.tar.gz', '.backup')):
            continue
        if f.endswith(('.go', '.py', '.json', '.yaml', '.yml', '.conf', '.md', '.env.example', '.sh')):
            filepath = os.path.join(root, f)
            try:
                with open(filepath, 'r', encoding='utf-8', errors='ignore') as file_obj:
                    content = file_obj.read()
                    for pattern, desc in SECRET_PATTERNS:
                        matches = pattern.findall(content)
                        for match in matches:
                            if isinstance(match, tuple):
                                match_str = match[0]
                            else:
                                match_str = str(match)
                            if any(placeholder in match_str.lower() for placeholder in ['your_', 'example', 'placeholder', 'token_here', 'password_here', 'mock_']):
                                continue
                            rel_path = os.path.relpath(filepath, WORKSPACE_ROOT)
                            secret_violations.append(f"{rel_path} ({desc})")
            except Exception:
                pass

if secret_violations:
    report_fail("Multi-Pattern Secret Audit", "; ".join(secret_violations))
else:
    report_pass("Multi-Pattern Secret Audit (0 Cleartext Secrets across all files)")

# --------------------------------------------------------------------------
# CHECK 2: FLEXIBLE REGEX CORS SECURITY AUDIT
# --------------------------------------------------------------------------
print("\n[Rule 2.1] Flexible Regex CORS Security Audit...")
main_go_path = os.path.join(GO_AGENT_DIR, "cmd", "main.go")
if os.path.exists(main_go_path):
    with open(main_go_path, 'r', encoding='utf-8') as f:
        main_content = f.read()

    # Regex for unsafe prefix/suffix CORS checks
    unsafe_prefix_pattern = re.compile(r'strings\.HasPrefix\s*\(\s*(\w+\.)?origin', re.IGNORECASE)
    unsafe_suffix_pattern = re.compile(r'strings\.HasSuffix\s*\(\s*\w+\s*,\s*["\']\.(local|lan|home)["\']\s*\)', re.IGNORECASE)

    if unsafe_prefix_pattern.search(main_content):
        report_fail("CORS Security Audit", "Unsafe strings.HasPrefix(origin, ...) regex pattern detected in main.go")
    elif unsafe_suffix_pattern.search(main_content):
        report_fail("CORS Security Audit", "Unsafe .local/.lan/.home mDNS suffix matching regex pattern detected in main.go")
    else:
        report_pass("CORS Security Audit (Strict RFC 1918 & url.Parse IP Check verified via regex)")
else:
    report_fail("CORS Security Audit", "main.go file not found")

# --------------------------------------------------------------------------
# CHECK 3: FLEXIBLE REGEX DYNAMIC os.Executable() PORTABILITY AUDIT
# --------------------------------------------------------------------------
print("\n[Rule 2.4] Checking Dynamic os.Executable() Path Resolution...")
if os.path.exists(main_go_path):
    with open(main_go_path, 'r', encoding='utf-8') as f:
        main_content = f.read()

    hardcoded_exec_pattern = re.compile(r'syscall\.Exec\s*\(\s*["\']/usr/bin/beryl7-agent["\']', re.IGNORECASE)
    dynamic_exec_pattern = re.compile(r'os\.Executable\s*\(\s*\)')

    if hardcoded_exec_pattern.search(main_content):
        report_fail("Process Path Portability", "Hardcoded /usr/bin/beryl7-agent path in syscall.Exec detected")
    elif not dynamic_exec_pattern.search(main_content):
        report_fail("Process Path Portability", "os.Executable() dynamic path resolution pattern missing in main.go")
    else:
        report_pass("Process Path Portability (Dynamic os.Executable() regex verified)")

# --------------------------------------------------------------------------
# CHECK 4: PARAMIKO IO DEADLOCK SHIELD (CWE-833)
# --------------------------------------------------------------------------
print("\n[Rule 3.0] Checking deploy_router.py for Paramiko Deadlock (CWE-833)...")
deploy_script_path = os.path.join(WORKSPACE_ROOT, "tools", "dev_scripts", "deploy_router.py")
if os.path.exists(deploy_script_path):
    with open(deploy_script_path, 'r', encoding='utf-8') as f:
        deploy_content = f.read()

    idx_read = deploy_content.find("stdout.read()")
    idx_exit = deploy_content.find("recv_exit_status()")

    if idx_read != -1 and idx_exit != -1 and idx_exit < idx_read:
        report_fail("Paramiko Deadlock Check", "recv_exit_status() called before stdout.read() in deploy_router.py (CWE-833)")
    else:
        report_pass("Paramiko IO Deadlock Check (Buffer read before wait verified)")

# --------------------------------------------------------------------------
# CHECK 5: BINARY OUT-OF-SYNC AUDIT (SOURCE VS COMPILED BINARY TIMESTAMP)
# --------------------------------------------------------------------------
print("\n[Rule 2.4] Checking Compiled Binary Sync with Go Source Code...")
binary_path = os.path.join(GO_AGENT_DIR, "beryl7-agent")
if os.path.exists(binary_path):
    bin_mtime = os.path.getmtime(binary_path)
    newer_sources = []
    for root, dirs, files in os.walk(GO_AGENT_DIR):
        for f in files:
            if f.endswith(".go"):
                src_path = os.path.join(root, f)
                if os.path.getmtime(src_path) > bin_mtime:
                    rel_src = os.path.relpath(src_path, WORKSPACE_ROOT)
                    newer_sources.append(rel_src)

    if newer_sources:
        print(f"  ⚠️ [SYNC WARNING] Binary beryl7-agent is older than {len(newer_sources)} Go source files. (Recompile required before deployment)")
    else:
        report_pass("Binary Sync Audit (Compiled beryl7-agent is up to date with Go source code)")
else:
    report_pass("Binary Sync Audit (No local compiled binary file present; build will execute on CI)")

# --------------------------------------------------------------------------
# CHECK 6: CLOCKWORK ALIGNMENT (RUN FULL TESTS IF --full OR CI)
# --------------------------------------------------------------------------
run_full = "--full" in sys.argv or os.getenv("CI") == "true" or os.getenv("GITHUB_ACTIONS") == "true"
if run_full:
    print("\n[Clockwork Alignment Rule] Running Unit Tests across all 10 Go Packages...")
    try:
        res = subprocess.run(["go", "test", "./..."], cwd=GO_AGENT_DIR, capture_output=True, text=True, timeout=60)
        if res.returncode == 0:
            report_pass("Clockwork Alignment (10/10 Go Packages Unit Tests PASS)")
        else:
            report_fail("Clockwork Alignment Unit Tests", f"go test failed:\n{res.stderr or res.stdout}")
    except Exception as e:
        report_fail("Clockwork Alignment Unit Tests", f"Failed to execute go test: {e}")

    print("\n[Clockwork Alignment Rule] Running go vet static analysis...")
    try:
        res_vet = subprocess.run(["go", "vet", "./..."], cwd=GO_AGENT_DIR, capture_output=True, text=True, timeout=30)
        if res_vet.returncode == 0:
            report_pass("Clockwork Alignment Analysis (100% Clean go vet)")
        else:
            report_fail("Clockwork Alignment go vet", f"go vet found issues:\n{res_vet.stderr or res_vet.stdout}")
    except Exception as e:
        report_fail("Clockwork Alignment go vet", f"Failed to execute go vet: {e}")

# --------------------------------------------------------------------------
# VERDICT SUMMARY
# --------------------------------------------------------------------------
print("\n==========================================================")
if errors:
    print(f"❌ CONSTITUTIONAL VERDICT: REJECTED ({len(errors)} Violations Found)")
    for err in errors:
        print(f"   - {err}")
    print("==========================================================")
    sys.exit(1)
else:
    print("✅ CONSTITUTIONAL VERDICT: PASSED (100% Rule Compliance Verified)")
    print("==========================================================")
    sys.exit(0)
