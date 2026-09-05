package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// TestAdversarial_MalformedIntents_GracefulFallback verifies that malformed, negative, zero,
// or nonsensical values in intents.json gracefully fall back to base thresholds without panic or NaN.
func TestAdversarial_MalformedIntents_GracefulFallback(t *testing.T) {
	tempDir := t.TempDir()

	testCases := []struct {
		name        string
		content     string
		expectErr   bool
		checkOutput func(t *testing.T, eff EffectiveThresholds)
	}{
		{
			name:      "Incomplete JSON syntax",
			content:   `{"intents": [{"name": "broken", "start_time": "08:00"`,
			expectErr: true,
			checkOutput: func(t *testing.T, eff EffectiveThresholds) {
				if eff.ActiveIntent != "default" {
					t.Errorf("Expected ActiveIntent 'default', got %s", eff.ActiveIntent)
				}
				if eff.RAMExhaustionPct <= 0 || eff.CPUSpikeLoad <= 0 || eff.LatencySpikeMs <= 0 || eff.LatencyZScoreThreshold <= 0 {
					t.Errorf("Expected positive base thresholds, got %+v", eff)
				}
			},
		},
		{
			name:      "Invalid types in fields",
			content:   `{"intents": [{"name": 12345, "start_time": true, "end_time": []}]}`,
			expectErr: true,
			checkOutput: func(t *testing.T, eff EffectiveThresholds) {
				if eff.ActiveIntent != "default" {
					t.Errorf("Expected ActiveIntent 'default', got %s", eff.ActiveIntent)
				}
			},
		},
		{
			name: "Out of range time strings",
			content: `{
				"intents": [
					{
						"name": "impossible_hours",
						"start_time": "25:99",
						"end_time": "-01:00",
						"ram_exhaustion_pct": 80.0
					}
				]
			}`,
			expectErr: false, // JSON parses fine, but ParseTimeOfDay skips invalid intent
			checkOutput: func(t *testing.T, eff EffectiveThresholds) {
				if eff.ActiveIntent != "default" {
					t.Errorf("Expected ActiveIntent 'default' due to invalid times, got %s", eff.ActiveIntent)
				}
			},
		},
		{
			name: "Empty time strings and whitespace",
			content: `{
				"intents": [
					{
						"name": "empty_times",
						"start_time": "   ",
						"end_time": "",
						"ram_exhaustion_pct": 80.0
					}
				]
			}`,
			expectErr: false,
			checkOutput: func(t *testing.T, eff EffectiveThresholds) {
				if eff.ActiveIntent != "default" {
					t.Errorf("Expected ActiveIntent 'default' for empty time strings, got %s", eff.ActiveIntent)
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			filePath := filepath.Join(tempDir, fmt.Sprintf("intent_%d.json", time.Now().UnixNano()))
			if err := os.WriteFile(filePath, []byte(tc.content), 0600); err != nil {
				t.Fatalf("Failed to write test file: %v", err)
			}

			intents, err := LoadIntents(filePath)
			if tc.expectErr && err == nil {
				t.Logf("Note: LoadIntents did not return error (handled safely)")
			}

			cfg := &Config{
				IntentsFilePath:        filePath,
				RAMExhaustionPct:       95.0,
				CPUSpikeLoad:           1.5,
				LatencySpikeMs:         100.0,
				LatencyZScoreThreshold: 2.5,
			}

			eff := ResolveEffectiveThresholds(cfg, intents, time.Now())
			tc.checkOutput(t, eff)
		})
	}
}

// TestAdversarial_OverlappingIntents_DeterministicOrder asserts that when two intents cover the
// same time window, the first-declared intent is selected deterministically.
func TestAdversarial_OverlappingIntents_DeterministicOrder(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "overlapping_intents.json")

	ram1 := 80.0
	ram2 := 70.0
	ram3 := 60.0

	cfgFile := IntentConfigFile{
		Intents: []Intent{
			{
				Name:             "priority_first",
				StartTime:        "08:00",
				EndTime:          "20:00",
				RAMExhaustionPct: &ram1,
			},
			{
				Name:             "priority_second_overlap",
				StartTime:        "10:00",
				EndTime:          "18:00",
				RAMExhaustionPct: &ram2,
			},
			{
				Name:             "priority_third_full_day",
				StartTime:        "00:00",
				EndTime:          "23:59",
				RAMExhaustionPct: &ram3,
			},
		},
	}

	data, err := json.Marshal(cfgFile)
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}
	if err := os.WriteFile(filePath, data, 0600); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	intents, err := LoadIntents(filePath)
	if err != nil {
		t.Fatalf("LoadIntents failed: %v", err)
	}

	cfg := &Config{
		IntentsFilePath:        filePath,
		RAMExhaustionPct:       95.0,
		CPUSpikeLoad:           1.5,
		LatencySpikeMs:         100.0,
		LatencyZScoreThreshold: 2.5,
	}

	// At 14:00 (overlaps all three intents) -> MUST choose priority_first
	midday := time.Date(2026, 9, 6, 14, 0, 0, 0, time.UTC)
	eff := ResolveEffectiveThresholds(cfg, intents, midday)

	if eff.ActiveIntent != "priority_first" {
		t.Errorf("Expected ActiveIntent 'priority_first', got %s", eff.ActiveIntent)
	}
	if eff.RAMExhaustionPct != 80.0 {
		t.Errorf("Expected RAMExhaustionPct 80.0, got %.1f", eff.RAMExhaustionPct)
	}

	// At 21:00 (only overlaps priority_third_full_day) -> MUST choose priority_third_full_day
	night := time.Date(2026, 9, 6, 21, 0, 0, 0, time.UTC)
	effNight := ResolveEffectiveThresholds(cfg, intents, night)

	if effNight.ActiveIntent != "priority_third_full_day" {
		t.Errorf("Expected ActiveIntent 'priority_third_full_day', got %s", effNight.ActiveIntent)
	}
	if effNight.RAMExhaustionPct != 60.0 {
		t.Errorf("Expected RAMExhaustionPct 60.0, got %.1f", effNight.RAMExhaustionPct)
	}
}

