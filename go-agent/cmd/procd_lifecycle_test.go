package main

import (
	"beryl7-agent/config"
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func buildAgent(t *testing.T) string {
	exeName := "beryl7-agent-test"
	if os.PathSeparator == '\\' {
		exeName += ".exe"
	}
	binPath := filepath.Join(t.TempDir(), exeName)
	cmd := exec.Command("go", "build", "-o", binPath, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build agent: %v\n%s", err, out)
	}
	return binPath
}

func TestProcdLifecycleVerbatim(t *testing.T) {
	binPath := buildAgent(t)

	tempDir := t.TempDir()
	tempEnvPath := filepath.Join(tempDir, "agent.env")
	os.WriteFile(tempEnvPath, []byte("HEALTH_PORT=9999\n"), 0644)

	tempKeyPath := filepath.Join(tempDir, "agent.key")
	os.WriteFile(tempKeyPath, []byte("GEMINI_API_KEY=testkey123\n"), 0644)

	// Test flag execution (E, F)
	t.Run("Case E: Version Flag", func(t *testing.T) {
		cmd := exec.Command(binPath, "-version")
		var out bytes.Buffer
		cmd.Stdout = &out
		if err := cmd.Run(); err != nil {
			t.Fatalf("expected exit 0, got %v", err)
		}
		if !strings.Contains(out.String(), "beryl7-agent version") {
			t.Errorf("expected version output, got: %s", out.String())
		}
	})

	t.Run("Case F: Benchmark Flag", func(t *testing.T) {
		cmd := exec.Command(binPath, "-benchmark")
		var out bytes.Buffer
		cmd.Stdout = &out
		if err := cmd.Run(); err != nil {
			t.Fatalf("expected exit 0, got %v", err)
		}
		if !strings.Contains(out.String(), "Running in-memory hardware micro-benchmarks") {
			t.Errorf("expected benchmark output, got: %s", out.String())
		}
	})

	// Test isolated config loading for A, B, C, D, G
	t.Run("Case A: Default Procd", func(t *testing.T) {
		cfg, _ := config.LoadConfigWithFlags("/etc/beryl7/agent.env", "", false, false)
		if cfg.ConfigFilePath != "/etc/beryl7/agent.env" {
			t.Errorf("expected /etc/beryl7/agent.env, got %s", cfg.ConfigFilePath)
		}
	})

	t.Run("Case B: Custom Config", func(t *testing.T) {
		cfg, _ := config.LoadConfigWithFlags(tempEnvPath, "", false, false)
		if cfg.ConfigFilePath != tempEnvPath {
			t.Errorf("expected %s, got %s", tempEnvPath, cfg.ConfigFilePath)
		}
		if cfg.HealthPort != 9999 {
			t.Errorf("expected port 9999 from custom config, got %d", cfg.HealthPort)
		}
	})

	t.Run("Case C: Custom Keyfile", func(t *testing.T) {
		cfg, _ := config.LoadConfigWithFlags("", tempKeyPath, false, false)
		if cfg.KeyFilePath != tempKeyPath {
			t.Errorf("expected %s, got %s", tempKeyPath, cfg.KeyFilePath)
		}
		if cfg.GeminiAPIKey != "testkey123" {
			t.Errorf("expected testkey123 from keyfile, got %s", cfg.GeminiAPIKey)
		}
	})

	t.Run("Case D: Dry-Run Mode", func(t *testing.T) {
		cfg, _ := config.LoadConfigWithFlags("", "", true, false)
		if !cfg.DryRun {
			t.Errorf("expected DryRun true")
		}
	})

	t.Run("Case G: Combined Flags", func(t *testing.T) {
		cfg, _ := config.LoadConfigWithFlags(tempEnvPath, tempKeyPath, true, false)
		if cfg.ConfigFilePath != tempEnvPath {
			t.Errorf("expected ConfigFilePath %s", tempEnvPath)
		}
		if cfg.KeyFilePath != tempKeyPath {
			t.Errorf("expected KeyFilePath %s", tempKeyPath)
		}
		if !cfg.DryRun {
			t.Errorf("expected DryRun true")
		}
		if cfg.HealthPort != 9999 {
			t.Errorf("expected HealthPort 9999")
		}
		if cfg.GeminiAPIKey != "testkey123" {
			t.Errorf("expected API Key testkey123")
		}
	})
}
