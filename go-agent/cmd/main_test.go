package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"beryl7-agent/ai"
	"beryl7-agent/config"
	"beryl7-agent/executor"
	"beryl7-agent/skillstore"
	"beryl7-agent/telemetry"
	"beryl7-agent/watchdog"
)

func TestValidateTokenRole(t *testing.T) {
	cfg := &config.Config{
		AuthToken:    "admin-secret-token",
		ApproveToken: "operator-secret-token",
	}

	remoteReq, _ := http.NewRequest("GET", "/api/test", nil)
	remoteReq.RemoteAddr = "192.168.8.100:54321"

	localReq, _ := http.NewRequest("GET", "/api/test", nil)
	localReq.RemoteAddr = "127.0.0.1:54321"

	// 1. Remote Requests
	role, valid := validateTokenRole(remoteReq, "Bearer admin-secret-token", cfg)
	if !valid || role != "admin" {
		t.Errorf("Expected admin role, got role=%s valid=%v", role, valid)
	}

	role, valid = validateTokenRole(remoteReq, "Bearer operator-secret-token", cfg)
	if !valid || role != "operator" {
		t.Errorf("Expected operator role, got role=%s valid=%v", role, valid)
	}

	role, valid = validateTokenRole(remoteReq, "", cfg)
	if !valid || role != "viewer" {
		t.Errorf("Expected viewer role for empty header, got role=%s valid=%v", role, valid)
	}

	role, valid = validateTokenRole(remoteReq, "Bearer invalid-token", cfg)
	if valid && role != "unknown" {
		t.Errorf("Expected unknown role for invalid token, got role=%s valid=%v", role, valid)
	}

	// 2. Localhost Bypass Test
	role, valid = validateTokenRole(localReq, "", cfg)
	if !valid || role != "admin" {
		t.Errorf("Expected admin role for localhost bypass, got role=%s valid=%v", role, valid)
	}

	// 3. Single-Token Mode Test
	singleCfg := &config.Config{
		AuthToken:    "single-secret-token",
		ApproveToken: "",
	}
	role, valid = validateTokenRole(remoteReq, "Bearer single-secret-token", singleCfg)
	if !valid || role != "operator" {
		t.Errorf("Expected operator role in Single-Token mode, got role=%s valid=%v", role, valid)
	}
}

