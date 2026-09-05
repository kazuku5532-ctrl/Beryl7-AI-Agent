package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestParseTimeOfDay(t *testing.T) {
	tests := []struct {
		input       string
		expectedMin int
		expectErr   bool
	}{
		{"00:00", 0, false},
		{"01:00", 60, false},
		{"08:15", 495, false},
		{"12:30", 750, false},
		{"18:00", 1080, false},
		{"23:59", 1439, false},
		{"  09:45  ", 585, false},
		{"24:00", 0, true},
		{"12:60", 0, true},
		{"-01:00", 0, true},
		{"08:-10", 0, true},
		{"abc:def", 0, true},
		{"12", 0, true},
		{"12:30:45", 0, true},
		{"", 0, true},
		{":", 0, true},
	}

	for _, tt := range tests {
		min, err := ParseTimeOfDay(tt.input)
		if tt.expectErr && err == nil {
			t.Errorf("ParseTimeOfDay(%q) expected error, got %d min", tt.input, min)
		} else if !tt.expectErr && err != nil {
			t.Errorf("ParseTimeOfDay(%q) unexpected error: %v", tt.input, err)
		} else if !tt.expectErr && min != tt.expectedMin {
			t.Errorf("ParseTimeOfDay(%q) = %d, expected %d", tt.input, min, tt.expectedMin)
		}
	}
}

func TestIsTimeInWindow_Daytime(t *testing.T) {
	startMin, _ := ParseTimeOfDay("18:00") // 1080
	endMin, _ := ParseTimeOfDay("23:00")   // 1380

	cases := []struct {
		timeStr  string
		expected bool
	}{
		{"18:00", true},
		{"20:00", true},
		{"22:59", true},
		{"23:00", false},
		{"17:59", false},
		{"12:00", false},
		{"00:00", false},
	}

	for _, tc := range cases {
		nowMin, _ := ParseTimeOfDay(tc.timeStr)
		result := IsTimeInWindow(nowMin, startMin, endMin)
		if result != tc.expected {
			t.Errorf("IsTimeInWindow(%s, 18:00, 23:00) = %v, expected %v", tc.timeStr, result, tc.expected)
		}
	}
}

func TestIsTimeInWindow_Overnight(t *testing.T) {
	startMin, _ := ParseTimeOfDay("23:00") // 1380
	endMin, _ := ParseTimeOfDay("06:00")   // 360

	cases := []struct {
		timeStr  string
		expected bool
	}{
		{"23:00", true},
		{"23:30", true},
		{"00:00", true},
		{"02:00", true},
		{"05:59", true},
		{"06:00", false},
		{"12:00", false},
		{"22:59", false},
	}

	for _, tc := range cases {
		nowMin, _ := ParseTimeOfDay(tc.timeStr)
		result := IsTimeInWindow(nowMin, startMin, endMin)
		if result != tc.expected {
			t.Errorf("IsTimeInWindow(%s, 23:00, 06:00) = %v, expected %v", tc.timeStr, result, tc.expected)
		}
	}
}

func TestResolveEffectiveThresholds_NoIntentMatch(t *testing.T) {
	cfg := &Config{
		RAMExhaustionPct:       90.0,
		CPUSpikeLoad:           2.0,
		LatencySpikeMs:         80.0,
		LatencyZScoreThreshold: 2.0,
	}

	intents := []Intent{
		{
			Name:      "night_maintenance",
			StartTime: "01:00",
			EndTime:   "04:00",
		},
	}

	// Test at 12:00 (No match)
	now := time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC)
	effective := ResolveEffectiveThresholds(cfg, intents, now)

	if effective.ActiveIntent != "default" {
		t.Errorf("Expected ActiveIntent 'default', got %q", effective.ActiveIntent)
	}
	if effective.RAMExhaustionPct != 90.0 || effective.CPUSpikeLoad != 2.0 || effective.LatencySpikeMs != 80.0 || effective.LatencyZScoreThreshold != 2.0 {
		t.Errorf("Expected base thresholds, got %+v", effective)
	}
}

