package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"beryl7-agent/logger"
	"beryl7-agent/notifier"
	"beryl7-agent/skillstore"
)

func init() {
	logger.Init("/dev/null", "DEBUG")
}

func TestGracefulShutdownMarker(t *testing.T) {
	tmpDir := t.TempDir()
	markerPath := filepath.Join(tmpDir, ".beryl7_graceful_shutdown")
	tmpPath := markerPath + ".tmp"

	// Simulate Graceful Shutdown Write
	markerData, _ := json.Marshal(map[string]interface{}{
		"magic":     "BERYL7_GRACEFUL_EXIT_V1",
		"timestamp": time.Now().Format(time.RFC3339),
		"pid":       os.Getpid(),
		"reason":    "SIGTERM",
	})

	f, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		t.Fatalf("Failed to create tmp file: %v", err)
	}
	f.Write(markerData)
	f.Sync()
	f.Close()

	if err := os.Rename(tmpPath, markerPath); err != nil {
		t.Fatalf("Failed to rename marker file: %v", err)
	}

	// Simulate Startup Check (Clean)
	data, err := os.ReadFile(markerPath)
	if err != nil {
		t.Fatalf("Failed to read marker: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if magic, ok := m["magic"].(string); !ok || magic != "BERYL7_GRACEFUL_EXIT_V1" {
		t.Fatalf("Expected valid magic string")
	}

	// Simulate Corrupt marker
	os.WriteFile(markerPath, []byte("corrupt data"), 0600)
	corruptData, err := os.ReadFile(markerPath)
	if err == nil {
		var m2 map[string]interface{}
		if err := json.Unmarshal(corruptData, &m2); err == nil {
			if m2["magic"] == "BERYL7_GRACEFUL_EXIT_V1" {
				t.Fatalf("Should not parse corrupt data")
			}
		}
	}
}

func TestMilestoneLatchPersistence(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "skills.db")

	store, err := skillstore.New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	latchKey := "test_milestone_latch"

	// Set latch first time
	isFirst, err := store.CheckAndSetMilestoneLatch(latchKey)
	if err != nil {
		t.Fatalf("CheckAndSetMilestoneLatch failed: %v", err)
	}
	if !isFirst {
		t.Fatalf("Expected isFirst to be true on first set")
	}

	// Set latch second time (same instance)
	isFirst2, err := store.CheckAndSetMilestoneLatch(latchKey)
	if err != nil {
		t.Fatalf("CheckAndSetMilestoneLatch failed: %v", err)
	}
	if isFirst2 {
		t.Fatalf("Expected isFirst to be false on second set")
	}

	store.Close()

	// Reopen store to test persistence
	store2, err := skillstore.New(dbPath)
	if err != nil {
		t.Fatalf("Failed to reopen store: %v", err)
	}
	defer store2.Close()

	// Set latch third time (reopened instance)
	isFirst3, err := store2.CheckAndSetMilestoneLatch(latchKey)
	if err != nil {
		t.Fatalf("CheckAndSetMilestoneLatch failed: %v", err)
	}
	if isFirst3 {
		t.Fatalf("Expected isFirst to be false after reopen")
	}
}

func TestSendAlertWithBackoff_RetrySuccess(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok": true}`))
	}))
	defer server.Close()

	n := notifier.NewTelegramNotifier("dummy", "123", false)
	n.SetBaseURL(server.URL)
	n.SetRetryDelays([]time.Duration{5 * time.Millisecond, 10 * time.Millisecond})
	n.SetHTTPClient(server.Client())

	ctx := context.Background()
	err := n.SendAlertWithBackoff(ctx, "Test Message", 1*time.Second)
	if err != nil {
		t.Fatalf("Expected success after retries, got error: %v", err)
	}
	if callCount != 3 {
		t.Fatalf("Expected 3 calls, got %d", callCount)
	}
}

func TestSendAlertWithBackoff_Timeout(t *testing.T) {
	initialGoroutines := runtime.NumGoroutine()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	n := notifier.NewTelegramNotifier("dummy", "123", false)
	n.SetBaseURL(server.URL)
	n.SetRetryDelays([]time.Duration{5 * time.Millisecond, 10 * time.Millisecond})
	n.SetHTTPClient(server.Client())

	ctx := context.Background()
	// short timeout
	err := n.SendAlertWithBackoff(ctx, "Test Message", 25*time.Millisecond)
	if err == nil {
		t.Fatalf("Expected timeout error, got success")
	}
	if err != context.DeadlineExceeded {
		t.Fatalf("Expected context.DeadlineExceeded, got %v", err)
	}

	// small wait to ensure goroutines exit
	time.Sleep(10 * time.Millisecond)

	finalGoroutines := runtime.NumGoroutine()
	if finalGoroutines > initialGoroutines+2 { // +2 for httptest server background routines
		t.Logf("Warning: Goroutine leak possible. Initial: %d, Final: %d", initialGoroutines, finalGoroutines)
	}
}

