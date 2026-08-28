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

	// Test fallback key file reading into Config
	cfg := &Config{KeyFilePath: keyPath}
	k, errRead := readSecureKeyFile(cfg.KeyFilePath)
	if errRead != nil || k != "my-secret-key" {
		t.Errorf("Expected my-secret-key from cfg.KeyFilePath, got %s", k)
	}
}

func TestV16EnterpriseConfigFunctions(t *testing.T) {
	cfg, _ := LoadConfig()

	_ = EnsureSysupgradePreservation()
	_ = EnsureFilePermissions()
	if err := EnsureProcdInitService(); err != nil {
		t.Errorf("EnsureProcdInitService failed: %v", err)
	}
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
	t.Setenv("BERYL7_AIRGAPPED_MODE", "true")
	t.Setenv("TELEGRAM_BOT_TOKEN", "123456:TEST_TOKEN")
	t.Setenv("TELEGRAM_CHAT_ID", "987654321")
	cfgEnv, _ := LoadConfig()
	if cfgEnv.RAMExhaustionPct != 95.5 || cfgEnv.LatencyZScoreThreshold != 3.0 || !cfgEnv.AirgappedMode {
		t.Errorf("Expected RAM 95.5, ZScore 3.0, and Airgapped true, got RAM=%.1f ZScore=%.1f Airgapped=%v", cfgEnv.RAMExhaustionPct, cfgEnv.LatencyZScoreThreshold, cfgEnv.AirgappedMode)
	}
	if cfgEnv.TelegramBotToken != "123456:TEST_TOKEN" || cfgEnv.TelegramChatID != "987654321" {
		t.Errorf("Expected TelegramBotToken and ChatID to match env, got token=%s chatID=%s", cfgEnv.TelegramBotToken, cfgEnv.TelegramChatID)
	}
}

func TestParseEnvFile(t *testing.T) {
	tempDir := t.TempDir()
	envPath := filepath.Join(tempDir, "agent.env")
	envContent := `
HEALTH_PORT=9999
BIND_HOST=0.0.0.0
LOG_LEVEL=DEBUG
DRY_RUN=true
DISABLE_AUTO_HEALING=true
AUTH_TOKEN=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
APPROVE_TOKEN=fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210
TELEGRAM_BOT_TOKEN=888888:ENV_TOKEN
TELEGRAM_CHAT_ID=777777
BERYL7_AIRGAPPED_MODE=true
`
	_ = os.WriteFile(envPath, []byte(envContent), 0600)

	cfg := &Config{}
	err := parseEnvFile(envPath, cfg)
	if err != nil {
		t.Fatalf("Failed to parse env file: %v", err)
	}

	if cfg.HealthPort != 9999 || cfg.BindHost != "0.0.0.0" || !cfg.DryRun || !cfg.DisableAutoHeal || !cfg.AirgappedMode {
		t.Errorf("Parsed config mismatch: %+v", cfg)
	}

	if cfg.TelegramBotToken != "888888:ENV_TOKEN" || cfg.TelegramChatID != "777777" {
		t.Errorf("Telegram config mismatch in parseEnvFile: token=%s chatID=%s", cfg.TelegramBotToken, cfg.TelegramChatID)
	}
}

func TestValidateSystemConfigurationErrors(t *testing.T) {
	// Nil config
	if err := ValidateSystemConfiguration(nil); err == nil {
		t.Errorf("Expected error for nil config")
	}

	// Empty AuthToken
	cfg := &Config{
		AuthToken: "",
	}
	if err := ValidateSystemConfiguration(cfg); err == nil {
		t.Errorf("Expected error for empty AuthToken")
	}

	// Weak AuthToken (< 32 chars)
	cfg.AuthToken = "short-token"
	if err := ValidateSystemConfiguration(cfg); err == nil {
		t.Errorf("Expected error for weak AuthToken")
	}

	// ApproveToken same as AuthToken
	cfg.AuthToken = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	cfg.ApproveToken = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	if err := ValidateSystemConfiguration(cfg); err == nil {
		t.Errorf("Expected error when ApproveToken equals AuthToken")
	}
}