func TestResolveEffectiveThresholds_PartialOverrides(t *testing.T) {
	cfg := &Config{
		RAMExhaustionPct:       95.0,
		CPUSpikeLoad:           1.5,
		LatencySpikeMs:         100.0,
		LatencyZScoreThreshold: 2.5,
	}

	ramOverride := 80.0
	latOverride := 50.0

	intents := []Intent{
		{
			Name:             "prime_gaming",
			Description:      "Low latency for gaming hours",
			StartTime:        "19:00",
			EndTime:          "23:30",
			RAMExhaustionPct: &ramOverride,
			LatencySpikeMs:   &latOverride,
			// CPUSpikeLoad and LatencyZScoreThreshold omitted (nil)
		},
	}

	now := time.Date(2026, 9, 5, 20, 15, 0, 0, time.UTC)
	effective := ResolveEffectiveThresholds(cfg, intents, now)

	if effective.ActiveIntent != "prime_gaming" {
		t.Errorf("Expected ActiveIntent 'prime_gaming', got %q", effective.ActiveIntent)
	}
	if effective.RAMExhaustionPct != 80.0 {
		t.Errorf("Expected RAMExhaustionPct 80.0, got %.1f", effective.RAMExhaustionPct)
	}
	if effective.LatencySpikeMs != 50.0 {
		t.Errorf("Expected LatencySpikeMs 50.0, got %.1f", effective.LatencySpikeMs)
	}
	if effective.CPUSpikeLoad != 1.5 {
		t.Errorf("Expected CPUSpikeLoad fallback 1.5, got %.1f", effective.CPUSpikeLoad)
	}
	if effective.LatencyZScoreThreshold != 2.5 {
		t.Errorf("Expected LatencyZScoreThreshold fallback 2.5, got %.1f", effective.LatencyZScoreThreshold)
	}
}

func TestResolveEffectiveThresholds_OvernightMatching(t *testing.T) {
	cfg := &Config{
		RAMExhaustionPct:       95.0,
		CPUSpikeLoad:           1.5,
		LatencySpikeMs:         100.0,
		LatencyZScoreThreshold: 2.5,
	}

	ramRelaxed := 98.0
	cpuRelaxed := 3.0
	latRelaxed := 300.0
	zRelaxed := 4.0

	intents := []Intent{
		{
			Name:                   "overnight_backup",
			StartTime:              "23:00",
			EndTime:                "06:00",
			RAMExhaustionPct:       &ramRelaxed,
			CPUSpikeLoad:           &cpuRelaxed,
			LatencySpikeMs:         &latRelaxed,
			LatencyZScoreThreshold: &zRelaxed,
		},
	}

	// 02:30 AM
	now := time.Date(2026, 9, 5, 2, 30, 0, 0, time.UTC)
	effective := ResolveEffectiveThresholds(cfg, intents, now)

	if effective.ActiveIntent != "overnight_backup" {
		t.Errorf("Expected ActiveIntent 'overnight_backup', got %q", effective.ActiveIntent)
	}
	if effective.RAMExhaustionPct != 98.0 || effective.CPUSpikeLoad != 3.0 || effective.LatencySpikeMs != 300.0 || effective.LatencyZScoreThreshold != 4.0 {
		t.Errorf("Overnight threshold overrides mismatch: %+v", effective)
	}
}

func TestLoadIntents_MissingFile(t *testing.T) {
	intents, err := LoadIntents("/non/existent/path/intents.json")
	if err != nil {
		t.Errorf("Expected nil error for non-existent file, got %v", err)
	}
	if len(intents) != 0 {
		t.Errorf("Expected empty slice, got %d items", len(intents))
	}
}

func TestLoadIntents_EmptyPath(t *testing.T) {
	intents, err := LoadIntents("")
	if err != nil || len(intents) != 0 {
		t.Errorf("Expected nil error and empty slice for empty path, got err=%v len=%d", err, len(intents))
	}
}

func TestLoadIntents_CorruptJSON(t *testing.T) {
	tempDir := t.TempDir()
	badJSONPath := filepath.Join(tempDir, "corrupt.json")
	_ = os.WriteFile(badJSONPath, []byte(`{ "intents": [ { "name": "bad" `), 0600)

	intents, err := LoadIntents(badJSONPath)
	if err == nil {
		t.Errorf("Expected error for malformed JSON, got nil")
	}
	if len(intents) != 0 {
		t.Errorf("Expected empty intents on corrupt file, got %d", len(intents))
	}
}

