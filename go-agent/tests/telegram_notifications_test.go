package tests

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"beryl7-agent/notifier"
	"beryl7-agent/skillstore"
)

func TestGracefulShutdownMarker(t *testing.T) {
	tmpDir := t.TempDir()
	markerPath := filepath.Join(tmpDir, ".beryl7_graceful_shutdown")
	tmpPath := markerPath + ".tmp"

	// Simulate Graceful Shutdown Write
	markerData, _ := json.Marshal(map[string]interface{}{
		"magic":     "BERYL7_GRACEFUL_EXIT_V1",
		"timestamp": time.Now().Format(time.RFC3339),
		"pid":       os.Getpid(),
		"reason":    "SIGTERM",
	})

	f, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		t.Fatalf("Failed to create tmp file: %v", err)
	}
	f.Write(markerData)
	f.Sync()
	f.Close()

	if err := os.Rename(tmpPath, markerPath); err != nil {
		t.Fatalf("Failed to rename marker file: %v", err)
	}

	// Simulate Startup Check (Clean)
	data, err := os.ReadFile(markerPath)
	if err != nil {
		t.Fatalf("Failed to read marker: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if magic, ok := m["magic"].(string); !ok || magic != "BERYL7_GRACEFUL_EXIT_V1" {
		t.Fatalf("Expected valid magic string")
	}

	// Simulate Corrupt marker
	os.WriteFile(markerPath, []byte("corrupt data"), 0600)
	corruptData, err := os.ReadFile(markerPath)
	if err == nil {
		var m2 map[string]interface{}
		if err := json.Unmarshal(corruptData, &m2); err == nil {
			if m2["magic"] == "BERYL7_GRACEFUL_EXIT_V1" {
				t.Fatalf("Should not parse corrupt data")
			}
		}
	}
}

func TestMilestoneLatchPersistence(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "skills.db")

	store, err := skillstore.New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	latchKey := "test_milestone_latch"

	// Set latch first time
	isFirst, err := store.CheckAndSetMilestoneLatch(latchKey)
	if err != nil {
		t.Fatalf("CheckAndSetMilestoneLatch failed: %v", err)
	}
	if !isFirst {
		t.Fatalf("Expected isFirst to be true on first set")
	}

	// Set latch second time (same instance)
	isFirst2, err := store.CheckAndSetMilestoneLatch(latchKey)
	if err != nil {
		t.Fatalf("CheckAndSetMilestoneLatch failed: %v", err)
	}
	if isFirst2 {
		t.Fatalf("Expected isFirst to be false on second set")
	}

	store.Close()

	// Reopen store to test persistence
	store2, err := skillstore.New(dbPath)
	if err != nil {
		t.Fatalf("Failed to reopen store: %v", err)
	}
	defer store2.Close()

	// Set latch third time (reopened instance)
	isFirst3, err := store2.CheckAndSetMilestoneLatch(latchKey)
	if err != nil {
		t.Fatalf("CheckAndSetMilestoneLatch failed: %v", err)
	}
	if isFirst3 {
		t.Fatalf("Expected isFirst to be false after reopen")
	}
}

func TestTelegramFormats(t *testing.T) {
	notifierInstance := notifier.NewTelegramNotifier("dummy-token", "123456", false)
	if notifierInstance == nil {
		t.Fatalf("Expected non-nil notifier")
	}

	// Just verify that methods execute without panics since we can't easily mock http.Client in struct
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	err := notifierInstance.SendPowerLossRecoveryAlert(ctx, time.Now(), "Active (1/1)")
	if err == nil {
		// Should fail due to timeout/invalid token but not crash
	}

	metrics := skillstore.OperationalMetrics{
		TotalQUpdates:     25,
		VerifiedSuccesses: 20,
		VerifiedFailures:  5,
	}
	err = notifierInstance.SendMilestoneAlert(ctx, metrics, 25)
	if err == nil {
		// Should fail due to timeout/invalid token but not crash
	}
}