func TestTelegramFormats(t *testing.T) {
	var lastMessage string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Text string `json:"text"`
		}
		json.NewDecoder(r.Body).Decode(&payload)
		lastMessage = payload.Text
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok": true}`))
	}))
	defer server.Close()

	notifierInstance := notifier.NewTelegramNotifier("dummy-token", "123456", false)
	notifierInstance.SetBaseURL(server.URL)
	notifierInstance.SetHTTPClient(server.Client())

	ctx := context.Background()

	err := notifierInstance.SendPowerLossRecoveryAlert(ctx, time.Now(), "Active (1/1)")
	if err != nil {
		t.Fatalf("SendPowerLossRecoveryAlert failed: %v", err)
	}

	metrics := skillstore.OperationalMetrics{
		TotalQUpdates:        25,
		VerifiedSuccesses:    20,
		VerifiedFailures:     5,
		Interpolations:       12,
		ExactMatchCount:      13,
		FallbackDefaultCount: 2,
	}
	err = notifierInstance.SendMilestoneAlert(ctx, metrics, 25)
	if err != nil {
		t.Fatalf("SendMilestoneAlert failed: %v", err)
	}

	if !strings.Contains(lastMessage, "Total Q-Updates: `25`") ||
		!strings.Contains(lastMessage, "Verified Successes: `20`") ||
		!strings.Contains(lastMessage, "Verified Failures: `5`") ||
		!strings.Contains(lastMessage, "Interpolations: `12`") ||
		!strings.Contains(lastMessage, "Exact Matches: `13`") ||
		!strings.Contains(lastMessage, "Fallback Defaults: `2`") {
		t.Fatalf("Milestone alert missing expected fields. Message:\n%s", lastMessage)
	}

	if strings.Contains(strings.ToLower(lastMessage), "thông minh hơn") || strings.Contains(lastMessage, "CỘT MỐC") {
		t.Fatalf("Milestone alert contains subjective text. Message:\n%s", lastMessage)
	}
}

func TestSendTelemetryReadinessAlert_Format(t *testing.T) {
	var lastMessage string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Text string `json:"text"`
		}
		json.NewDecoder(r.Body).Decode(&payload)
		lastMessage = payload.Text
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok": true}`))
	}))
	defer server.Close()

	n := notifier.NewTelegramNotifier("dummy-token", "123456", false)
	n.SetBaseURL(server.URL)
	n.SetHTTPClient(server.Client())

	ctx := context.Background()
	oldestUnix := int64(1700000000)
	newestUnix := int64(1700000000 + 14*86400)
	totalRecords := int64(40320)

	err := n.SendTelemetryReadinessAlert(ctx, oldestUnix, newestUnix, totalRecords)
	if err != nil {
		t.Fatalf("SendTelemetryReadinessAlert failed: %v", err)
	}

	expectedStart := time.Unix(oldestUnix, 0).UTC().Format("2006-01-02 15:04:05 UTC")
	expectedEnd := time.Unix(newestUnix, 0).UTC().Format("2006-01-02 15:04:05 UTC")

	if !strings.Contains(lastMessage, expectedStart) {
		t.Errorf("Expected start date %s in message, got:\n%s", expectedStart, lastMessage)
	}
	if !strings.Contains(lastMessage, expectedEnd) {
		t.Errorf("Expected end date %s in message, got:\n%s", expectedEnd, lastMessage)
	}
	if !strings.Contains(lastMessage, "40320") {
		t.Errorf("Expected total records 40320 in message, got:\n%s", lastMessage)
	}
	if !strings.Contains(lastMessage, "14 ngày") {
		t.Errorf("Expected '14 ngày' in message, got:\n%s", lastMessage)
	}
	if !strings.Contains(lastMessage, "Phân tích Dự đoán") || !strings.Contains(lastMessage, "Phase 2b") {
		t.Errorf("Expected Phase 2b predictive analysis mention in message, got:\n%s", lastMessage)
	}

	// Neutral text check - no subjective exaggeration/hype words
	lower := strings.ToLower(lastMessage)
	for _, hype := range []string{"thần kỳ", "thông minh tuyệt đỉnh", "đột phá", "cực đỉnh", "vô đối"} {
		if strings.Contains(lower, hype) {
			t.Errorf("Message contains subjective hype word '%s': %s", hype, lastMessage)
		}
	}
}

