package tests

import (
	"context"
	"runtime"
	"testing"

	"beryl7-agent/skillstore"
	"beryl7-agent/telemetry"
)

func BenchmarkSkillStoreGetSkill(b *testing.B) {
	store, _ := skillstore.New(":memory:")
	defer store.Close()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		store.GetSkill("TEST", "TEST_ACTION")
	}
}

func BenchmarkSkillStoreUpdateQValue(b *testing.B) {
	store, _ := skillstore.New(":memory:")
	defer store.Close()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		store.UpdateQValue("TEST_STATE", "TEST_ACTION", 1.0)
	}
}

func BenchmarkSkillStoreRecommendWithInterpolation(b *testing.B) {
	store, _ := skillstore.New(":memory:")
	defer store.Close()
	sig := &skillstore.StateSignature{
		StateName: "TEST",
		RAMPct:    50,
		LatencyMs: 20,
		CPUPct:    10,
		TempC:     40,
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		store.RecommendBestActionWithInterpolation("UNKNOWN", sig, "default")
	}
}

func BenchmarkTelemetryCollectMetrics(b *testing.B) {
	collector := telemetry.NewCollector()
	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		collector.CollectMetrics(ctx)
	}
}

func BenchmarkTelemetryProcessResourceStats(b *testing.B) {
	collector := telemetry.NewCollector()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		collector.GetProcessResourceStats()
	}
}

func TestResourceStatsIntegrity(t *testing.T) {
	collector := telemetry.NewCollector()
	stats := collector.GetProcessResourceStats()
	if stats.Goroutines <= 0 {
		t.Errorf("Expected positive Goroutine count, got %d", stats.Goroutines)
	}
	if stats.HeapAllocMB <= 0 {
		t.Errorf("Expected > 0 HeapAllocMB, got %f", stats.HeapAllocMB)
	}
	if stats.RSSMB <= 0 {
		t.Errorf("Expected > 0 RSSMB, got %f", stats.RSSMB)
	}
}

func TestMemoryGrowthUnderContinuousLoad(t *testing.T) {
	store, _ := skillstore.New(":memory:")
	defer store.Close()
	collector := telemetry.NewCollector()
	ctx := context.Background()

	runtime.GC()
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)

	sig := &skillstore.StateSignature{
		StateName: "TEST",
		RAMPct:    50,
		LatencyMs: 20,
		CPUPct:    10,
		TempC:     40,
	}

	for i := 0; i < 2000; i++ {
		store.UpdateQValue("TEST_STATE", "TEST_ACTION", 1.0)
		store.RecommendBestActionWithInterpolation("UNKNOWN", sig, "default")
		collector.CollectMetrics(ctx)
	}

	runtime.GC()
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)

	growthMB := float64(m2.Alloc - m1.Alloc) / 1024 / 1024
	if growthMB > 1.0 {
		t.Errorf("Heap growth exceeded 1.0MB: grew by %f MB", growthMB)
	}
}