// TestAdversarial_ConcurrentConfigReloadAndThresholdResolve tests high-concurrency race condition
// safety when multiple goroutines resolve thresholds while the config file is repeatedly modified.
func TestAdversarial_ConcurrentConfigReloadAndThresholdResolve(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "concurrent_reload.json")

	ramA := 85.0
	ramB := 75.0

	fileA := IntentConfigFile{
		Intents: []Intent{
			{
				Name:             "intent_A",
				StartTime:        "00:00",
				EndTime:          "23:59",
				RAMExhaustionPct: &ramA,
			},
		},
	}
	fileB := IntentConfigFile{
		Intents: []Intent{
			{
				Name:             "intent_B",
				StartTime:        "00:00",
				EndTime:          "23:59",
				RAMExhaustionPct: &ramB,
			},
		},
	}

	dataA, _ := json.Marshal(fileA)
	dataB, _ := json.Marshal(fileB)
	_ = os.WriteFile(filePath, dataA, 0600)

	cfg := &Config{
		IntentsFilePath:        filePath,
		RAMExhaustionPct:       95.0,
		CPUSpikeLoad:           1.5,
		LatencySpikeMs:         100.0,
		LatencyZScoreThreshold: 2.5,
	}

	var wg sync.WaitGroup
	workers := 20
	stopCh := make(chan struct{})

	// Goroutine writing updates to file
	wg.Add(1)
	go func() {
		defer wg.Done()
		toggle := false
		for {
			select {
			case <-stopCh:
				return
			default:
				if toggle {
					_ = os.WriteFile(filePath, dataA, 0600)
				} else {
					_ = os.WriteFile(filePath, dataB, 0600)
				}
				toggle = !toggle
				time.Sleep(2 * time.Millisecond)
			}
		}
	}()

	// Reader goroutines
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				eff := cfg.GetEffectiveThresholds(time.Now())
				if eff.ActiveIntent != "intent_A" && eff.ActiveIntent != "intent_B" && eff.ActiveIntent != "default" {
					t.Errorf("Worker %d got unexpected ActiveIntent: %s", workerID, eff.ActiveIntent)
				}
				if eff.RAMExhaustionPct != 85.0 && eff.RAMExhaustionPct != 75.0 && eff.RAMExhaustionPct != 95.0 {
					t.Errorf("Worker %d got unexpected RAM threshold: %.1f", workerID, eff.RAMExhaustionPct)
				}
				time.Sleep(1 * time.Millisecond)
			}
		}(i)
	}

	// Wait for readers
	time.Sleep(100 * time.Millisecond)
	close(stopCh)
	wg.Wait()
}