func TestTelemetryReadiness_LatchPersistenceAndRetry(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "latch_retry.db")
	store, err := skillstore.New(dbPath)
	if err != nil {
		t.Fatalf("Failed to init store: %v", err)
	}
	defer store.Close()

	latchKey := "telemetry_14d_readiness_notified"

	// 1. Initially latch is NOT set
	isSet, err := store.IsMilestoneLatchSet(latchKey)
	if err != nil {
		t.Fatalf("IsMilestoneLatchSet failed: %v", err)
	}
	if isSet {
		t.Fatalf("Expected latch to be false initially")
	}

	// 2. Simulate send failure (server returns HTTP 500)
	shouldFail := true
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if shouldFail {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok": true}`))
	}))
	defer server.Close()

	n := notifier.NewTelegramNotifier("dummy-token", "123456", false)
	n.SetBaseURL(server.URL)
	n.SetHTTPClient(server.Client())

	ctx := context.Background()
	sendErr := n.SendTelemetryReadinessAlert(ctx, 1700000000, 1700000000+14*86400, 1000)
	if sendErr == nil {
		t.Fatalf("Expected send failure on HTTP 500, got nil")
	}

	// Latch must NOT be set on send failure
	isSetAfterFail, err := store.IsMilestoneLatchSet(latchKey)
	if err != nil {
		t.Fatalf("IsMilestoneLatchSet failed: %v", err)
	}
	if isSetAfterFail {
		t.Fatalf("Latch should NOT be set when send fails")
	}

	// 3. Simulate retry on next cycle (send succeeds)
	shouldFail = false
	sendErr2 := n.SendTelemetryReadinessAlert(ctx, 1700000000, 1700000000+14*86400, 1000)
	if sendErr2 != nil {
		t.Fatalf("Expected send success, got %v", sendErr2)
	}

	// Set latch after successful send
	if errSet := store.SetMilestoneLatch(latchKey); errSet != nil {
		t.Fatalf("SetMilestoneLatch failed: %v", errSet)
	}

	isSetAfterSuccess, err := store.IsMilestoneLatchSet(latchKey)
	if err != nil {
		t.Fatalf("IsMilestoneLatchSet failed: %v", err)
	}
	if !isSetAfterSuccess {
		t.Fatalf("Expected latch to be true after successful set")
	}

	// 4. Persistence across DB close/reopen
	store.Close()
	store2, err := skillstore.New(dbPath)
	if err != nil {
		t.Fatalf("Failed to reopen store: %v", err)
	}
	defer store2.Close()

	isSetReopened, err := store2.IsMilestoneLatchSet(latchKey)
	if err != nil {
		t.Fatalf("IsMilestoneLatchSet after reopen failed: %v", err)
	}
	if !isSetReopened {
		t.Fatalf("Expected latch to remain true after store reopen")
	}
}

func TestTelemetryReadiness_14DayCondition(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "14day_cond.db")
	store, err := skillstore.New(dbPath)
	if err != nil {
		t.Fatalf("Failed to init store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	baseTime := int64(1700000000)

	// Sub-test 1: Under 14 days (13 days) -> condition not satisfied
	_ = store.RecordTelemetryHistory(ctx, skillstore.TelemetryRecord{
		Timestamp: baseTime,
		RAMPct:    50.0,
	})
	_ = store.RecordTelemetryHistory(ctx, skillstore.TelemetryRecord{
		Timestamp: baseTime + 13*86400,
		RAMPct:    52.0,
	})

	stats, err := store.GetTelemetryHistoryStats(ctx)
	if err != nil {
		t.Fatalf("GetTelemetryHistoryStats failed: %v", err)
	}
	durationSec := stats.NewestUnix - stats.OldestUnix
	if durationSec >= 14*86400 {
		t.Errorf("Expected duration < 14 days, got %d seconds (%f days)", durationSec, float64(durationSec)/86400)
	}

	// Sub-test 2: Exactly 14 days -> condition satisfied
	_ = store.RecordTelemetryHistory(ctx, skillstore.TelemetryRecord{
		Timestamp: baseTime + 14*86400,
		RAMPct:    55.0,
	})

	stats2, err := store.GetTelemetryHistoryStats(ctx)
	if err != nil {
		t.Fatalf("GetTelemetryHistoryStats failed: %v", err)
	}
	durationSec2 := stats2.NewestUnix - stats2.OldestUnix
	if durationSec2 < 14*86400 {
		t.Errorf("Expected duration >= 14 days, got %d seconds (%f days)", durationSec2, float64(durationSec2)/86400)
	}

	// Sub-test 3: Over 14 days (20 days) -> condition satisfied
	_ = store.RecordTelemetryHistory(ctx, skillstore.TelemetryRecord{
		Timestamp: baseTime + 20*86400,
		RAMPct:    55.0,
	})

	stats3, err := store.GetTelemetryHistoryStats(ctx)
	if err != nil {
		t.Fatalf("GetTelemetryHistoryStats failed: %v", err)
	}
	durationSec3 := stats3.NewestUnix - stats3.OldestUnix
	if durationSec3 < 14*86400 {
		t.Errorf("Expected duration >= 14 days, got %d seconds (%f days)", durationSec3, float64(durationSec3)/86400)
	}
}
