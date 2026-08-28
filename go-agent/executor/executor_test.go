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
		"no_action_required":    0.50,
		"purge_memory_cache":    0.60,
		"restart_wan_interface": 0.85,
		"optimize_wifi_channel": 0.85,
		"boost_wifi_bandwidth":  0.90,
		"revert_wifi_bandwidth": 0.90,
		"remediate_sticky_clients": 0.70,
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
		"no_action_required",
		"purge_memory_cache",
		"restart_wan_interface",
		"restart_interface",
		"optimize_wifi_channel",
		"set_qos_priority",
		"block_device",
		"set_wan_mac",
		"boost_wifi_bandwidth",
		"revert_wifi_bandwidth",
		"tune_network_performance",
		"enable_cake_sqm",
		"remediate_sticky_clients",
	}

	// Test dry run mode
	for _, act := range actions {
		req := &ActionRequest{ActionName: act, Target: "wan", Parameters: map[string]interface{}{"mac": "00:11:22:33:44:55", "interface": "wan", "channel": 6, "priority": "high"}}
		err := exec.ExecuteAction(ctx, req, true)
		if err != nil {
			t.Errorf("Action %s in dry-run failed: %v", act, err)
		}
	}

	// Test real action execution mode (non dry-run)
	for _, act := range actions {
		req := &ActionRequest{ActionName: act, Target: "wan", Parameters: map[string]interface{}{"mac": "00:11:22:33:44:55", "interface": "wan", "channel": 6, "priority": "high"}}
		_ = exec.ExecuteAction(ctx, req, false)
	}

	// Test additional enterprise execution actions
	extraActions := []string{
		"scale_tx_power_down",
		"align_channels",
		"ap_failover",
	}
	for _, act := range extraActions {
		req := &ActionRequest{ActionName: act, Target: "radio1"}
		_ = exec.ExecuteAction(ctx, req, true)
		_ = exec.ExecuteAction(ctx, req, false)
	}

	_ = exec.TriggerWiFiReload(ctx)
	exec.SetTelemetryProvider(nil)

	// Test invalid / empty action
	_ = exec.ExecuteAction(ctx, nil, true)
	_ = exec.ExecuteAction(ctx, &ActionRequest{ActionName: ""}, true)
	_ = exec.ExecuteAction(ctx, &ActionRequest{ActionName: "invalid_action"}, false)
}
