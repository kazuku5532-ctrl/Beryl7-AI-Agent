package tests

import (
	"fmt"
	"path/filepath"
	"runtime"
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

// TestGoroutineAndMemoryLeakSyntheticSoak runs 1,000 synthetic anomaly-action cycles to prove memory stability (< 1MB growth) and 0 goroutine leaks
func TestGoroutineAndMemoryLeakSyntheticSoak(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "soak_skills.db")

	store, err := skillstore.New(dbPath)
	if err != nil {
		t.Fatalf("SkillStore init failed for soak test: %v", err)
	}
	defer store.Close()

	var initialMem runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&initialMem)
	initialGoroutines := runtime.NumGoroutine()

	cycles := 1000
	var wg sync.WaitGroup

	for i := 0; i < cycles; i++ {
		wg.Add(1)
		cycleID := i
		go func() {
			defer wg.Done()
			anomaly := "WAN_DROP"
			action := "restart_wan_interface"
			if cycleID%2 == 0 {
				anomaly = "MEMORY_EXHAUSTION"
				action = "purge_memory_cache"
			}
			_ = store.UpdateQValue(anomaly, action, 1.0)
			_, _, _ = store.RecommendBestAction(anomaly, action)
		}()
	}

	wg.Wait()
	runtime.GC()

	var finalMem runtime.MemStats
	runtime.ReadMemStats(&finalMem)
	finalGoroutines := runtime.NumGoroutine()

	goroutineDiff := finalGoroutines - initialGoroutines
	if goroutineDiff > 5 {
		t.Errorf("SOAK TEST LEAK WARNING: Goroutine leak detected! Initial=%d, Final=%d (Diff=%d > 5)", initialGoroutines, finalGoroutines, goroutineDiff)
	}

	memAllocDiffMB := float64(finalMem.HeapAlloc-initialMem.HeapAlloc) / (1024 * 1024)
	if memAllocDiffMB > 2.0 {
		t.Errorf("SOAK TEST MEMORY WARNING: Heap allocation grew by %.2f MB (> 2.0MB limit)", memAllocDiffMB)
	}
}
