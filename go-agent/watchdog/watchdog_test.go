package watchdog

import (
	"path/filepath"
	"testing"
)

func TestWatchdogCore(t *testing.T) {
	tempDir := t.TempDir()
	checkpointPath := filepath.Join(tempDir, "checkpoint.uci")

	wd := New(checkpointPath)
	if !wd.IsSafeMode() {
		t.Logf("Initial safe mode handled")
	}

	err := wd.SaveCheckpoint(map[string]string{"network.wan.proto": "dhcp"})
	if err != nil {
		t.Fatalf("Failed to save checkpoint: %v", err)
	}

	err = wd.LoadAndVerifyCheckpoint()
	if err != nil {
		t.Fatalf("Failed to verify checkpoint: %v", err)
	}

	for i := 0; i < 4; i++ {
		wd.RecordHealthCheckSuccess()
	}

	_ = wd.ExecuteRollback()
	_ = UCISyntaxPreCheck()
}
