package skillstore

import (
	"math"
	"path/filepath"
	"sync"
	"testing"
)

func TestComputeStateDistance(t *testing.T) {
	s1 := &StateSignature{
		RAMPct:     90.0,
		LatencyMs:  15.0,
		CPUPct:     25.0,
		TempC:      60.0,
		WANOffline: false,
		WiFiDown:   false,
	}

	// Identical signature distance must be 0
	dist0 := ComputeStateDistance(s1, s1)
	if dist0 != 0.0 {
		t.Errorf("Expected distance 0.0 for identical signatures, got %.4f", dist0)
	}

	// Nearby signature with slight RAM difference (92% vs 90%)
	s2 := &StateSignature{
		RAMPct:     92.0,
		LatencyMs:  15.0,
		CPUPct:     25.0,
		TempC:      60.0,
		WANOffline: false,
		WiFiDown:   false,
	}
	distSmall := ComputeStateDistance(s1, s2)
	// delta_ram = (92-90)/10 = 0.2; dist = sqrt(1.5 * 0.04) = sqrt(0.06) ≈ 0.2449
	expected := math.Sqrt(1.5 * 0.2 * 0.2)
	if math.Abs(distSmall-expected) > 0.001 {
		t.Errorf("Expected distance %.4f, got %.4f", expected, distSmall)
	}

	// Completely different signature (WAN offline, huge latency)
	s3 := &StateSignature{
		RAMPct:     45.0,
		LatencyMs:  999.0,
		CPUPct:     15.0,
		TempC:      55.0,
		WANOffline: true,
		WiFiDown:   false,
	}
	distLarge := ComputeStateDistance(s1, s3)
	if distLarge < 2.5 {
		t.Errorf("Expected large distance > 2.5 between RAM anomaly and WAN drop, got %.4f", distLarge)
	}
}

func TestRecommendBestActionWithInterpolation(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_interp.db")

	store, err := New(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize SkillStore: %v", err)
	}
	defer store.Close()

	// 1. Test Exact Match Precedence
	action, qVal, matchedState, isInterp, errExact := store.RecommendBestActionWithInterpolation(
		"MEMORY_EXHAUSTION",
		nil,
		"default_action",
	)
	if errExact != nil {
		t.Fatalf("Exact match failed: %v", errExact)
	}
	if action != "purge_memory_cache" || isInterp || matchedState != "MEMORY_EXHAUSTION" {
		t.Errorf("Expected exact match purge_memory_cache without interpolation, got action=%s isInterp=%v matched=%s", action, isInterp, matchedState)
	}
	if qVal < 0.5 {
		t.Errorf("Expected Q-value >= 0.5 for seed, got %.2f", qVal)
	}

	// 2. Test Nearest Neighbor Interpolation for Novel Unseen Anomaly
	// Unseen Anomaly: "HIGH_ANONYMOUS_MEM_PRESSURE" with vector close to MEMORY_EXHAUSTION
	novelSig := &StateSignature{
		StateName:  "HIGH_ANONYMOUS_MEM_PRESSURE",
		RAMPct:     91.5,
		LatencyMs:  18.0,
		CPUPct:     28.0,
		TempC:      61.0,
		WANOffline: false,
		WiFiDown:   false,
	}

	actionInterp, qInterp, matchedInterp, isInterp2, errInterp := store.RecommendBestActionWithInterpolation(
		"HIGH_ANONYMOUS_MEM_PRESSURE",
		novelSig,
		"fallback_action",
	)
	if errInterp != nil {
		t.Fatalf("Interpolation lookup failed: %v", errInterp)
	}
	if !isInterp2 {
		t.Errorf("Expected isInterpolated=true for unseen anomaly")
	}
	if matchedInterp != "MEMORY_EXHAUSTION" {
		t.Errorf("Expected matchedState=MEMORY_EXHAUSTION, got %s", matchedInterp)
	}
	if actionInterp != "purge_memory_cache" {
		t.Errorf("Expected action=purge_memory_cache, got %s", actionInterp)
	}
	if qInterp <= 0.0 || qInterp > 0.60 {
		t.Errorf("Expected decayed Q-value in (0.0, 0.60], got %.4f", qInterp)
	}

	// 3. Test Distance Cutoff Threshold (Far-away strange anomaly should fallback to default)
	alienSig := &StateSignature{
		StateName:  "ALIEN_HARDWARE_GLITCH",
		RAMPct:     12.0, // Low RAM
		LatencyMs:  500.0,
		CPUPct:     99.0, // Crazy CPU
		TempC:      95.0, // Overheating
		WANOffline: true,
		WiFiDown:   true,
	}
	actionAlien, qAlien, _, isInterpAlien, errAlien := store.RecommendBestActionWithInterpolation(
		"ALIEN_HARDWARE_GLITCH",
		alienSig,
		"safe_reboot",
	)
	if errAlien != nil {
		t.Fatalf("Alien anomaly lookup failed: %v", errAlien)
	}
	if actionAlien != "safe_reboot" || isInterpAlien || qAlien != 0.0 {
		t.Errorf("Expected fallback to default safe_reboot with Q=0.0, got action=%s isInterp=%v q=%.2f", actionAlien, isInterpAlien, qAlien)
	}

	// 4. Test Harmonization: Negative Q-Value Rejection on Nearest Neighbor
	// Penalize REPEATER_SIGNAL_WEAK action to negative Q
	_ = store.UpdateQValue("REPEATER_SIGNAL_WEAK", "scale_tx_power_down", -0.8)
	_ = store.UpdateQValue("REPEATER_SIGNAL_WEAK", "scale_tx_power_down", -0.8)
	_ = store.UpdateQValue("REPEATER_SIGNAL_WEAK", "scale_tx_power_down", -0.8)

	weakSig := &StateSignature{
		StateName:  "WEAK_SIGNAL_ROAMING",
		RAMPct:     48.5,
		LatencyMs:  86.0,
		CPUPct:     19.0,
		TempC:      56.5,
		WANOffline: false,
		WiFiDown:   false,
	}
	actionWeak, qWeak, _, isInterpWeak, errWeak := store.RecommendBestActionWithInterpolation(
		"WEAK_SIGNAL_ROAMING",
		weakSig,
		"fallback_scan",
	)
	if errWeak != nil {
		t.Fatalf("Weak signal lookup failed: %v", errWeak)
	}
	if actionWeak != "fallback_scan" || isInterpWeak || qWeak != 0.0 {
		t.Errorf("Harmonization failed: Expected negative Q neighbor to be rejected and fallback to fallback_scan, got action=%s isInterp=%v q=%.2f", actionWeak, isInterpWeak, qWeak)
	}
}

func TestRecordStateSignatureAndConcurrency(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_sig_concurrency.db")

	store, err := New(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize SkillStore: %v", err)
	}
	defer store.Close()

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sig := &StateSignature{
				StateName:  "CONCURRENT_ANOMALY",
				RAMPct:     80.0 + float64(idx%10),
				LatencyMs:  50.0 + float64(idx*2),
				CPUPct:     20.0 + float64(idx),
				TempC:      55.0,
				WANOffline: false,
				WiFiDown:   false,
			}
			_ = store.RecordStateSignature(sig)
			_, _, _ = store.FindNearestState(sig, 5.0)
		}(i)
	}
	wg.Wait()
}
