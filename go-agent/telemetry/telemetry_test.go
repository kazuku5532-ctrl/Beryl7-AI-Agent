package telemetry

import (
	"context"
	"strings"
	"testing"
)

func TestNewCollector(t *testing.T) {
	c := NewCollector()
	if c == nil {
		t.Fatal("Expected non-nil Collector")
	}
}

func TestCollectMetrics(t *testing.T) {
	c := NewCollector()
	ctx := context.Background()
	m := c.CollectMetrics(ctx)
	if m == nil {
		t.Fatal("Expected non-nil Metric output from CollectMetrics")
	}
	if m.CPUUsagePct < 0 || m.CPUUsagePct > 100 {
		t.Errorf("Invalid CPU Usage Pct: %.2f", m.CPUUsagePct)
	}
	if m.RAMUsagePct < 0 || m.RAMUsagePct > 100 {
		t.Errorf("Invalid RAM Usage Pct: %.2f", m.RAMUsagePct)
	}
}

func TestExportPrometheusMetrics(t *testing.T) {
	c := NewCollector()
	metricObj := &Metric{
		CPUUsagePct:     12.5,
		RAMUsagePct:     45.2,
		HardwareTempC:   59.5,
		LatencyMs:       28.0,
		SystemUptimeSec: 10000,
	}

	promOut := c.ExportPrometheusMetrics(metricObj)
	if !strings.Contains(promOut, "beryl7_cpu_usage_pct 12.50") {
		t.Errorf("Prometheus output missing expected CPU metric: %s", promOut)
	}
	if !strings.Contains(promOut, "beryl7_ram_usage_pct 45.20") {
		t.Errorf("Prometheus output missing expected RAM metric: %s", promOut)
	}
	if !strings.Contains(promOut, "beryl7_hardware_temp_c 59.50") {
		t.Errorf("Prometheus output missing expected Temp metric: %s", promOut)
	}
}
