package tests

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"beryl7-agent/notifier"
	"beryl7-agent/skillstore"
)

// TestAdversarial_TelegramReadiness_ConcurrentDispatchZeroDuplicate verifies that even when
// multiple maintenance routines trigger simultaneously, the one-shot latch guarantees zero duplicate sends.
func TestAdversarial_TelegramReadiness_ConcurrentDispatchZeroDuplicate(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "readiness_race.db")

	store, err := skillstore.New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create skillstore: %v", err)
	}
	defer store.Close()

	var telegramHitCount atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		telegramHitCount.Add(1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok": true}`))
	}))
	defer server.Close()

	n := notifier.NewTelegramNotifier("dummy-token", "123456", false)
	n.SetBaseURL(server.URL)
	n.SetHTTPClient(server.Client())

	ctx := context.Background()
	latchKey := "telemetry_14d_readiness_notified"
	concurrentWorkers := 30
	var wg sync.WaitGroup
	startBarrier := make(chan struct{})

	for i := 0; i < concurrentWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-startBarrier

			// Strict one-shot latch check-and-set
			isFirstTime, errLatch := store.CheckAndSetMilestoneLatch(latchKey)
			if errLatch == nil && isFirstTime {
				_ = n.SendTelemetryReadinessAlert(ctx, 1700000000, 1700000000+14*86400, 5000)
			}
		}()
	}

	close(startBarrier)
	wg.Wait()

	if telegramHitCount.Load() != 1 {
		t.Fatalf("DUPLICATE SEND DETECTED: Telegram alert was dispatched %d times, expected exactly 1", telegramHitCount.Load())
	}

	isSet, errFinal := store.IsMilestoneLatchSet(latchKey)
	if errFinal != nil || !isSet {
		t.Fatalf("Expected milestone latch to be true, got %v (err: %v)", isSet, errFinal)
	}
}

// TestAdversarial_MainMonitoringLoop_DiskPressureResilience verifies that when SQLite
// encounters disk pressure (closed DB handle), the agent monitoring cycle logs warning and survives without panics.
func TestAdversarial_MainMonitoringLoop_DiskPressureResilience(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "disk_pressure_loop.db")

	store, err := skillstore.New(dbPath)
	if err != nil {
		t.Fatalf("Failed to init store: %v", err)
	}

	// Close store to simulate catastrophic disk exhaustion or permission revocation
	_ = store.Close()

	ctx := context.Background()

	// Simulate 100 monitoring cycles attempting to record telemetry on failed store
	for i := 0; i < 100; i++ {
		rec := skillstore.TelemetryRecord{
			Timestamp:    time.Now().Unix(),
			RAMPct:       90.0,
			CPUPct:       25.0,
			LatencyMs:    30.0,
			TempC:        55.0,
			WANOffline:   false,
			WiFiFail:     false,
			ActiveIntent: "default",
		}

		// Must handle error gracefully without panicking
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("FATAL PANIC during disk pressure cycle %d: %v", i, r)
				}
			}()

			errWrite := store.RecordTelemetryHistory(ctx, rec)
			if errWrite == nil {
				t.Errorf("Expected write error on closed DB, got nil")
			}
		}()
	}
}

// TestAdversarial_TelemetryStore_RetentionBoundaryAndClockDrift tests pruning behavior with
// historical records spanning past cutoff, recent records, and NTP clock drift into the future.
func TestAdversarial_TelemetryStore_RetentionBoundaryAndClockDrift(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "retention_drift.db")

	store, err := skillstore.New(dbPath)
	if err != nil {
		t.Fatalf("Failed to init store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	now := time.Now()

	// 1. Records 60 days old (should be pruned)
	for i := 0; i < 10; i++ {
		_ = store.RecordTelemetryHistory(ctx, skillstore.TelemetryRecord{
			Timestamp:    now.AddDate(0, 0, -60).Unix() + int64(i),
			RAMPct:       50.0,
			ActiveIntent: "ancient",
		})
	}

	// 2. Records 40 days old (should be pruned with 30-day retention)
	for i := 0; i < 10; i++ {
		_ = store.RecordTelemetryHistory(ctx, skillstore.TelemetryRecord{
			Timestamp:    now.AddDate(0, 0, -40).Unix() + int64(i),
			RAMPct:       55.0,
			ActiveIntent: "old",
		})
	}

	// 3. Records 10 days old (should be preserved)
	for i := 0; i < 10; i++ {
		_ = store.RecordTelemetryHistory(ctx, skillstore.TelemetryRecord{
			Timestamp:    now.AddDate(0, 0, -10).Unix() + int64(i),
			RAMPct:       60.0,
			ActiveIntent: "recent",
		})
	}

	// 4. Records in the future due to NTP clock skew (should be preserved)
	for i := 0; i < 5; i++ {
		_ = store.RecordTelemetryHistory(ctx, skillstore.TelemetryRecord{
			Timestamp:    now.AddDate(0, 0, 2).Unix() + int64(i),
			RAMPct:       65.0,
			ActiveIntent: "future_drift",
		})
	}

	// Initial count: 10 + 10 + 10 + 5 = 35
	statsBefore, errStats := store.GetTelemetryHistoryStats(ctx)
	if errStats != nil || statsBefore.TotalRecords != 35 {
		t.Fatalf("Expected 35 records before prune, got %d (err: %v)", statsBefore.TotalRecords, errStats)
	}

	// Prune older than 30 days -> should delete 20 records (60d + 40d)
	pruned, errPrune := store.PruneTelemetryHistory(ctx, 30)
	if errPrune != nil {
		t.Fatalf("PruneTelemetryHistory failed: %v", errPrune)
	}
	if pruned != 20 {
		t.Errorf("Expected 20 rows pruned, got %d", pruned)
	}

	// Records left: 10 recent + 5 future = 15
	statsAfter, _ := store.GetTelemetryHistoryStats(ctx)
	if statsAfter.TotalRecords != 15 {
		t.Errorf("Expected 15 records remaining after prune, got %d", statsAfter.TotalRecords)
	}

	remaining, errRemaining := store.GetTelemetryHistory(ctx, 0, 100)
	if errRemaining != nil {
		t.Fatalf("GetTelemetryHistory failed: %v", errRemaining)
	}
	for _, r := range remaining {
		if r.ActiveIntent == "ancient" || r.ActiveIntent == "old" {
			t.Errorf("Found stale unpruned record: %+v", r)
		}
	}
}