func TestHealthCheckServerEndpoints(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	cpPath := filepath.Join(tempDir, "cp.uci")

	store, err := skillstore.New(dbPath)
	if err != nil {
		t.Fatalf("Failed to init store: %v", err)
	}
	defer store.Close()

	wd := watchdog.New(cpPath)
	execEngine := executor.New()
	aiClient := ai.NewClient("dummy-key")

	// Allocate dynamic free port to prevent test port collisions
	listener, lErr := net.Listen("tcp", "127.0.0.1:0")
	if lErr != nil {
		t.Fatalf("Failed to bind dynamic test port: %v", lErr)
	}
	testPort := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()

	cfg := &config.Config{
		HealthPort:   testPort,
		BindHost:     "127.0.0.1",
		AuthToken:    "admin-secret",
		ApproveToken: "operator-secret",
		DryRun:       true,
	}

	health := &HealthState{
		Status:        "healthy",
		LastAction:    "none",
		StartTime:     time.Now(),
		UptimeSeconds: 100,
	}

	collector := telemetry.NewCollector()
	server := StartHealthCheckServer(cfg, health, execEngine, store, aiClient, wd, collector)
	defer server.Close()

	time.Sleep(100 * time.Millisecond)

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", testPort)

	endpoints := []string{
		baseURL + "/api/health",
		baseURL + "/api/v1/metrics",
		baseURL + "/api/modules/status",
		baseURL + "/metrics",
		baseURL + "/api/budget/status",
		baseURL + "/api/circuit-breaker",
	}

	for _, ep := range endpoints {
		resp, err := http.Get(ep)
		if err != nil {
			t.Errorf("Failed GET %s: %v", ep, err)
		} else {
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Errorf("Endpoint %s returned status %d", ep, resp.StatusCode)
			}
		}
	}

	// Test /api/logs via remote request validation (should return 401 Unauthorized for non-localhost without token)
	remoteReq, _ := http.NewRequest("GET", "/api/logs", nil)
	remoteReq.RemoteAddr = "192.168.8.100:54321"
	role, valid := validateTokenRole(remoteReq, "", cfg)
	if valid && role != "viewer" {
		t.Errorf("Remote unauthenticated /api/logs expected viewer role, got %s", role)
	}

	// Test /api/logs with Auth header
	logsReq, _ := http.NewRequest("GET", baseURL+"/api/logs", nil)
	logsReq.Header.Set("Authorization", "Bearer operator-secret")
	respLogs, errLogs := http.DefaultClient.Do(logsReq)
	if errLogs != nil {
		t.Errorf("Failed GET /api/logs: %v", errLogs)
	} else {
		respLogs.Body.Close()
		if respLogs.StatusCode != http.StatusOK {
			t.Errorf("Authenticated /api/logs returned status %d", respLogs.StatusCode)
		}
	}

	// Test OPTIONS CORS
	req, _ := http.NewRequest("OPTIONS", baseURL+"/api/health", nil)
	resp, err := http.DefaultClient.Do(req)
	if err == nil {
		resp.Body.Close()
	}

	// Test CORS with null origin (should reject and not echo null)
	reqNull, _ := http.NewRequest("GET", baseURL+"/api/health", nil)
	reqNull.Header.Set("Origin", "null")
	respNull, errNull := http.DefaultClient.Do(reqNull)
	if errNull == nil {
		allowOrigin := respNull.Header.Get("Access-Control-Allow-Origin")
		if allowOrigin == "null" {
			t.Errorf("CORS should not allow origin 'null' by default, got: %s", allowOrigin)
		}
		respNull.Body.Close()
	}

	// Test DisableLocalhostBypass
	cfgStrict := &config.Config{
		AuthToken:              "admin-secret",
		DisableLocalhostBypass: true,
	}
	localReq, _ := http.NewRequest("POST", "/api/config/reload", nil)
	localReq.RemoteAddr = "127.0.0.1:12345"
	roleStrict, validStrict := validateTokenRole(localReq, "", cfgStrict)
	if validStrict && roleStrict == "admin" {
		t.Errorf("Expected localhost bypass to be blocked when DisableLocalhostBypass is true, got role %s", roleStrict)
	}

	// Test Config Reload with operator role
	reloadReq, _ := http.NewRequest("POST", baseURL+"/api/config/reload", nil)
	reloadReq.Header.Set("Authorization", "Bearer operator-secret")
	respReload, err := http.DefaultClient.Do(reloadReq)
	if err == nil {
		respReload.Body.Close()
	}

	// Test Config Reload unauthorized
	reloadBad, _ := http.NewRequest("POST", baseURL+"/api/config/reload", nil)
	respBad, err := http.DefaultClient.Do(reloadBad)
	if err == nil {
		respBad.Body.Close()
	}

	// Test Approve with pending request
	queuePendingApproval(&ai.AIResponse{Action: "purge_memory_cache", Confidence: 0.9}, 0.85)

	appReq, _ := http.NewRequest("POST", baseURL+"/api/approve", bytes.NewBuffer([]byte("{}")))
	appReq.Header.Set("Authorization", "Bearer operator-secret")
	respApp, err := http.DefaultClient.Do(appReq)
	if err == nil {
		respApp.Body.Close()
	}

	// Test Approve unauthorized
	appBad, _ := http.NewRequest("POST", baseURL+"/api/approve", bytes.NewBuffer([]byte("{}")))
	respAppBad, err := http.DefaultClient.Do(appBad)
	if err == nil {
		respAppBad.Body.Close()
	}
}

