package skillstore

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
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

func TestSkillStore_OperationalMetrics_SixFields(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "metrics_skills.db")

	store, err := New(dbPath)
	if err != nil {
		t.Fatalf("Failed to init store: %v", err)
	}
	defer store.Close()

	store.UpdateQValue("state-1", "action-1", 1.0)
	store.UpdateQValue("state-2", "action-2", -0.5)
	store.UpdateQValue("state-3", "action-3", 0.0)

	store.RecommendBestAction("state-1", "default")
	store.RecommendBestAction("state-unknown", "default")

	store.RecommendBestActionWithInterpolation("state-1", nil, "default")
	store.RecommendBestActionWithInterpolation("state-2", nil, "default")

	sig1 := &StateSignature{StateName: "state-1", RAMPct: 100.0, LatencyMs: 50.0}
	store.RecordStateSignature(sig1)
	sig2 := &StateSignature{StateName: "state-new", RAMPct: 105.0, LatencyMs: 55.0}
	store.RecommendBestActionWithInterpolation("state-new", sig2, "default")

	sig3 := &StateSignature{StateName: "state-2", RAMPct: 200.0, LatencyMs: 80.0}
	store.RecordStateSignature(sig3)
	sig4 := &StateSignature{StateName: "state-new2", RAMPct: 205.0, LatencyMs: 85.0}
	store.RecommendBestActionWithInterpolation("state-new2", sig4, "default")

	store.RecommendBestActionWithInterpolation("state-completely-new", nil, "default")

	m := store.GetOperationalMetrics()
	
	if m.TotalQUpdates != 3 { t.Errorf("Expected 3 TotalQUpdates, got %d", m.TotalQUpdates) }
	if m.VerifiedSuccesses != 1 { t.Errorf("Expected 1 VerifiedSuccesses, got %d", m.VerifiedSuccesses) }
	if m.VerifiedFailures != 1 { t.Errorf("Expected 1 VerifiedFailures, got %d", m.VerifiedFailures) }
	if m.ExactMatchCount != 2 { t.Errorf("Expected 2 ExactMatchCount, got %d", m.ExactMatchCount) }
	if m.FallbackDefaultCount != 4 { t.Errorf("Expected 4 FallbackDefaultCount, got %d", m.FallbackDefaultCount) }
	if m.Interpolations != 1 { t.Errorf("Expected 1 Interpolations, got %d", m.Interpolations) }
}

