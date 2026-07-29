package watchdog

import (
	"path/filepath"
	"testing"
)

func TestWatchdogCore(t *testing.T) {
	tempDir := t.TempDir()
	checkpointPath := filepath.Join(tempDir, "checkpoint.uci")

	wd := New(checkpointPath)
	_ = wd.SaveCheckpoint(map[string]string{"test": "val"})

	_ = wd.LoadAndVerifyCheckpoint()
	wd.RecordHealthCheckSuccess()
	_ = wd.ExecuteRollback()
}
