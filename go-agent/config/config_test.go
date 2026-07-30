package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigDefaults(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}
	if cfg.HealthPort <= 0 {
		t.Errorf("Expected positive HealthPort, got %d", cfg.HealthPort)
	}

	_ = cfg.GetAPIKeySnapshot()
}

func TestKillSwitch(t *testing.T) {
	cfg, _ := LoadConfig()

	if IsKillSwitchActive(cfg) {
		t.Logf("Kill switch active state checked")
	}

	cfg.DisableAutoHeal = true
	if !IsKillSwitchActive(cfg) {
		t.Errorf("Expected kill switch active when DisableAutoHeal is true")
	}
}

func TestReadSecureKeyFile(t *testing.T) {
	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "agent.key")

	_ = os.WriteFile(keyPath, []byte("  my-secret-key  "), 0600)

	key, err := readSecureKeyFile(keyPath)
	if err != nil || key != "my-secret-key" {
		t.Errorf("Expected my-secret-key, got %s (err=%v)", key, err)
	}
}

func TestV16EnterpriseConfigFunctions(t *testing.T) {
	cfg, _ := LoadConfig()

	_ = EnsureSysupgradePreservation()
	_ = EnsureFilePermissions()
	_ = EnsureProcdInitService()
	_ = DetectSystemCapability(cfg)

	warnings := DryRunUpgradeCheck("5.0")
	if len(warnings) == 0 {
		t.Logf("DryRunUpgradeCheck executed")
	}
}
