package telemetry

import (
	"context"
	"os"
	"path/filepath"
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

func TestCollectMetrics(t *testing.T) {
	c := NewCollector()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	m := c.CollectMetrics(ctx)
	if m == nil {
		t.Fatal("Expected non-nil Metric output from CollectMetrics")
	}
}

func TestTelemetryInternalHelpers(t *testing.T) {
	c := NewCollector()
	_ = c.readCPUUsage()
	_ = c.readRAMUsage()
	_ = c.readHardwareTemp()
	_ = c.ReadConntrackCount()

	// Mock file reads for Linux telemetry paths
	tempDir := t.TempDir()

	statFile := filepath.Join(tempDir, "stat")
	_ = os.WriteFile(statFile, []byte("cpu  2255 34 2290 226255 12 0 0 0 0 0\n"), 0644)

	memFile := filepath.Join(tempDir, "meminfo")
	_ = os.WriteFile(memFile, []byte("MemTotal:        512000 kB\nMemAvailable:    256000 kB\n"), 0644)

	thermalFile := filepath.Join(tempDir, "temp")
	_ = os.WriteFile(thermalFile, []byte("59950\n"), 0644)
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
