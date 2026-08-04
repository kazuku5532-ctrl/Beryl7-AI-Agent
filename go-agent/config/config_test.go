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

func TestValidateSystemDependenciesAndConfig(t *testing.T) {
	_ = ValidateSystemDependencies()

	cfg, _ := LoadConfig()
	cfg.AuthToken = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	cfg.ApproveToken = "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"
	err := ValidateSystemConfiguration(cfg)
	if err != nil {
		t.Errorf("ValidateSystemConfiguration failed: %v", err)
	}

	t.Setenv("BERYL7_RAM_EXHAUSTION_PCT", "95.5")
	t.Setenv("BERYL7_LATENCY_ZSCORE", "3.0")
	cfgEnv, _ := LoadConfig()
	if cfgEnv.RAMExhaustionPct != 95.5 || cfgEnv.LatencyZScoreThreshold != 3.0 {
		t.Errorf("Expected RAM 95.5 and ZScore 3.0, got RAM=%.1f ZScore=%.1f", cfgEnv.RAMExhaustionPct, cfgEnv.LatencyZScoreThreshold)
	}
}