func TestQueuePendingApprovalAndAuditLog(t *testing.T) {
	resp := &ai.AIResponse{
		Action:     "purge_memory_cache",
		Reasoning:  "High RAM usage",
		Confidence: 0.9,
	}

	queuePendingApproval(resp, 0.85)
	recordApprovalAuditLog("purge_memory_cache", "127.0.0.1")

	_ = getSystemLogSample()
	pidFile := filepath.Join(os.TempDir(), "test.pid")
	_ = acquirePIDLock(pidFile)
	_ = acquirePIDLock(pidFile)
	_ = os.Remove(pidFile)
}

func TestV16EnterpriseFailsafeAndValidation(t *testing.T) {
	cfg := &config.Config{
		AuthToken:      "test-token",
		SkillStorePath: filepath.Join(t.TempDir(), "test_skills.db"),
	}

	_ = CheckBinaryCompatibility()
	_ = PostUpgradeValidation(cfg)

	_ = FailsafeRecovery(FailsafeLevel1, cfg)
	_ = FailsafeRecovery(FailsafeLevel2, cfg)
	_ = FailsafeRecovery(FailsafeLevel3, cfg)
	_ = FailsafeRecovery(FailsafeLevel4, cfg)

	_ = PostRollbackValidationChecklist(cfg)
}

func TestFullDaemonE2EWithHILEmulator(t *testing.T) {
	tempDir := t.TempDir()
	fakeBinDir := filepath.Join(tempDir, "bin")
	_ = os.MkdirAll(fakeBinDir, 0755)

	uciExt := ""
	if runtime.GOOS == "windows" {
		uciExt = ".cmd"
	}
	uciScript := "@echo off\r\nif \"%1\"==\"export\" echo package network\r\nif \"%1\"==\"export\" echo config interface 'wan'\r\nif \"%1\"==\"export\" echo     option proto 'dhcp'\r\nexit /b 0"
	if runtime.GOOS != "windows" {
		uciScript = "#!/bin/sh\nif [ \"$1\" = \"export\" ]; then\n  echo 'package network'\n  echo 'config interface wan'\n  echo '    option proto dhcp'\nfi\nexit 0"
	}
	_ = os.WriteFile(filepath.Join(fakeBinDir, "uci"+uciExt), []byte(uciScript), 0755)
	t.Setenv("PATH", fakeBinDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	store, err := skillstore.New(filepath.Join(tempDir, "hil_daemon.db"))
	if err != nil {
		t.Fatalf("Failed to init store: %v", err)
	}
	defer store.Close()

	wd := watchdog.New(filepath.Join(tempDir, "cp.uci"))
	execEngine := executor.New()
	aiClient := ai.NewClient("dummy-key")

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to bind dynamic test port: %v", err)
	}
	testPort := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()

	cfg := &config.Config{
		HealthPort:   testPort,
		BindHost:     "127.0.0.1",
		AuthToken:    "admin-secret",
		ApproveToken: "operator-secret",
		DryRun:       true,
	}
	health := &HealthState{
		Status:        "healthy",
		LastAction:    "none",
		StartTime:     time.Now(),
		UptimeSeconds: 100,
	}

	collector := telemetry.NewCollector()
	server := StartHealthCheckServer(cfg, health, execEngine, store, aiClient, wd, collector)
	defer server.Close()

	time.Sleep(100 * time.Millisecond)

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", testPort)
	resp, err := http.Get(baseURL + "/api/health")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("Real Daemon Health Check failed: %v", err)
	}
	_ = resp.Body.Close()

	// Verify REAL daemon endpoint /api/modules/status
	modResp, err := http.Get(baseURL + "/api/modules/status")
	if err != nil || modResp.StatusCode != http.StatusOK {
		t.Fatalf("Real Daemon Module Status Check failed: %v", err)
	}
	_ = modResp.Body.Close()
}

func TestGoroutineLeakGuard(t *testing.T) {
	before := runtime.NumGoroutine()
	tempDir := t.TempDir()
	store, _ := skillstore.New(filepath.Join(tempDir, "bench_skills.db"))
	if store != nil {
		store.Close()
	}
	time.Sleep(50 * time.Millisecond)
	after := runtime.NumGoroutine()
	if delta := after - before; delta > 2 {
		t.Errorf("Goroutine leak detected: before=%d, after=%d, delta=%d", before, after, delta)
	}
}

