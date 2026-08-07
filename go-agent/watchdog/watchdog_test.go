package watchdog

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWatchdogAllBranches(t *testing.T) {
	tempDir := t.TempDir()
	cpPath := filepath.Join(tempDir, "checkpoint.uci")

	wd := New(cpPath)
	if !wd.IsSafeMode() {
		t.Errorf("Expected SafeMode=true when checkpoint file is missing")
	}

	err := wd.SaveCheckpoint(map[string]string{"network.wan.proto": "dhcp"})
	if err != nil {
		t.Fatalf("Failed to save checkpoint: %v", err)
	}

	err = wd.LoadAndVerifyCheckpoint()
	if err != nil {
		t.Fatalf("Failed to verify checkpoint: %v", err)
	}

	// Corrupt checkpoint file to test checksum error
	_ = os.WriteFile(cpPath, []byte(`{"version":1, "checksum":"corrupted"}`), 0600)
	errCorrupt := wd.LoadAndVerifyCheckpoint()
	if errCorrupt == nil {
		t.Errorf("Expected checksum mismatch error for corrupted file")
	}

	// Re-save valid checkpoint
	_ = wd.SaveCheckpoint(map[string]string{"test": "ok"})
	wd.safeModeActive = true
	wd.successfulChecks = 0

	for i := 0; i < 5; i++ {
		exited := wd.RecordHealthCheckSuccess()
		if i == 4 && !exited {
			t.Errorf("Expected RecordHealthCheckSuccess to return true on 5th success")
		}
	}

	if wd.IsSafeMode() {
		t.Errorf("Expected SafeMode=false after 5 successes")
	}

	_ = wd.RecordHealthCheckSuccess()

	// Rollback
	_ = wd.ExecuteRollback()
	if !wd.IsSafeMode() {
		t.Errorf("Expected SafeMode=true after ExecuteRollback")
	}

	_ = UCISyntaxPreCheck()
}

// TestWatchdogSuspendResume covers the Suspend/Resume branch inside IsSafeMode
func TestWatchdogSuspendResume(t *testing.T) {
	tempDir := t.TempDir()
	cpPath := filepath.Join(tempDir, "checkpoint_suspend.uci")
	wd := New(cpPath)

	// Force safeMode = true (no checkpoint file → New sets it)
	if !wd.IsSafeMode() {
		t.Errorf("Expected SafeMode=true on fresh watchdog with no checkpoint")
	}

	// Suspend → IsSafeMode should return false even though safeModeActive=true
	wd.Suspend()
	if wd.IsSafeMode() {
		t.Errorf("Expected IsSafeMode=false while suspended")
	}

	// Resume → IsSafeMode should return true again
	wd.Resume()
	if !wd.IsSafeMode() {
		t.Errorf("Expected IsSafeMode=true after Resume")
	}
}

// TestRecordHealthCheckSuccessNotInSafeMode ensures RecordHealthCheckSuccess returns false
// immediately when safeModeActive=false (not in safe mode).
func TestRecordHealthCheckSuccessNotInSafeMode(t *testing.T) {
	tempDir := t.TempDir()
	cpPath := filepath.Join(tempDir, "checkpoint_nosafe.uci")
	// Pre-create a valid checkpoint so New() loads it without entering safe mode
	wd := &Watchdog{
		checkpointPath: cpPath,
		safeModeActive: false,
		rollbackWindow: 0,
	}
	exited := wd.RecordHealthCheckSuccess()
	if exited {
		t.Errorf("Expected false when not in safe mode, got true")
	}
}

// TestWatchdogCheckpointRoundTrip verifies save→load round-trip integrity for different configs
func TestWatchdogCheckpointRoundTrip(t *testing.T) {
	tempDir := t.TempDir()
	cpPath := filepath.Join(tempDir, "checkpoint_rt.uci")
	wd := &Watchdog{
		checkpointPath: cpPath,
		safeModeActive: true,
		successfulChecks: 3,
	}

	cfg := map[string]string{
		"network.wan.proto":    "pppoe",
		"network.wan.username": "testuser",
	}
	if err := wd.SaveCheckpoint(cfg); err != nil {
		t.Fatalf("SaveCheckpoint failed: %v", err)
	}

	wd2 := &Watchdog{checkpointPath: cpPath}
	if err := wd2.LoadAndVerifyCheckpoint(); err != nil {
		t.Fatalf("LoadAndVerifyCheckpoint failed: %v", err)
	}
	if !wd2.safeModeActive {
		t.Errorf("Expected safeModeActive=true after round-trip")
	}
	if wd2.successfulChecks != 3 {
		t.Errorf("Expected successfulChecks=3, got %d", wd2.successfulChecks)
	}
}

// TestLoadAndVerifyCheckpointMalformedJSON covers the JSON unmarshal error branch
func TestLoadAndVerifyCheckpointMalformedJSON(t *testing.T) {
	tempDir := t.TempDir()
	cpPath := filepath.Join(tempDir, "checkpoint_bad.uci")
	_ = os.WriteFile(cpPath, []byte(`not valid json at all {{{`), 0600)
	wd := &Watchdog{checkpointPath: cpPath}
	if err := wd.LoadAndVerifyCheckpoint(); err == nil {
		t.Errorf("Expected error on malformed JSON checkpoint")
	}
}
