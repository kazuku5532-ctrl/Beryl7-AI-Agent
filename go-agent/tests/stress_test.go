package tests

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"beryl7-agent/skillstore"
)

func TestSkillStoreHighConcurrencyStress(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "stress_skills.db")

	store, err := skillstore.New(dbPath)
	if err != nil {
		t.Fatalf("SkillStore init failed for stress test: %v", err)
	}
	defer store.Close()

	workers := 20
	opsPerWorker := 20
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		workerID := i
		go func() {
			defer wg.Done()
			for j := 0; j < opsPerWorker; j++ {
				condition := fmt.Sprintf("ANOMALY_%d", (workerID+j)%5)
				action := fmt.Sprintf("ACTION_%d_%d", workerID, j)
				skillID := fmt.Sprintf("skill_%d_%d", workerID, j)
				skill := &skillstore.Skill{
					ID:           skillID,
					Action:       action,
					Condition:    condition,
					Confidence:   0.9,
					SuccessCount: 1,
					FailureCount: 0,
				}
				_ = store.SaveOrUpdateSkill(skill, true, 0.3)
				_ = store.GetSkill(condition, action)
			}
		}()
	}

	wg.Wait()

	checkSkill := store.GetSkill("ANOMALY_0", "ACTION_0_0")
	if checkSkill == nil {
		t.Errorf("SkillStore GetSkill returned nil after high concurrency stress test")
	}
}
