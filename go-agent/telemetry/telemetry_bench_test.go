package telemetry

import (
	"context"
	"testing"
)

func BenchmarkCollectMetrics(b *testing.B) {
	t := NewCollector()
	ctx := context.Background()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = t.CollectMetrics(ctx)
	}
}

func BenchmarkUpdateEWMALatency(b *testing.B) {
	t := NewCollector()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = t.UpdateEWMALatency(45.5, 0.2)
	}
}
