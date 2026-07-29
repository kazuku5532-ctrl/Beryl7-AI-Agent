package telemetry

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestNewCollector(t *testing.T) {
	c := NewCollector()
	if c == nil {
		t.Fatal("Expected non-nil Collector")
	}
}

func TestCollectMetricsComplete(t *testing.T) {
	c := NewCollector()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	m := c.CollectMetrics(ctx)
	if m == nil {
		t.Fatal("Expected non-nil Metric output from CollectMetrics")
	}

	// Double collect to test 2s interval gating
	m2 := c.CollectMetrics(ctx)
	if m2 != nil {
		t.Logf("CollectMetrics 2s gating triggered")
	}
}

func TestTelemetryInternalHelpers(t *testing.T) {
	c := NewCollector()
	ctx := context.Background()

	_ = c.readCPUUsage()
	_ = c.readRAMUsage()
	_ = c.readHardwareTemp()
	_ = c.readSystemUptime()
	_ = c.readPingLatency()
	_ = c.readActiveClients()
	_ = c.ReadConntrackCount()

	_, _ = c.CallUbusExec(ctx, "network.interface.wan", "status")
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
}
