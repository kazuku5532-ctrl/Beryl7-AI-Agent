package executor

import (
	"context"
	"testing"
)

func TestNewExecutor(t *testing.T) {
	exec := New()
	if exec == nil {
		t.Fatalf("Expected New() to return valid pointer, got nil")
	}
}

func TestGetActionRiskThreshold(t *testing.T) {
	exec := New()
	th1 := exec.GetActionRiskThreshold("restart_wan_interface")
	if th1 != 0.85 {
		t.Errorf("Expected threshold 0.85 for restart_wan_interface, got %.2f", th1)
	}

	th2 := exec.GetActionRiskThreshold("unknown_action")
	if th2 != 0.90 {
		t.Errorf("Expected threshold 0.90 for unknown action, got %.2f", th2)
	}
}

func TestExecuteActionDryRun(t *testing.T) {
	exec := New()
	req := &ActionRequest{
		ActionName: "restart_wan_interface",
		Target:     "wan",
	}
	err := exec.ExecuteAction(context.Background(), req, true)
	if err != nil {
		t.Fatalf("Expected dry run execution to succeed, got %v", err)
	}
}
