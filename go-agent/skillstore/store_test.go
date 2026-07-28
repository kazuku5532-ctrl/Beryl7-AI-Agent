package skillstore

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitStore(t *testing.T) {
	dbPath := filepath.Join(os.TempDir(), "test_skills.db")
	_ = os.Remove(dbPath)
	defer func() { _ = os.Remove(dbPath) }()

	store, err := New(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize SkillStore: %v", err)
	}
	defer func() { _ = store.Close() }()

	skill := &Skill{
		ID:         "WAN_DROP:restart_wan",
		Action:     "restart_wan",
		Condition:  "WAN_DROP",
		Confidence: 0.95,
	}

	err = store.SaveOrUpdateSkill(skill, true, 0.2)
	if err != nil {
		t.Fatalf("Failed to save skill: %v", err)
	}

	retrieved := store.GetSkill("WAN_DROP", "restart_wan")
	if retrieved == nil {
		t.Fatalf("Failed to retrieve skill")
	}

	if retrieved.Action != "restart_wan" {
		t.Errorf("Expected action restart_wan, got %s", retrieved.Action)
	}
}
