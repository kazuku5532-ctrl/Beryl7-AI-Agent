package tests

import (
	"os"
	"testing"
	"time"

	"beryl7-agent/skillstore"
)

func TestSkillStoreSaveAndRetrieve(t *testing.T) {
	tmpDB := "test_skills.db"
	defer os.Remove(tmpDB)

	store, err := skillstore.New(tmpDB)
	if err != nil {
		t.Fatalf("Failed to init skillstore: %v", err)
	}
	defer store.Close()

	sk := &skillstore.Skill{
		ID:         "restart_wan_interface",
		Action:     "restart_wan_interface",
		Condition:  "WAN_DROP",
		Confidence: 0.5,
		CreatedAt:  time.Now(),
	}

	err = store.SaveOrUpdateSkill(sk, true, 0.3)
	if err != nil {
		t.Fatalf("Failed to save skill: %v", err)
	}

	retrieved := store.GetSkill("restart_wan_interface")
	if retrieved == nil {
		t.Fatalf("Retrieved skill is nil")
	}

	if retrieved.Confidence <= 0.5 {
		t.Errorf("Expected confidence > 0.5 after success, got %.2f", retrieved.Confidence)
	}
}
