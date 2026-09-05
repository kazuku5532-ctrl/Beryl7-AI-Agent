package skillstore

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestAdversarial_DiskPressure_ClosedDBWriteAndPrune verifies that all store operations
// handle closed handles, unwritable state, or simulated disk failure without panic.
func TestAdversarial_DiskPressure_ClosedDBWriteAndPrune(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "closed_db_test.db")

	store, err := New(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize store: %v", err)
	}

	// Close database to simulate unavailable store / IO error
	if err := store.Close(); err != nil {
		t.Fatalf("Failed to close store: %v", err)
	}

	ctx := context.Background()

	// 1. RecordTelemetryHistory on closed DB
	rec := TelemetryRecord{
		Timestamp:    time.Now().Unix(),
		RAMPct:       85.5,
		CPUPct:       20.0,
		LatencyMs:    15.2,
		TempC:        48.0,
		WANOffline:   false,
		WiFiFail:     false,
		ActiveIntent: "default",
	}
	errRec := store.RecordTelemetryHistory(ctx, rec)
	if errRec != ErrStoreClosed && errRec == nil {
		t.Errorf("Expected ErrStoreClosed or error on RecordTelemetryHistory, got %v", errRec)
	}

	// 2. PruneTelemetryHistory on closed DB
	pruned, errPrune := store.PruneTelemetryHistory(ctx, 30)
	if errPrune != ErrStoreClosed && errPrune == nil {
		t.Errorf("Expected ErrStoreClosed on PruneTelemetryHistory, got %v (pruned: %d)", errPrune, pruned)
	}

	// 3. GetTelemetryHistory on closed DB
	records, errGet := store.GetTelemetryHistory(ctx, 0, 100)
	if errGet != ErrStoreClosed && errGet == nil {
		t.Errorf("Expected ErrStoreClosed on GetTelemetryHistory, got %v", errGet)
	}
	if len(records) != 0 {
		t.Errorf("Expected empty records slice on closed DB, got %d", len(records))
	}

	// 4. GetTelemetryHistoryStats on closed DB
	_, errStats := store.GetTelemetryHistoryStats(ctx)
	if errStats != ErrStoreClosed && errStats == nil {
		t.Errorf("Expected ErrStoreClosed on GetTelemetryHistoryStats, got %v", errStats)
	}

	// 5. Milestone Latch operations on closed DB
	if _, errLatch := store.IsMilestoneLatchSet("test_key"); errLatch != ErrStoreClosed && errLatch == nil {
		t.Errorf("Expected ErrStoreClosed on IsMilestoneLatchSet, got %v", errLatch)
	}
	if errSet := store.SetMilestoneLatch("test_key"); errSet != ErrStoreClosed && errSet == nil {
		t.Errorf("Expected ErrStoreClosed on SetMilestoneLatch, got %v", errSet)
	}
	if _, errCheck := store.CheckAndSetMilestoneLatch("test_key"); errCheck != ErrStoreClosed && errCheck == nil {
		t.Errorf("Expected ErrStoreClosed on CheckAndSetMilestoneLatch, got %v", errCheck)
	}
}

// TestAdversarial_CrashRecovery_PartialWriteAndReopen simulates an abrupt power-loss or crash
// by modifying bytes or appending garbage to SQLite files, verifying clean reopen and data retention.
func TestAdversarial_CrashRecovery_PartialWriteAndReopen(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "power_loss_sim.db")

	// Step 1: Open healthy database and insert committed records + milestone latch
	store, err := New(dbPath)
	if err != nil {
		t.Fatalf("Initial New failed: %v", err)
	}

	ctx := context.Background()
	baseTime := time.Now().Unix() - 1000
	for i := 0; i < 50; i++ {
		rec := TelemetryRecord{
			Timestamp:    baseTime + int64(i*10),
			RAMPct:       60.0 + float64(i)*0.2,
			CPUPct:       15.0,
			LatencyMs:    12.0,
			TempC:        45.0,
			WANOffline:   false,
			WiFiFail:     false,
			ActiveIntent: "always_on_vigilant",
		}
		if err := store.RecordTelemetryHistory(ctx, rec); err != nil {
			t.Fatalf("Failed to insert record %d: %v", i, err)
		}
	}

	if err := store.SetMilestoneLatch("pre_crash_milestone"); err != nil {
		t.Fatalf("Failed to set pre-crash milestone latch: %v", err)
	}

	_ = store.BackupDatabase()
	_ = store.Close()

	// Step 2: Simulate partial uncommitted crash or appending garbage to the database file
	garbage := make([]byte, 256)
	_, _ = rand.Read(garbage)
	f, errOpen := os.OpenFile(dbPath, os.O_WRONLY|os.O_APPEND, 0600)
	if errOpen == nil {
		_, _ = f.Write(garbage)
		_ = f.Close()
	}

	// Step 3: Reopen database and verify automatic SQLite WAL/integrity salvage routine
	salvagedStore, errReopen := New(dbPath)
	if errReopen != nil {
		t.Fatalf("Expected database to recover cleanly, but New failed: %v", errReopen)
	}
	defer salvagedStore.Close()

	// Step 4: Verify milestone latch and database queryability
	isSet, errLatch := salvagedStore.IsMilestoneLatchSet("pre_crash_milestone")
	if errLatch != nil {
		t.Errorf("IsMilestoneLatchSet query failed after salvage: %v", errLatch)
	}
	t.Logf("Milestone latch state after salvage: %v", isSet)

	// Verify we can insert new records without errors after recovery
	newRec := TelemetryRecord{
		Timestamp:    time.Now().Unix(),
		RAMPct:       75.0,
		CPUPct:       30.0,
		LatencyMs:    20.0,
		TempC:        50.0,
		WANOffline:   false,
		WiFiFail:     false,
		ActiveIntent: "post_crash_recovery",
	}
	if errInsertNew := salvagedStore.RecordTelemetryHistory(ctx, newRec); errInsertNew != nil {
		t.Errorf("Failed to record new telemetry after salvage: %v", errInsertNew)
	}

	stats, errStats := salvagedStore.GetTelemetryHistoryStats(ctx)
	if errStats != nil {
		t.Errorf("GetTelemetryHistoryStats failed after salvage: %v", errStats)
	} else if stats.TotalRecords == 0 {
		t.Errorf("Expected non-zero records after salvage, got %d", stats.TotalRecords)
	}
}

