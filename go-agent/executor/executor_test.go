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
		"boost_wifi_bandwidth":  0.90,
		"revert_wifi_bandwidth": 0.90,
		"unknown_action":        0.90,
	}

	for action, expected := range thresholds {
		val := exec.GetActionRiskThreshold(action)
		if val != expected {
			t.Errorf("Action %s: expected threshold %.2f, got %.2f", action, expected, val)
		}
	}
}

func TestExecuteActionAllWhitelisted(t *testing.T) {
	exec := New()
	ctx := context.Background()

	actions := []string{
		"purge_memory_cache",
		"restart_wan_interface",
		"optimize_wifi_channel",
		"boost_wifi_bandwidth",
		"revert_wifi_bandwidth",
	}

	for _, act := range actions {
		req := &ActionRequest{ActionName: act, Target: "wan"}
		err := exec.ExecuteAction(ctx, req, true)
		if err != nil {
			t.Errorf("Action %s in dry-run failed: %v", act, err)
		}
	}

	invalidReq := &ActionRequest{ActionName: "rm -rf /", Target: "sys"}
	err := exec.ExecuteAction(ctx, invalidReq, true)
	if err == nil {
		t.Errorf("Expected rejection for non-whitelisted action")
	}
}