func TestLoadIntents_OversizedFile(t *testing.T) {
	tempDir := t.TempDir()
	largePath := filepath.Join(tempDir, "large.json")

	// Create 128KB padding
	largeData := make([]byte, 128*1024)
	for i := range largeData {
		largeData[i] = ' '
	}
	_ = os.WriteFile(largePath, largeData, 0600)

	intents, err := LoadIntents(largePath)
	// io.LimitReader should safely read max 64KB and return cleanly
	if err != nil {
		t.Logf("Handled oversized empty content safely: %v", err)
	}
	if len(intents) != 0 {
		t.Errorf("Expected 0 intents, got %d", len(intents))
	}
}

func TestLoadIntents_ValidConfigFile(t *testing.T) {
	tempDir := t.TempDir()
	jsonPath := filepath.Join(tempDir, "intents.json")

	content := `{
		"intents": [
			{
				"name": "work_from_home",
				"description": "High reliability during work hours",
				"start_time": "09:00",
				"end_time": "17:00",
				"latency_spike_ms": 60.0
			},
			{
				"name": "night_idle",
				"start_time": "23:00",
				"end_time": "06:00",
				"ram_exhaustion_pct": 98.0
			}
		]
	}`

	if err := os.WriteFile(jsonPath, []byte(content), 0600); err != nil {
		t.Fatalf("Failed to write test intents.json: %v", err)
	}

	intents, err := LoadIntents(jsonPath)
	if err != nil {
		t.Fatalf("LoadIntents failed: %v", err)
	}
	if len(intents) != 2 {
		t.Fatalf("Expected 2 intents, got %d", len(intents))
	}

	if intents[0].Name != "work_from_home" || intents[0].LatencySpikeMs == nil || *intents[0].LatencySpikeMs != 60.0 {
		t.Errorf("First intent parsed incorrectly: %+v", intents[0])
	}
	if intents[1].Name != "night_idle" || intents[1].RAMExhaustionPct == nil || *intents[1].RAMExhaustionPct != 98.0 {
		t.Errorf("Second intent parsed incorrectly: %+v", intents[1])
	}
}

func TestLoadIntents_DirectSliceFormat(t *testing.T) {
	tempDir := t.TempDir()
	jsonPath := filepath.Join(tempDir, "direct_slice.json")

	content := `[
		{
			"name": "direct_intent",
			"start_time": "08:00",
			"end_time": "18:00",
			"cpu_spike_load": 2.5
		}
	]`

	_ = os.WriteFile(jsonPath, []byte(content), 0600)

	intents, err := LoadIntents(jsonPath)
	if err != nil {
		t.Fatalf("LoadIntents failed for direct slice: %v", err)
	}
	if len(intents) != 1 || intents[0].Name != "direct_intent" {
		t.Errorf("Unexpected direct slice result: %+v", intents)
	}
}

func TestConcurrentAccessSafety(t *testing.T) {
	tempDir := t.TempDir()
	jsonPath := filepath.Join(tempDir, "concurrent_intents.json")

	cfgFile := IntentConfigFile{
		Intents: []Intent{
			{
				Name:      "test_intent",
				StartTime: "00:00",
				EndTime:   "23:59",
			},
		},
	}
	data, _ := json.Marshal(cfgFile)
	_ = os.WriteFile(jsonPath, data, 0600)

	cfg := &Config{
		IntentsFilePath:        jsonPath,
		RAMExhaustionPct:       95.0,
		CPUSpikeLoad:           1.5,
		LatencySpikeMs:         100.0,
		LatencyZScoreThreshold: 2.5,
	}

	var wg sync.WaitGroup
	workers := 50
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				eff := cfg.GetEffectiveThresholds(time.Now())
				if eff.ActiveIntent != "test_intent" {
					t.Errorf("Expected ActiveIntent 'test_intent', got %s", eff.ActiveIntent)
				}
			}
		}()
	}

	wg.Wait()
}

func TestGetEffectiveThresholds_NilConfig(t *testing.T) {
	var cfg *Config = nil
	eff := cfg.GetEffectiveThresholds(time.Now())
	if eff.ActiveIntent != "default" {
		t.Errorf("Expected default active intent for nil config, got %s", eff.ActiveIntent)
	}
	if eff.RAMExhaustionPct != 95.0 {
		t.Errorf("Expected fallback RAM threshold 95.0, got %.1f", eff.RAMExhaustionPct)
	}
}