func TestTelemetryHistory_RecordAndQuery(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "telemetry_test.db")

	store, err := New(dbPath)
	if err != nil {
		t.Fatalf("Failed to init store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	now := time.Now().Unix()

	rec1 := TelemetryRecord{
		Timestamp:    now - 3600,
		RAMPct:       45.5,
		LatencyMs:    12.3,
		CPUPct:       20.1,
		TempC:        52.0,
		WANOffline:   false,
		WiFiFail:     false,
		ActiveIntent: "default",
	}
	rec2 := TelemetryRecord{
		Timestamp:    now - 1800,
		RAMPct:       88.2,
		LatencyMs:    150.7,
		CPUPct:       75.4,
		TempC:        63.5,
		WANOffline:   true,
		WiFiFail:     true,
		ActiveIntent: "gaming",
	}
	rec3 := TelemetryRecord{
		Timestamp:    now,
		RAMPct:       50.0,
		LatencyMs:    15.0,
		CPUPct:       25.0,
		TempC:        55.0,
		WANOffline:   false,
		WiFiFail:     false,
		ActiveIntent: "eco",
	}

	if err := store.RecordTelemetryHistory(ctx, rec1); err != nil {
		t.Fatalf("Failed to record telemetry 1: %v", err)
	}
	if err := store.RecordTelemetryHistory(ctx, rec2); err != nil {
		t.Fatalf("Failed to record telemetry 2: %v", err)
	}
	if err := store.RecordTelemetryHistory(ctx, rec3); err != nil {
		t.Fatalf("Failed to record telemetry 3: %v", err)
	}

	// Query all 3 records since 2 hours ago
	records, err := store.GetTelemetryHistory(ctx, now-7200, 100)
	if err != nil {
		t.Fatalf("Failed to get telemetry history: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("Expected 3 records, got %d", len(records))
	}

	// Verify chronological ASC order and field accuracy
	if records[0].Timestamp != rec1.Timestamp || records[0].RAMPct != rec1.RAMPct || records[0].WANOffline != false {
		t.Errorf("Record 1 mismatch: %+v", records[0])
	}
	if records[1].Timestamp != rec2.Timestamp || records[1].WANOffline != true || records[1].WiFiFail != true || records[1].ActiveIntent != "gaming" {
		t.Errorf("Record 2 mismatch: %+v", records[1])
	}
	if records[2].Timestamp != rec3.Timestamp || records[2].ActiveIntent != "eco" {
		t.Errorf("Record 3 mismatch: %+v", records[2])
	}

	// Query partial window (since 20 minutes ago) -> should only return rec3
	partial, err := store.GetTelemetryHistory(ctx, now-1200, 100)
	if err != nil {
		t.Fatalf("Failed partial query: %v", err)
	}
	if len(partial) != 1 {
		t.Fatalf("Expected 1 partial record, got %d", len(partial))
	}
	if partial[0].Timestamp != rec3.Timestamp {
		t.Errorf("Expected record 3, got record timestamp %d", partial[0].Timestamp)
	}

	// Test limit bounding
	limited, err := store.GetTelemetryHistory(ctx, now-7200, 2)
	if err != nil {
		t.Fatalf("Failed limited query: %v", err)
	}
	if len(limited) != 2 {
		t.Fatalf("Expected 2 limited records, got %d", len(limited))
	}
}

func TestTelemetryHistory_Prune(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "prune_test.db")

	store, err := New(dbPath)
	if err != nil {
		t.Fatalf("Failed to init store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	now := time.Now()

	// Old record: 45 days ago (should be pruned with 30-day retention)
	oldRec := TelemetryRecord{
		Timestamp:    now.AddDate(0, 0, -45).Unix(),
		RAMPct:       40.0,
		LatencyMs:    10.0,
		CPUPct:       10.0,
		TempC:        45.0,
		ActiveIntent: "default",
	}
	// Mid record: 15 days ago (should NOT be pruned with 30-day retention)
	midRec := TelemetryRecord{
		Timestamp:    now.AddDate(0, 0, -15).Unix(),
		RAMPct:       50.0,
		LatencyMs:    15.0,
		CPUPct:       20.0,
		TempC:        50.0,
		ActiveIntent: "default",
	}
	// Recent record: today (should NOT be pruned)
	recentRec := TelemetryRecord{
		Timestamp:    now.Unix(),
		RAMPct:       60.0,
		LatencyMs:    20.0,
		CPUPct:       30.0,
		TempC:        55.0,
		ActiveIntent: "streaming",
	}

	_ = store.RecordTelemetryHistory(ctx, oldRec)
	_ = store.RecordTelemetryHistory(ctx, midRec)
	_ = store.RecordTelemetryHistory(ctx, recentRec)

	deleted, err := store.PruneTelemetryHistory(ctx, 30)
	if err != nil {
		t.Fatalf("PruneTelemetryHistory failed: %v", err)
	}
	if deleted != 1 {
		t.Errorf("Expected 1 pruned record, got %d", deleted)
	}

	remaining, err := store.GetTelemetryHistory(ctx, 0, 100)
	if err != nil {
		t.Fatalf("Failed to get remaining records: %v", err)
	}
	if len(remaining) != 2 {
		t.Fatalf("Expected 2 remaining records, got %d", len(remaining))
	}
	if remaining[0].Timestamp != midRec.Timestamp || remaining[1].Timestamp != recentRec.Timestamp {
		t.Errorf("Remaining records do not match expected timestamps: %+v", remaining)
	}
}

func TestTelemetryHistory_EmptyDB(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "empty_test.db")

	store, err := New(dbPath)
	if err != nil {
		t.Fatalf("Failed to init store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	records, err := store.GetTelemetryHistory(ctx, 0, 100)
	if err != nil {
		t.Fatalf("Query on empty DB returned error: %v", err)
	}
	if records == nil {
		t.Fatalf("Expected non-nil slice on empty DB, got nil")
	}
	if len(records) != 0 {
		t.Fatalf("Expected 0 records on empty DB, got %d", len(records))
	}
}

func TestTelemetryHistory_Stats(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "stats_test.db")

	store, err := New(dbPath)
	if err != nil {
		t.Fatalf("Failed to init store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Stats on empty table
	emptyStats, err := store.GetTelemetryHistoryStats(ctx)
	if err != nil {
		t.Fatalf("GetTelemetryHistoryStats on empty table failed: %v", err)
	}
	if emptyStats.TotalRecords != 0 || emptyStats.EstimatedBytes != 0 || emptyStats.OldestUnix != 0 || emptyStats.NewestUnix != 0 {
		t.Errorf("Unexpected empty stats: %+v", emptyStats)
	}

	// Insert 5 records
	tBase := int64(1700000000)
	for i := 0; i < 5; i++ {
		rec := TelemetryRecord{
			Timestamp:    tBase + int64(i*60),
			RAMPct:       40.0 + float64(i),
			LatencyMs:    10.0 + float64(i),
			CPUPct:       15.0,
			TempC:        50.0,
			ActiveIntent: "default",
		}
		if err := store.RecordTelemetryHistory(ctx, rec); err != nil {
			t.Fatalf("Failed to record telemetry: %v", err)
		}
	}

	stats, err := store.GetTelemetryHistoryStats(ctx)
	if err != nil {
		t.Fatalf("GetTelemetryHistoryStats failed: %v", err)
	}
	if stats.TotalRecords != 5 {
		t.Errorf("Expected 5 TotalRecords, got %d", stats.TotalRecords)
	}
	if stats.OldestUnix != tBase {
		t.Errorf("Expected OldestUnix %d, got %d", tBase, stats.OldestUnix)
	}
	if stats.NewestUnix != tBase+240 {
		t.Errorf("Expected NewestUnix %d, got %d", tBase+240, stats.NewestUnix)
	}
	if stats.EstimatedBytes != 5*64 {
		t.Errorf("Expected EstimatedBytes %d, got %d", 5*64, stats.EstimatedBytes)
	}
}

func TestMilestoneLatch_SetAndQuery(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "latch_test.db")

	store, err := New(dbPath)
	if err != nil {
		t.Fatalf("Failed to init store: %v", err)
	}
	defer store.Close()

	key := "test_custom_latch"

	// 1. Check before setting
	isSet, err := store.IsMilestoneLatchSet(key)
	if err != nil {
		t.Fatalf("IsMilestoneLatchSet failed: %v", err)
	}
	if isSet {
		t.Errorf("Expected isSet=false before setting latch")
	}

	// 2. Set latch
	if err := store.SetMilestoneLatch(key); err != nil {
		t.Fatalf("SetMilestoneLatch failed: %v", err)
	}

	// 3. Check after setting
	isSetAfter, err := store.IsMilestoneLatchSet(key)
	if err != nil {
		t.Fatalf("IsMilestoneLatchSet after setting failed: %v", err)
	}
	if !isSetAfter {
		t.Errorf("Expected isSet=true after setting latch")
	}

	// 4. Overwrite latch (INSERT OR REPLACE)
	if err := store.SetMilestoneLatch(key); err != nil {
		t.Fatalf("SetMilestoneLatch overwrite failed: %v", err)
	}
	isSetAfterOverwrite, err := store.IsMilestoneLatchSet(key)
	if err != nil || !isSetAfterOverwrite {
		t.Errorf("Expected isSet=true after overwrite, got %v (err: %v)", isSetAfterOverwrite, err)
	}

	// 5. Check behavior when store is closed
	store.Close()
	if _, err := store.IsMilestoneLatchSet(key); err != ErrStoreClosed {
		t.Errorf("Expected ErrStoreClosed on IsMilestoneLatchSet, got %v", err)
	}
	if err := store.SetMilestoneLatch(key); err != ErrStoreClosed {
		t.Errorf("Expected ErrStoreClosed on SetMilestoneLatch, got %v", err)
	}
	if _, err := store.CheckAndSetMilestoneLatch(key); err != ErrStoreClosed {
		t.Errorf("Expected ErrStoreClosed on CheckAndSetMilestoneLatch, got %v", err)
	}
}
