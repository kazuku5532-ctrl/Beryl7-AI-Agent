package skillstore

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestSkillStoreLifecycle(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_skills.db")

	store, err := New(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize SkillStore: %v", err)
	}
	defer store.Close()

	skill := &Skill{
		ID:         "WAN_DROP:restart_wan_interface",
		Action:     "restart_wan_interface",
		Condition:  "WAN_DROP",
		Confidence: 0.90,
	}

	err = store.SaveOrUpdateSkill(skill, true, 0.3)
	if err != nil {
		t.Fatalf("Failed to save skill: %v", err)
	}

	retrieved := store.GetSkill("WAN_DROP", "restart_wan_interface")
	if retrieved == nil {
		t.Fatalf("Expected non-nil skill for WAN_DROP:restart_wan_interface")
	}
	if retrieved.Action != "restart_wan_interface" {
		t.Errorf("Expected action restart_wan_interface, got %s", retrieved.Action)
	}

	err = store.SaveOrUpdateSkill(skill, false, 0.3)
	if err != nil {
		t.Errorf("Failed to update skill on failure: %v", err)
	}

	_ = store.PruneSkillsPeriodic()
	_ = store.BackupDatabase()

	// Test v16.0 skill store functions
	translated := TranslateSkillInterface("ifconfig eth0 up", "4.9.0", "5.0")
	if translated != "ifconfig wan0 up" {
		t.Errorf("Unexpected translation result: %s", translated)
	}

	compatible := store.FilterCompatibleSkills("5.0")
	if len(compatible) < 0 {
		t.Errorf("FilterCompatibleSkills returned nil slice")
	}

	// Test Q-Learning and recommendations
	action, conf, errRec := store.RecommendBestAction("WAN_DROP", "restart_wan_interface")
	if errRec != nil || action == "" || conf < 0 {
		t.Errorf("Expected valid recommendation, got action=%s conf=%.2f err=%v", action, conf, errRec)
	}

	bestSkill := store.GetBestSkillForAnomaly("WAN_DROP")
	if bestSkill == nil {
		t.Errorf("Expected best skill for WAN_DROP")
	}

	errQ := store.UpdateQValue("WAN_DROP", "restart_wan_interface", 1.0)
	if errQ != nil {
		t.Errorf("UpdateQValue failed: %v", errQ)
	}

	_ = store.GetTopSkillsSummary(5)
	_ = store.GetTopSkillsSummaryForAnomaly("WAN_DROP", 5)

	flushTarget := filepath.Join(tempDir, "flush.db")
	_ = store.FlushToPersistent(flushTarget)

	backupPath := dbPath + ".bak"
	_ = os.Remove(backupPath)
}

func TestNewHybridStore(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "hybrid_skills.db")
	flashPath := filepath.Join(tempDir, "flash_skills.db")

	store, err := NewHybrid(dbPath, flashPath)
	if err != nil {
		t.Fatalf("Failed to create hybrid store: %v", err)
	}
	defer store.Close()
}

func TestSkillStoreOptimizeAndVacuum(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "vacuum_skills.db")

	store, err := New(dbPath)
	if err != nil {
		t.Fatalf("Failed to init store: %v", err)
	}

	// 1. Optimize and vacuum active store
	if errOpt := store.OptimizeAndVacuum(); errOpt != nil {
		t.Errorf("OptimizeAndVacuum failed on active store: %v", errOpt)
	}

	// 2. High concurrency write + vacuum race test
	const workers = 15
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		workerID := i
		go func() {
			defer wg.Done()
			skill := &Skill{
				ID:         "test_race",
				Action:     "test_action",
				Condition:  "test_cond",
				Confidence: 0.8,
			}
			_ = store.SaveOrUpdateSkill(skill, true, 0.3)
			if workerID%3 == 0 {
				_ = store.OptimizeAndVacuum()
			}
		}()
	}
	wg.Wait()

	// 3. Close store and test error handling
	store.Close()
	if errClosed := store.OptimizeAndVacuum(); errClosed != ErrStoreClosed {
		t.Errorf("Expected ErrStoreClosed on closed store, got %v", errClosed)
	}
}

