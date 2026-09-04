package main

import (
	"beryl7-agent/config"
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func buildAgent(t *testing.T) string {
	exeName := "beryl7-agent-test"
	if os.PathSeparator == '\\' {
		exeName += ".exe"
	}
	binPath := filepath.Join(t.TempDir(), exeName)
	cmd := exec.Command("go", "build", "-o", binPath, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build agent binary: %v\n%s", err, out)
	}
	return binPath
}

// runBinaryCheckFlags executes the real compiled binary with specified CLI flags
// and asserts that it does NOT crash due to Go's flag.Parse rejecting unrecognized flags.
func runBinaryCheckFlags(t *testing.T, binPath string, args ...string) {
	t.Helper()
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd := exec.Command(binPath, args...)
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start process with args %v: %v", args, err)
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		stderr := stderrBuf.String()
		if strings.Contains(stderr, "flag provided but not defined") {
			t.Fatalf("CRASH REGRESSION: flag.Parse failed on args %v!\nStderr: %s", args, stderr)
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 2 {
				t.Fatalf("CRASH REGRESSION: Process exited with flag error code 2 on args %v!\nStderr: %s", args, stderr)
			}
		}
	case <-time.After(600 * time.Millisecond):
		// The binary is actively running its daemon loop without crashing at flag.Parse()
		_ = cmd.Process.Kill()
		<-done
		stderr := stderrBuf.String()
		if strings.Contains(stderr, "flag provided but not defined") {
			t.Fatalf("CRASH REGRESSION: flag error detected in stderr: %s", stderr)
		}
	}
}

func TestProcdLifecycleVerbatim(t *testing.T) {
	binPath := buildAgent(t)

	tempDir := t.TempDir()
	tempEnvPath := filepath.Join(tempDir, "agent.env")
	if err := os.WriteFile(tempEnvPath, []byte("HEALTH_PORT=9999\n"), 0644); err != nil {
		t.Fatalf("failed to write temp env file: %v", err)
	}

	tempKeyPath := filepath.Join(tempDir, "agent.key")
	if err := os.WriteFile(tempKeyPath, []byte("GEMINI_API_KEY=testkey123\n"), 0644); err != nil {
		t.Fatalf("failed to write temp key file: %v", err)
	}

	// =========================================================================
	// 1. REAL PROCESS EXECUTION TESTS (Testing main() & flag.Parse() Layer)
	// =========================================================================

	t.Run("Case A: Verbatim Procd Invocation (-config /etc/beryl7/agent.env)", func(t *testing.T) {
		// Runs the real compiled binary with the EXACT parameters used by /etc/init.d/beryl7-agent.
		// Guarantees flag.Parse() accepts "-config" without crashing with exit code 2.
		runBinaryCheckFlags(t, binPath, "-config", "/etc/beryl7/agent.env")
	})

	t.Run("Case B: Custom Config Invocation (-config <tempEnv>)", func(t *testing.T) {
		runBinaryCheckFlags(t, binPath, "-config", tempEnvPath)
	})

	t.Run("Case C: Custom Keyfile Invocation (-config <tempEnv> -keyfile <tempKey>)", func(t *testing.T) {
		runBinaryCheckFlags(t, binPath, "-config", tempEnvPath, "-keyfile", tempKeyPath)
	})

	t.Run("Case D: Dry-Run Mode Invocation (-config <tempEnv> -dry-run)", func(t *testing.T) {
		runBinaryCheckFlags(t, binPath, "-config", tempEnvPath, "-dry-run")
	})

	t.Run("Case E: Version Flag (-version)", func(t *testing.T) {
		cmd := exec.Command(binPath, "-version")
		var out bytes.Buffer
		cmd.Stdout = &out
		if err := cmd.Run(); err != nil {
			t.Fatalf("expected exit code 0 for -version, got %v", err)
		}
		if !strings.Contains(out.String(), "beryl7-agent version") {
			t.Errorf("expected version output, got: %s", out.String())
		}
	})

	t.Run("Case F: Benchmark Flag (-benchmark)", func(t *testing.T) {
		cmd := exec.Command(binPath, "-benchmark")
		var out bytes.Buffer
		cmd.Stdout = &out
		if err := cmd.Run(); err != nil {
			t.Fatalf("expected exit code 0 for -benchmark, got %v", err)
		}
		if !strings.Contains(out.String(), "Running in-memory hardware micro-benchmarks") {
			t.Errorf("expected benchmark output, got: %s", out.String())
		}
	})

	t.Run("Case G: Combined Flags (-config <tempEnv> -keyfile <tempKey> -dry-run)", func(t *testing.T) {
		runBinaryCheckFlags(t, binPath, "-config", tempEnvPath, "-keyfile", tempKeyPath, "-dry-run")
	})

	t.Run("Negative Control: Undefined Flag Rejection", func(t *testing.T) {
		// Validates that the test assertion actively catches flag parsing failures.
		// If an unregistered flag is passed, flag.Parse() must exit with status 2 and error in stderr.
		cmd := exec.Command(binPath, "-undefined-flag-xyz")
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		err := cmd.Run()
		if err == nil {
			t.Fatalf("expected process to fail on undefined flag, but it exited 0")
		}
		if exitErr, ok := err.(*exec.ExitError); !ok || exitErr.ExitCode() != 2 {
			t.Errorf("expected exit code 2 from flag package, got %v", err)
		}
		if !strings.Contains(stderr.String(), "flag provided but not defined: -undefined-flag-xyz") {
			t.Errorf("expected 'flag provided but not defined' in stderr, got: %s", stderr.String())
		}
	})

	// =========================================================================
	// 2. IN-MEMORY CONFIGURATION VERIFICATION (Testing config.LoadConfigWithFlags)
	// =========================================================================

	t.Run("Config Logic: Custom Config Values", func(t *testing.T) {
		cfg, err := config.LoadConfigWithFlags(tempEnvPath, "", false, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.ConfigFilePath != tempEnvPath {
			t.Errorf("expected %s, got %s", tempEnvPath, cfg.ConfigFilePath)
		}
		if cfg.HealthPort != 9999 {
			t.Errorf("expected port 9999 from custom config, got %d", cfg.HealthPort)
		}
	})

	t.Run("Config Logic: Custom Keyfile Values", func(t *testing.T) {
		cfg, err := config.LoadConfigWithFlags("", tempKeyPath, false, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.KeyFilePath != tempKeyPath {
			t.Errorf("expected %s, got %s", tempKeyPath, cfg.KeyFilePath)
		}
		if cfg.GeminiAPIKey != "testkey123" {
			t.Errorf("expected testkey123 from keyfile, got %s", cfg.GeminiAPIKey)
		}
	})

	t.Run("Config Logic: DryRun Setting", func(t *testing.T) {
		cfg, err := config.LoadConfigWithFlags("", "", true, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !cfg.DryRun {
			t.Errorf("expected DryRun true")
		}
	})

	t.Run("Config Logic: Combined Flag Properties", func(t *testing.T) {
		cfg, err := config.LoadConfigWithFlags(tempEnvPath, tempKeyPath, true, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
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
