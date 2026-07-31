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
