package main

import (
	"os"
	"path/filepath"
	"testing"

	"beryl7-agent/ai"
	"beryl7-agent/config"
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

func TestQueuePendingApprovalAndAudit(t *testing.T) {
	tempDir := t.TempDir()
	pendingFile := filepath.Join(tempDir, "pending.json")
	auditFile := filepath.Join(tempDir, "audit.log")

	resp := &ai.AIResponse{
		Action:     "purge_memory_cache",
		Reasoning:  "High RAM usage",
		Confidence: 0.9,
	}

	queuePendingApproval(resp, 0.85)

	if _, err := os.Stat("/var/run/beryl7_pending_approval.json"); err == nil {
		_ = os.Remove("/var/run/beryl7_pending_approval.json")
	}

	recordApprovalAuditLog("purge_memory_cache", "127.0.0.1")
	_ = pendingFile
	_ = auditFile
}