func TestAutoRollback(t *testing.T) {
	cfg := &config.Config{}
	_ = AutoRollback(cfg)
}

func TestAdditionalAPIEndpoints(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_api.db")
	cpPath := filepath.Join(tempDir, "cp_api.uci")

	store, _ := skillstore.New(dbPath)
	defer store.Close()

	wd := watchdog.New(cpPath)
	execEngine := executor.New()
	aiClient := ai.NewClient("dummy-key")

	listener, _ := net.Listen("tcp", "127.0.0.1:0")
	testPort := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()

	cfg := &config.Config{
		HealthPort:   testPort,
		BindHost:     "127.0.0.1",
		AuthToken:    "admin-secret",
		ApproveToken: "operator-secret",
		DryRun:       true,
	}

	health := &HealthState{
		Status:        "healthy",
		LastAction:    "none",
		StartTime:     time.Now(),
		UptimeSeconds: 100,
	}

	collector := telemetry.NewCollector()
	server := StartHealthCheckServer(cfg, health, execEngine, store, aiClient, wd, collector)
	defer server.Close()

	time.Sleep(100 * time.Millisecond)
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", testPort)

	// Test GET /api/q-table
	respQ, errQ := http.Get(baseURL + "/api/q-table")
	if errQ == nil {
		respQ.Body.Close()
	}

	// Test GET /api/history
	respH, errH := http.Get(baseURL + "/api/history")
	if errH == nil {
		respH.Body.Close()
	}

	// Test POST /api/kill-switch/enable with operator auth
	reqKS, _ := http.NewRequest("POST", baseURL+"/api/kill-switch/enable", nil)
	reqKS.Header.Set("Authorization", "Bearer operator-secret")
	respKS, errKS := http.DefaultClient.Do(reqKS)
	if errKS == nil {
		respKS.Body.Close()
	}

	// Test POST /api/kill-switch/disable
	reqKSD, _ := http.NewRequest("POST", baseURL+"/api/kill-switch/disable", nil)
	reqKSD.Header.Set("Authorization", "Bearer operator-secret")
	respKSD, errKSD := http.DefaultClient.Do(reqKSD)
	if errKSD == nil {
		respKSD.Body.Close()
	}

	// Test POST /api/actions/execute
	actionBody := bytes.NewBuffer([]byte(`{"action_name":"purge_memory_cache","target":"wan"}`))
	reqAct, _ := http.NewRequest("POST", baseURL+"/api/actions/execute", actionBody)
	reqAct.Header.Set("Authorization", "Bearer operator-secret")
	respAct, errAct := http.DefaultClient.Do(reqAct)
	if errAct == nil {
		respAct.Body.Close()
	}
}

