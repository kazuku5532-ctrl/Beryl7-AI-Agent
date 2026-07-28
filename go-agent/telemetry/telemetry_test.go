package telemetry

import (
	"testing"
)

func TestNewCollector(t *testing.T) {
	col := NewCollector()
	if col == nil {
		t.Fatalf("Expected NewCollector() to return valid pointer, got nil")
	}
}

func TestExportPrometheusMetrics(t *testing.T) {
	col := NewCollector()
	m := &Metric{
		CPUUsagePct:     1.5,
		RAMUsagePct:     43.9,
		HardwareTempC:   59.5,
		LatencyMs:       31.0,
		SystemUptimeSec: 12000,
	}

	out := col.ExportPrometheusMetrics(m)
	if len(out) == 0 {
		t.Fatalf("Expected non-empty Prometheus metrics output")
	}
}
