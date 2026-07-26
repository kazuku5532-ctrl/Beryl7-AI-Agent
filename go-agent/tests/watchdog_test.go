package tests

import (
	"os"
	"testing"

	"beryl7-agent/watchdog"
)

func TestWatchdogCheckpointSHA256(t *testing.T) {
	checkpointFile := "test_checkpoint.uci"
	defer os.Remove(checkpointFile)
	defer os.Remove(checkpointFile + ".tmp")

	wd := watchdog.New(checkpointFile)

	err := wd.SaveCheckpoint(map[string]string{"network.wan.proto": "dhcp"})
	if err != nil {
		t.Fatalf("Failed to save checkpoint: %v", err)
	}

	err = wd.LoadAndVerifyCheckpoint()
	if err != nil {
		t.Fatalf("Checkpoint SHA256 verification failed: %v", err)
	}

	// Test Safe Mode exit counter
	for i := 0; i < 2; i++ {
		exited := wd.RecordHealthCheckSuccess()
		if exited {
			t.Errorf("Should not exit safe mode before 3 checks")
		}
	}
}