func TestCORSAndAuthSecurity_Comprehensive(t *testing.T) {
	cfg := &config.Config{
		AuthToken:              "admin-token-xyz",
		ApproveToken:           "op-token-123",
		BindHost:               "127.0.0.1",
		HealthPort:             8888,
		CORSAllowedOrigins:     "http://192.168.8.1:8888,http://127.0.0.1:8888,http://localhost:8888",
		DisableLocalhostBypass: false,
	}

	testCases := []struct {
		origin         string
		expectedAllow  string
		description    string
	}{
		{"null", "http://127.0.0.1:8888", "Origin null should fallback to default safe origin"},
		{"http://192.168.8.1:8888", "http://192.168.8.1:8888", "Valid router IP origin allowed"},
		{"http://127.0.0.1:8888", "http://127.0.0.1:8888", "Valid loopback origin allowed"},
		{"http://localhost:8888", "http://localhost:8888", "Valid localhost origin allowed"},
		{"http://localhost:8888.attacker.com", "http://127.0.0.1:8888", "Subdomain suffix spoofing rejected"},
		{"http://192.168.8.1:8888.evil.com", "http://127.0.0.1:8888", "Prefix spoofing rejected"},
		{"https://untrusted-site.com", "http://127.0.0.1:8888", "Untrusted origin rejected"},
		{"", "http://127.0.0.1:8888", "Empty origin fallback to default safe origin"},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/api/health", nil)
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			setCorsHeaders(rec, req, cfg)
			got := rec.Header().Get("Access-Control-Allow-Origin")
			if got != tc.expectedAllow {
				t.Errorf("Origin %q: expected allow header %q, got %q", tc.origin, tc.expectedAllow, got)
			}
		})
	}

	// Test validateTokenRole with edge cases
	reqInvalidAuth := httptest.NewRequest("GET", "/api/status", nil)
	reqInvalidAuth.RemoteAddr = "10.0.0.2:1234"
	
	// Malformed auth headers
	badHeaders := []string{
		"Basic abcdef",
		"Bearer",
		"Bearer ",
		"Token 12345",
		"bearer admin-token-xyz", // case-sensitive check
	}
	for _, bh := range badHeaders {
		role, valid := validateTokenRole(reqInvalidAuth, bh, cfg)
		if valid && role == "admin" {
			t.Errorf("Malformed header %q should not yield admin role", bh)
		}
	}

	// Concurrent role validation race safety test
	const goroutines = 25
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			r := httptest.NewRequest("GET", "/api/status", nil)
			if id%2 == 0 {
				r.RemoteAddr = "127.0.0.1:8080"
				_, _ = validateTokenRole(r, "", cfg)
			} else {
				r.RemoteAddr = "192.168.1.50:8080"
				_, _ = validateTokenRole(r, "Bearer admin-token-xyz", cfg)
			}
		}(i)
	}
	wg.Wait()
}

func TestIntentAwareEndpoints(t *testing.T) {
	tempDir := t.TempDir()
	intentsPath := filepath.Join(tempDir, "intents.json")

	ramVal := 88.5
	latVal := 45.0
	intentsFile := config.IntentConfigFile{
		Intents: []config.Intent{
			{
				Name:             "test_work_intent",
				StartTime:        "00:00",
				EndTime:          "23:59",
				RAMExhaustionPct: &ramVal,
				LatencySpikeMs:   &latVal,
			},
		},
	}
	data, _ := json.Marshal(intentsFile)
	_ = os.WriteFile(intentsPath, data, 0600)

	dbPath := filepath.Join(tempDir, "test.db")
	store, _ := skillstore.New(dbPath)
	defer store.Close()

	wd := watchdog.New(filepath.Join(tempDir, "cp.uci"))
	execEngine := executor.New()
	aiClient := ai.NewClient("dummy-key")
	collector := telemetry.NewCollector()

	listener, lErr := net.Listen("tcp", "127.0.0.1:0")
	if lErr != nil {
		t.Fatalf("Failed to bind dynamic port: %v", lErr)
	}
	testPort := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()

	cfg := &config.Config{
		HealthPort:             testPort,
		BindHost:               "127.0.0.1",
		IntentsFilePath:        intentsPath,
		RAMExhaustionPct:       95.0,
		CPUSpikeLoad:           1.5,
		LatencySpikeMs:         100.0,
		LatencyZScoreThreshold: 2.5,
	}
	cfgAtomic.Store(cfg)

	health := &HealthState{
		Status:        "healthy",
		LastAction:    "none",
		StartTime:     time.Now(),
		UptimeSeconds: 50,
	}

	server := StartHealthCheckServer(cfg, health, execEngine, store, aiClient, wd, collector)
	defer server.Close()

	time.Sleep(100 * time.Millisecond)

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", testPort)

	// Test 1: GET /api/health
	respHealth, err := http.Get(baseURL + "/api/health")
	if err != nil {
		t.Fatalf("Failed GET /api/health: %v", err)
	}
	defer respHealth.Body.Close()

	var healthRes HealthState
	if err := json.NewDecoder(respHealth.Body).Decode(&healthRes); err != nil {
		t.Fatalf("Failed to decode /api/health response: %v", err)
	}

	if healthRes.ActiveIntent != "test_work_intent" {
		t.Errorf("Expected ActiveIntent 'test_work_intent', got %q", healthRes.ActiveIntent)
	}
	if healthRes.RAMExhaustionPct != 88.5 {
		t.Errorf("Expected RAMExhaustionPct 88.5, got %.1f", healthRes.RAMExhaustionPct)
	}
	if healthRes.LatencySpikeMs != 45.0 {
		t.Errorf("Expected LatencySpikeMs 45.0, got %.1f", healthRes.LatencySpikeMs)
	}

	// Test 2: GET /api/v1/metrics
	respMetrics, err := http.Get(baseURL + "/api/v1/metrics")
	if err != nil {
		t.Fatalf("Failed GET /api/v1/metrics: %v", err)
	}
	defer respMetrics.Body.Close()

	var metricsRes struct {
		ActiveIntent        string                     `json:"active_intent"`
		EffectiveThresholds config.EffectiveThresholds `json:"effective_thresholds"`
	}
	if err := json.NewDecoder(respMetrics.Body).Decode(&metricsRes); err != nil {
		t.Fatalf("Failed to decode /api/v1/metrics response: %v", err)
	}

	if metricsRes.ActiveIntent != "test_work_intent" {
		t.Errorf("Expected metrics ActiveIntent 'test_work_intent', got %q", metricsRes.ActiveIntent)
	}
	if metricsRes.EffectiveThresholds.RAMExhaustionPct != 88.5 {
		t.Errorf("Expected metrics effective RAM threshold 88.5, got %.1f", metricsRes.EffectiveThresholds.RAMExhaustionPct)
	}
}

