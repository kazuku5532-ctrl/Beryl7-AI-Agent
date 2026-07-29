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
}

func TestKillSwitch(t *testing.T) {
	cfg, _ := LoadConfig()

	if IsKillSwitchActive(cfg) {
		t.Logf("Kill switch active state checked")
	}

	disablePath := filepath.Join(os.TempDir(), "beryl7-disable")
	_ = os.WriteFile(disablePath, []byte("1"), 0600)
	defer os.Remove(disablePath)
}
