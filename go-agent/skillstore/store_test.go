package skillstore

import (
	"os"
	"path/filepath"
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
	backupPath := dbPath + ".bak"
	_ = os.Remove(backupPath)
}
