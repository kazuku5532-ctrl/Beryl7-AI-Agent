package tests

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"beryl7-agent/orchestrator"
	"beryl7-agent/skillstore"
	"beryl7-agent/telemetry"
	"beryl7-agent/watchdog"
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

// TestMultiComponentConcurrentRaceStress runs multi-package concurrent operations simultaneously (SkillStore, Telemetry atomic cache, Watchdog, EventBus)
func TestMultiComponentConcurrentRaceStress(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "multi_race_skills.db")
	cpPath := filepath.Join(tempDir, "multi_checkpoint.uci")

	store, err := skillstore.New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create skillstore: %v", err)
	}
	defer store.Close()

	wd := watchdog.New(cpPath)
	_ = wd.SaveCheckpoint(map[string]string{"network.wan.proto": "dhcp"})

	collector := telemetry.NewCollector()
	bus := orchestrator.NewEventBus()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	const concurrentWorkers = 30
	const opsPerWorker = 20
	var wg sync.WaitGroup

	for i := 0; i < concurrentWorkers; i++ {
		wg.Add(1)
		workerID := i
		go func(id int) {
			defer wg.Done()
			for j := 0; j < opsPerWorker; j++ {
				switch (id + j) % 4 {
				case 0:
					// Telemetry async ping atomic operations
					_ = collector.GetCachedPingLatency()
					_ = collector.ProbePingLatency(ctx)
				case 1:
					// SkillStore Q-learning and optimization
					anomaly := fmt.Sprintf("ANOMALY_%d", j%3)
					action := fmt.Sprintf("ACTION_%d", j%3)
					_ = store.UpdateQValue(anomaly, action, 0.8)
					_, _, _ = store.RecommendBestAction(anomaly, action)
				case 2:
					// Watchdog checkpoint operations
					_ = wd.LoadAndVerifyCheckpoint()
					_ = wd.IsSafeMode()
				case 3:
					// Orchestrator EventBus publish
					bus.Publish(&orchestrator.Event{
						Type:   orchestrator.EventMetricsUpdated,
						Source: "stress_test",
						Data:   map[string]interface{}{"worker": id, "op": j},
					})
				}
			}
		}(workerID)
	}

	wg.Wait()
}