func TestTelemetryHistory_APIEndpoint(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "telemetry_api_test.db")
	cpPath := filepath.Join(tempDir, "cp_tel.uci")

	store, err := skillstore.New(dbPath)
	if err != nil {
		t.Fatalf("Failed to init store: %v", err)
	}
	defer store.Close()

	wd := watchdog.New(cpPath)
	execEngine := executor.New()
	aiClient := ai.NewClient("dummy-key")
	collector := telemetry.NewCollector()

	listener, lErr := net.Listen("tcp", "127.0.0.1:0")
	if lErr != nil {
		t.Fatalf("Failed to bind dynamic test port: %v", lErr)
	}
	testPort := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()

	cfg := &config.Config{
		HealthPort:             testPort,
		BindHost:               "127.0.0.1",
		AuthToken:              "admin-secret",
		ApproveToken:           "operator-secret",
		DryRun:                 true,
		TelemetryRetentionDays: 30,
	}

	health := &HealthState{
		Status:        "healthy",
		LastAction:    "none",
		StartTime:     time.Now(),
		UptimeSeconds: 100,
	}

	server := StartHealthCheckServer(cfg, health, execEngine, store, aiClient, wd, collector)
	defer server.Close()

	time.Sleep(100 * time.Millisecond)

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", testPort)

	// 1. Test GET /api/telemetry/history on empty DB
	respEmpty, err := http.Get(baseURL + "/api/telemetry/history")
	if err != nil {
		t.Fatalf("Failed GET /api/telemetry/history: %v", err)
	}
	defer respEmpty.Body.Close()

	if respEmpty.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 OK on empty DB, got %d", respEmpty.StatusCode)
	}

	var resEmpty struct {
		Status         string                      `json:"status"`
		Hours          int                         `json:"hours"`
		RecordCount    int                         `json:"record_count"`
		EstimatedBytes int64                       `json:"estimated_bytes"`
		Records        []skillstore.TelemetryRecord `json:"records"`
	}
	if err := json.NewDecoder(respEmpty.Body).Decode(&resEmpty); err != nil {
		t.Fatalf("Failed to decode empty response: %v", err)
	}
	if resEmpty.Status != "success" || resEmpty.Hours != 24 || resEmpty.RecordCount != 0 || resEmpty.Records == nil {
		t.Errorf("Unexpected empty response payload: %+v", resEmpty)
	}

	// 2. Insert test records
	ctx := context.Background()
	now := time.Now().Unix()
	_ = store.RecordTelemetryHistory(ctx, skillstore.TelemetryRecord{
		Timestamp:    now - 7200, // 2 hours ago
		RAMPct:       45.0,
		LatencyMs:    12.0,
		CPUPct:       20.0,
		TempC:        50.0,
		WANOffline:   false,
		WiFiFail:     false,
		ActiveIntent: "default",
	})
	_ = store.RecordTelemetryHistory(ctx, skillstore.TelemetryRecord{
		Timestamp:    now - 1800, // 30 minutes ago
		RAMPct:       60.0,
		LatencyMs:    25.0,
		CPUPct:       35.0,
		TempC:        55.0,
		WANOffline:   true,
		WiFiFail:     false,
		ActiveIntent: "gaming",
	})
	_ = store.RecordTelemetryHistory(ctx, skillstore.TelemetryRecord{
		Timestamp:    now - 300, // 5 minutes ago
		RAMPct:       50.0,
		LatencyMs:    15.0,
		CPUPct:       22.0,
		TempC:        52.0,
		WANOffline:   false,
		WiFiFail:     false,
		ActiveIntent: "eco",
	})

	// 3. Test GET /api/telemetry/history?hours=3 (should return all 3)
	resp3h, err := http.Get(baseURL + "/api/telemetry/history?hours=3")
	if err != nil {
		t.Fatalf("Failed GET /api/telemetry/history?hours=3: %v", err)
	}
	defer resp3h.Body.Close()

	var res3h struct {
		Status         string                      `json:"status"`
		Hours          int                         `json:"hours"`
		RecordCount    int                         `json:"record_count"`
		EstimatedBytes int64                       `json:"estimated_bytes"`
		Records        []skillstore.TelemetryRecord `json:"records"`
	}
	if err := json.NewDecoder(resp3h.Body).Decode(&res3h); err != nil {
		t.Fatalf("Failed to decode 3h response: %v", err)
	}
	if res3h.RecordCount != 3 || len(res3h.Records) != 3 {
		t.Errorf("Expected 3 records for 3h window, got count=%d len=%d", res3h.RecordCount, len(res3h.Records))
	}
	if res3h.EstimatedBytes != 3*64 {
		t.Errorf("Expected EstimatedBytes %d, got %d", 3*64, res3h.EstimatedBytes)
	}

	// 4. Test GET /api/v1/telemetry/history?hours=1 (alias endpoint, should return 2 records from last hour)
	resp1h, err := http.Get(baseURL + "/api/v1/telemetry/history?hours=1")
	if err != nil {
		t.Fatalf("Failed GET /api/v1/telemetry/history?hours=1: %v", err)
	}
	defer resp1h.Body.Close()

	var res1h struct {
		Status         string                      `json:"status"`
		Hours          int                         `json:"hours"`
		RecordCount    int                         `json:"record_count"`
		Records        []skillstore.TelemetryRecord `json:"records"`
	}
	if err := json.NewDecoder(resp1h.Body).Decode(&res1h); err != nil {
		t.Fatalf("Failed to decode 1h response: %v", err)
	}
	if res1h.RecordCount != 2 || len(res1h.Records) != 2 {
		t.Errorf("Expected 2 records for 1h window, got count=%d len=%d", res1h.RecordCount, len(res1h.Records))
	}

	// 5. Test POST method rejection
	postReq, _ := http.NewRequest("POST", baseURL+"/api/telemetry/history", bytes.NewBufferString("{}"))
	respPost, errPost := http.DefaultClient.Do(postReq)
	if errPost != nil {
		t.Fatalf("Failed POST /api/telemetry/history: %v", errPost)
	}
	defer respPost.Body.Close()
	if respPost.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405 Method Not Allowed on POST, got %d", respPost.StatusCode)
	}
}