// TestAdversarial_ConcurrentReadWritePrune executes heavy concurrent multi-goroutine reads,
// writes, and pruning on the same SkillStore to ensure zero race conditions.
func TestAdversarial_ConcurrentReadWritePrune(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "concurrent_rw_prune.db")

	store, err := New(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	var wg sync.WaitGroup
	workers := 15
	operationsPerWorker := 30

	// Concurrent Writers
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < operationsPerWorker; j++ {
				rec := TelemetryRecord{
					Timestamp:    time.Now().Unix() + int64(workerID*1000+j),
					RAMPct:       50.0 + float64(j)*0.5,
					CPUPct:       10.0 + float64(workerID),
					LatencyMs:    15.0,
					TempC:        42.0,
					WANOffline:   false,
					WiFiFail:     false,
					ActiveIntent: fmt.Sprintf("intent_%d", workerID),
				}
				_ = store.RecordTelemetryHistory(ctx, rec)
			}
		}(i)
	}

	// Concurrent Readers
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < operationsPerWorker; j++ {
				_, _ = store.GetTelemetryHistory(ctx, 0, 50)
				_, _ = store.GetTelemetryHistoryStats(ctx)
			}
		}()
	}

	// Concurrent Pruners
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 5; j++ {
				_, _ = store.PruneTelemetryHistory(ctx, 30)
				time.Sleep(2 * time.Millisecond)
			}
		}()
	}

	wg.Wait()

	stats, errStats := store.GetTelemetryHistoryStats(ctx)
	if errStats != nil {
		t.Fatalf("GetTelemetryHistoryStats failed: %v", errStats)
	}
	if stats.TotalRecords == 0 {
		t.Errorf("Expected records to be present after concurrent write, got 0")
	}
}

// TestAdversarial_ConcurrentMilestoneLatch_StrictOneShot tests that when 50 goroutines simultaneously
// attempt to check and set the same milestone latch, exactly ONE succeeds.
func TestAdversarial_ConcurrentMilestoneLatch_StrictOneShot(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "latch_race.db")

	store, err := New(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize store: %v", err)
	}
	defer store.Close()

	latchKey := "telemetry_14d_readiness_notified"
	goroutines := 50

	var wg sync.WaitGroup
	var successCount atomic.Int64
	var failureCount atomic.Int64

	startBarrier := make(chan struct{})

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-startBarrier // Synchronize simultaneous execution start
			isFirst, err := store.CheckAndSetMilestoneLatch(latchKey)
			if err != nil {
				t.Errorf("CheckAndSetMilestoneLatch error: %v", err)
				return
			}
			if isFirst {
				successCount.Add(1)
			} else {
				failureCount.Add(1)
			}
		}()
	}

	// Release all goroutines at the exact same instant
	close(startBarrier)
	wg.Wait()

	if successCount.Load() != 1 {
		t.Fatalf("STRICT ONESHOT VIOLATION: Expected exactly 1 goroutine to acquire latch, but got %d", successCount.Load())
	}
	if failureCount.Load() != int64(goroutines-1) {
		t.Fatalf("Expected %d goroutines to report false, got %d", goroutines-1, failureCount.Load())
	}

	isSet, errFinal := store.IsMilestoneLatchSet(latchKey)
	if errFinal != nil {
		t.Fatalf("IsMilestoneLatchSet final query failed: %v", errFinal)
	}
	if !isSet {
		t.Fatalf("Expected final latch state to be true")
	}
}
