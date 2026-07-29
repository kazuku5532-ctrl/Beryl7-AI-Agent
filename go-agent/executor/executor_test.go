package executor

import (
	"context"
	"testing"
)

func TestNewExecutor(t *testing.T) {
	exec := New()
	if exec == nil {
		t.Fatal("Expected non-nil Executor")
	}
}

func TestGetActionRiskThreshold(t *testing.T) {
	exec := New()
	thresholds := map[string]float64{
		"purge_memory_cache":    0.60,
		"restart_wan_interface": 0.85,
		"optimize_wifi_channel": 0.85,
		"unknown_action":        0.90,
	}

	for action, expected := range thresholds {
		val := exec.GetActionRiskThreshold(action)
		if val != expected {
			t.Errorf("Action %s: expected threshold %.2f, got %.2f", action, expected, val)
		}
	}
}

func TestExecuteActionDryRun(t *testing.T) {
	exec := New()
	ctx := context.Background()

	req := &ActionRequest{
		ActionName: "purge_memory_cache",
		Target:     "sys",
	}

	err := exec.ExecuteAction(ctx, req, true)
	if err != nil {
		t.Errorf("Expected dry run execution to succeed, got %v", err)
	}

	invalidReq := &ActionRequest{
		ActionName: "malicious_script_injection",
		Target:     "sys",
	}
	err = exec.ExecuteAction(ctx, invalidReq, true)
	if err == nil {
		t.Errorf("Expected invalid action to be rejected by whitelist")
	}
}
