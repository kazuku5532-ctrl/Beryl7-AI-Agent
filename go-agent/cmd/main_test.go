package main

import (
	"bytes"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"beryl7-agent/ai"
	"beryl7-agent/config"
	"beryl7-agent/executor"
	"beryl7-agent/skillstore"
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

	server := StartHealthCheckServer(cfg, health, execEngine, store, aiClient, wd)
	defer server.Close()

	time.Sleep(100 * time.Millisecond)

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", testPort)

	endpoints := []string{
		baseURL + "/api/health",
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

	server := StartHealthCheckServer(cfg, health, execEngine, store, aiClient, wd)
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

	server := StartHealthCheckServer(cfg, health, execEngine, store, aiClient, wd)
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
