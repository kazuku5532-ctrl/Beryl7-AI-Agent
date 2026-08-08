package tests

import (
	"path/filepath"
	"testing"
	"time"

	"beryl7-agent/skillstore"
)

func TestSkillStoreSaveAndRetrieve(t *testing.T) {
	tmpDB := filepath.Join(t.TempDir(), "test_skills.db")

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

	retrieved := store.GetSkill("WAN_DROP", "restart_wan_interface")
	if retrieved == nil {
		t.Fatalf("Retrieved skill is nil")
	}

	if retrieved.Confidence <= 0.5 {
		t.Errorf("Expected confidence > 0.5 after success, got %.2f", retrieved.Confidence)
	}
}

// TestQLearningUpdateAndRecommendBestAction verifies Q-Learning V2 UpdateQValue, Bellman updates, bound clamping [-0.8, 1.0], and Cold Start handling
func TestQLearningUpdateAndRecommendBestAction(t *testing.T) {
	tmpDB := filepath.Join(t.TempDir(), "test_qlearning.db")

	store, err := skillstore.New(tmpDB)
	if err != nil {
		t.Fatalf("Failed to init skillstore: %v", err)
	}
	defer store.Close()

	// 1. Cold Start Test: State has no Q-Table entry -> should return defaultAction with 0.0 Q-value without error
	action, qval, err := store.RecommendBestAction("UNKNOWN_ANOMALY", "fallback_action")
	if err != nil {
		t.Fatalf("Cold start returned unexpected error: %v", err)
	}
	if action != "fallback_action" || qval != 0.0 {
		t.Errorf("Expected Cold Start fallback 'fallback_action' and 0.0 qval, got '%s', %f", action, qval)
	}

	// 2. Success Reward Test (+1.0)
	if err := store.UpdateQValue("WAN_DROP", "restart_wan_interface", 1.0); err != nil {
		t.Fatalf("UpdateQValue success reward failed: %v", err)
	}

	bestAction, bestQ, err := store.RecommendBestAction("WAN_DROP", "default_act")
	if err != nil {
		t.Fatalf("RecommendBestAction failed after update: %v", err)
	}
	if bestAction != "restart_wan_interface" {
		t.Errorf("Expected bestAction 'restart_wan_interface', got '%s'", bestAction)
	}
	if bestQ <= 0.0 {
		t.Errorf("Expected positive Q-value after +1.0 reward, got %f", bestQ)
	}

	// 3. Clamping Floor Test (-0.8 Floor Bound)
	// Repeat penalties 20 times to ensure Q-value is clamped at -0.8 floor bound
	for i := 0; i < 20; i++ {
		_ = store.UpdateQValue("LATENCY_SPIKE", "bad_action", -1.0)
	}
	_, badQ, err := store.RecommendBestAction("LATENCY_SPIKE", "default_act")
	if err != nil {
		t.Fatalf("RecommendBestAction failed: %v", err)
	}
	if badQ < -0.8 {
		t.Errorf("Expected Q-value clamped at -0.8 floor, got %f", badQ)
	}

	// 4. Clamping Ceiling Test (1.0 Ceiling Bound)
	for i := 0; i < 20; i++ {
		_ = store.UpdateQValue("HIGH_MEMORY", "purge_cache", 1.0)
	}
	_, topQ, err := store.RecommendBestAction("HIGH_MEMORY", "default_act")
	if err != nil {
		t.Fatalf("RecommendBestAction failed: %v", err)
	}
	if topQ > 1.0 {
		t.Errorf("Expected Q-value clamped at 1.0 ceiling, got %f", topQ)
	}
}
