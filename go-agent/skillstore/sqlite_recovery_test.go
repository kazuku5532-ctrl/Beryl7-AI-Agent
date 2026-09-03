package skillstore

import (
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"
)

// TestSQLiteCorruptionBugReproduction reproduces the bug where PRAGMA integrity_check error bypasses salvage
func TestSQLiteCorruptionBugReproduction(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "corrupt_test.db")
	bakPath := dbPath + ".bak"

	// 1. Create a healthy store and save data + backup
	store, err := New(dbPath)
	if err != nil {
		t.Fatalf("Initial New failed: %v", err)
	}
	sk := &Skill{
		ID:         "WAN_DROP:restart_wan",
		Action:     "restart_wan",
		Condition:  "WAN_DROP",
		Confidence: 0.85,
	}
	_ = store.SaveOrUpdateSkill(sk, true, 0.2)
	_ = store.BackupDatabase()
	_ = store.Close()

	// 2. Corrupt the database file header completely
	garbage := make([]byte, 1024)
	_, _ = rand.Read(garbage)
	if err := os.WriteFile(dbPath, garbage, 0600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// 3. Attempt to open corrupted database
	// Recovery Routine: OpenAndInit intercepts PRAGMA integrity_check error, safely closes the DB handle,
	// archives corrupted file, and restores the database from recent .bak snapshot.
	reopenedStore, errReopen := New(dbPath)
	if errReopen != nil {
		t.Fatalf("Expected database to recover cleanly, but New(dbPath) failed with error: %v", errReopen)
	}
	defer reopenedStore.Close()

	// 4. Verify data was restored from .bak snapshot
	restoredSkill := reopenedStore.GetSkill("WAN_DROP", "restart_wan")
	if restoredSkill == nil {
		t.Errorf("Expected skill WAN_DROP:restart_wan to be restored from backup, but got nil")
	} else if restoredSkill.Action != "restart_wan" {
		t.Errorf("Expected restored action restart_wan, got %s", restoredSkill.Action)
	}

	// Verify .bak file still exists
	if _, err := os.Stat(bakPath); err != nil {
		t.Errorf(".bak file missing: %v", err)
	}
}

// TestSQLiteMidFileCorruption tests behavior when SQLite header is intact but page data is corrupted
func TestSQLiteMidFileCorruption(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "corrupt_mid.db")

	// 1. Create healthy database with multiple records
	store, err := New(dbPath)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	for i := 0; i < 20; i++ {
		sk := &Skill{
			ID:         filepath.Join("test_id", string(rune('A'+i))),
			Action:     "action_test",
			Condition:  "cond_test",
			Confidence: 0.5,
		}
		_ = store.SaveOrUpdateSkill(sk, true, 0.2)
	}
	_ = store.Close()

	// 2. Corrupt bytes in the middle of the database file (preserving 100-byte SQLite header)
	dbData, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if len(dbData) > 512 {
		for i := 200; i < 400 && i < len(dbData); i++ {
			dbData[i] = 0xFF
		}
		_ = os.WriteFile(dbPath, dbData, 0600)
	}

	// 3. Attempt to open
	t.Logf("Attempting to open middle-corrupted database...")
	salvagedStore, err := New(dbPath)
	if err != nil {
		t.Logf("Observation: Middle-corrupted database open returned: %v", err)
	} else {
		defer salvagedStore.Close()
		t.Logf("Middle-corrupted store reopened successfully.")
	}
}
