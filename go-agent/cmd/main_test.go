package main

import (
	"bytes"
	"net/http"
	"os"
	"path/filepath"
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

	role, valid := validateTokenRole("Bearer admin-secret-token", cfg)
	if !valid || role != "admin" {
		t.Errorf("Expected admin role, got role=%s valid=%v", role, valid)
	}

	role, valid = validateTokenRole("Bearer operator-secret-token", cfg)
	if !valid || role != "operator" {
		t.Errorf("Expected operator role, got role=%s valid=%v", role, valid)
	}

	role, valid = validateTokenRole("", cfg)
	if !valid || role != "viewer" {
		t.Errorf("Expected viewer role for empty header, got role=%s valid=%v", role, valid)
	}

	role, valid = validateTokenRole("Bearer invalid-token", cfg)
	if valid && role != "unknown" {
		t.Errorf("Expected unknown role for invalid token, got role=%s valid=%v", role, valid)
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

	cfg := &config.Config{
		HealthPort:   8899,
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

	server := startHealthCheckServer(cfg, health, execEngine, store, aiClient, wd)
	defer server.Close()

	time.Sleep(100 * time.Millisecond)

	endpoints := []string{
		"http://127.0.0.1:8899/api/health",
		"http://127.0.0.1:8899/api/modules/status",
		"http://127.0.0.1:8899/metrics",
		"http://127.0.0.1:8899/api/budget/status",
		"http://127.0.0.1:8899/api/circuit-breaker",
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

	// Test /api/logs with Auth header
	logsReq, _ := http.NewRequest("GET", "http://127.0.0.1:8899/api/logs", nil)
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
	req, _ := http.NewRequest("OPTIONS", "http://127.0.0.1:8899/api/health", nil)
	resp, err := http.DefaultClient.Do(req)
	if err == nil {
		resp.Body.Close()
	}

	// Test Config Reload with operator role
	reloadReq, _ := http.NewRequest("POST", "http://127.0.0.1:8899/api/config/reload", nil)
	reloadReq.Header.Set("Authorization", "Bearer operator-secret")
	respReload, err := http.DefaultClient.Do(reloadReq)
	if err == nil {
		respReload.Body.Close()
	}

	// Test Config Reload unauthorized
	reloadBad, _ := http.NewRequest("POST", "http://127.0.0.1:8899/api/config/reload", nil)
	respBad, err := http.DefaultClient.Do(reloadBad)
	if err == nil {
		respBad.Body.Close()
	}

	// Test Approve with pending request
	queuePendingApproval(&ai.AIResponse{Action: "purge_memory_cache", Confidence: 0.9}, 0.85)

	appReq, _ := http.NewRequest("POST", "http://127.0.0.1:8899/api/approve", bytes.NewBuffer([]byte("{}")))
	appReq.Header.Set("Authorization", "Bearer operator-secret")
	respApp, err := http.DefaultClient.Do(appReq)
	if err == nil {
		respApp.Body.Close()
	}

	// Test Approve unauthorized
	appBad, _ := http.NewRequest("POST", "http://127.0.0.1:8899/api/approve", bytes.NewBuffer([]byte("{}")))
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
	_ = AutoRollback(cfg)
}
