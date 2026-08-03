package tests

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"beryl7-agent/config"
	"beryl7-agent/skillstore"
	"beryl7-agent/telemetry"
)

func TestEndToEndSystemIntegration(t *testing.T) {
	tempDir := t.TempDir()
	envPath := filepath.Join(tempDir, "agent.env")
	dbPath := filepath.Join(tempDir, "skills.db")
	checkpointPath := filepath.Join(tempDir, "checkpoint.uci")

	validAdminToken := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	validOpToken := "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"

	t.Setenv("AUTH_TOKEN", validAdminToken)
	t.Setenv("APPROVE_TOKEN", validOpToken)
	t.Setenv("BERYL7_RAM_EXHAUSTION_PCT", "93.5")
	t.Setenv("BERYL7_SKILLSTORE_PATH", dbPath)
	t.Setenv("BERYL7_CHECKPOINT_PATH", checkpointPath)
	t.Setenv("BERYL7_LATENCY_ZSCORE", "2.8")

	envContent := "AUTH_TOKEN=" + validAdminToken + "\nAPPROVE_TOKEN=" + validOpToken + "\nLOG_LEVEL=INFO\nBERYL7_LATENCY_ZSCORE=2.8\n"
	if err := os.WriteFile(envPath, []byte(envContent), 0600); err != nil {
		t.Fatalf("Failed to write test env file: %v", err)
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.SkillStorePath != dbPath {
		t.Errorf("Expected SkillStorePath %s, got %s", dbPath, cfg.SkillStorePath)
	}
	if cfg.CheckpointPath != checkpointPath {
		t.Errorf("Expected CheckpointPath %s, got %s", checkpointPath, cfg.CheckpointPath)
	}

	if err := config.ValidateSystemConfiguration(cfg); err != nil {
		t.Errorf("ValidateSystemConfiguration failed: %v", err)
	}

	store, err := skillstore.New(dbPath)
	if err != nil {
		t.Fatalf("SkillStore init failed: %v", err)
	}
	defer store.Close()

	testSkill := &skillstore.Skill{
		ID:           "skill-wan-drop-1",
		Action:       "restart_wan_interface",
		Condition:    "WAN_DROP",
		Confidence:   0.95,
		SuccessCount: 10,
		FailureCount: 0,
	}
	if err := store.SaveOrUpdateSkill(testSkill, true, 0.3); err != nil {
		t.Errorf("SaveOrUpdateSkill failed: %v", err)
	}

	retrievedSkill := store.GetSkill("WAN_DROP", "restart_wan_interface")
	if retrievedSkill == nil || retrievedSkill.Action != "restart_wan_interface" {
		t.Errorf("GetSkill failed or returned mismatch: skill=%v", retrievedSkill)
	}

	collector := telemetry.NewCollector()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	m := collector.CollectMetrics(ctx)
	if m == nil {
		t.Errorf("CollectMetrics returned nil in integration test")
	}
}
