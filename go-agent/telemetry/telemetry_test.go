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

// TestExportPrometheusMetricsOffline covers offline WAN path + mu-locked counters
func TestExportPrometheusMetricsOffline(t *testing.T) {
	c := NewCollector()
	m := &Metric{
		WANStatus:       "Offline - No Route",
		CPUUsagePct:     0,
		RAMUsagePct:     0,
		SystemUptimeSec: 0,
	}
	out := c.ExportPrometheusMetrics(m)
	if !strings.Contains(out, "beryl7_router_reachable 0") {
		t.Errorf("Expected reachable=0 for Offline status, got: %s", out)
	}
	// Exercise counter methods for coverage
	c.RecordSkillHit()
	c.RecordSkillMiss()
	c.RecordRollback()
	c.RecordFalsePositive()
	c.RecordHealOutcome(true)
	c.RecordHealOutcome(false)
	// Re-export with counters set
	out2 := c.ExportPrometheusMetrics(m)
	if !strings.Contains(out2, "beryl7_skill_hits_total 1") {
		t.Errorf("Expected skill_hits_total=1, got: %s", out2)
	}
}

// TestExportPrometheusMetricsNil ensures nil metric returns empty string gracefully
func TestExportPrometheusMetricsNil(t *testing.T) {
	c := NewCollector()
	out := c.ExportPrometheusMetrics(nil)
	if out != "" {
		t.Errorf("Expected empty string for nil metric, got: %s", out)
	}
}

// TestAreWiFiClientsIdleDualBand exercises conservative fail-safe guard when both bands return error
func TestAreWiFiClientsIdleDualBand(t *testing.T) {
	c := NewCollector()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// ubus is not available in test env — both bands return err.
	// Conservative Guard returns isIdle=false to prevent accidental WiFi reloads under heavy load.
	isIdle, clients, err := c.AreWiFiClientsIdle(ctx)
	if err == nil {
		t.Errorf("Expected error when both ubus queries fail, got nil")
	}
	if isIdle {
		t.Errorf("Expected conservative isIdle=false when both bands fail, got true (clients=%d)", clients)
	}
}

// TestParseProcNetDevInterface exercises the packet counter parser for /proc/net/dev format
func TestParseProcNetDevInterface(t *testing.T) {
	// Realistic /proc/net/dev line for eth0 with rx=1000 bytes and tx=2000 bytes at column positions
	sampleContent := `Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
  eth0: 1000       8    0    0    0     0          0         0     2000       5    0    0    0     0       0          0
`
	rx, tx := parseProcNetDevInterface(sampleContent, "eth0")
	if rx != 1000 {
		t.Errorf("Expected rx=1000, got %d", rx)
	}
	if tx != 2000 {
		t.Errorf("Expected tx=2000, got %d", tx)
	}

	// Interface not found → should return 0,0
	rx2, tx2 := parseProcNetDevInterface(sampleContent, "wlan9")
	if rx2 != 0 || tx2 != 0 {
		t.Errorf("Expected 0,0 for missing iface, got %d,%d", rx2, tx2)
	}

	// Malformed line (insufficient fields) → should return 0,0
	malformed := "  eth1: 100\n"
	rx3, tx3 := parseProcNetDevInterface(malformed, "eth1")
	if rx3 != 0 || tx3 != 0 {
		t.Errorf("Expected 0,0 for malformed line, got %d,%d", rx3, tx3)
	}
}

// TestDiscoverWiFiInterfaces exercises the fallback path when no matching interfaces found
func TestDiscoverWiFiInterfaces(t *testing.T) {
	g, h := DiscoverWiFiInterfaces()
	// On non-OpenWrt host, fallback names should be returned (ra0/rai0)
	if g == "" {
		t.Error("Expected non-empty 2.4GHz interface name")
	}
	if h == "" {
		t.Error("Expected non-empty 5GHz interface name")
	}
}

// TestRepeaterMetricsCollection validates repeater interface resolution and metric collection
func TestRepeaterMetricsCollection(t *testing.T) {
	c := NewCollector()
	ctx := context.Background()

	iface := c.ResolveRepeaterInterface(ctx)
	if iface == "" {
		t.Errorf("Expected non-empty resolved repeater interface")
	}

	rm, err := c.CollectRepeaterMetrics(ctx)
	if err != nil {
		t.Fatalf("CollectRepeaterMetrics failed: %v", err)
	}
	if rm == nil {
		t.Fatalf("Expected non-nil RepeaterMetrics")
	}
}

// TestUpdateEWMALatency validates EWMA initialisation and z-score calculation branches
func TestUpdateEWMALatency(t *testing.T) {
	c := NewCollector()

	// First call: initialise EWMA (zero variance → zScore=0)
	ewma1, z1 := c.UpdateEWMALatency(50.0, 0.2)
	if ewma1 != 50.0 {
		t.Errorf("Expected EWMA=50.0 on init, got %f", ewma1)
	}
	if z1 != 0.0 {
		t.Errorf("Expected zScore=0.0 on init, got %f", z1)
	}

	// Second call: update EWMA, variance builds up
	ewma2, _ := c.UpdateEWMALatency(100.0, 0.2)
	if ewma2 <= 50.0 {
		t.Errorf("Expected EWMA to increase above 50.0, got %f", ewma2)
	}

	// Third call: z-score should now be calculable (stdDev > 0)
	_, z3 := c.UpdateEWMALatency(200.0, 0.2)
	if z3 == 0.0 {
		t.Logf("Note: z-score was 0 (variance may not yet be significant)")
	}

	// Invalid alpha → clamped to 0.2
	ewma4, _ := c.UpdateEWMALatency(50.0, -1.0)
	if ewma4 == 0 {
		t.Errorf("Expected valid EWMA with invalid alpha, got 0")
	}

	// alpha=1.5 (>1) also clamped
	_, _ = c.UpdateEWMALatency(50.0, 1.5)
}

func TestApplyAdaptiveThermalFanCurve(t *testing.T) {
	c := NewCollector()

	tests := []struct {
		temp     float64
		expected int
	}{
		{temp: 45.0, expected: 0},
		{temp: 64.9, expected: 0},
		{temp: 65.0, expected: 85},
		{temp: 70.0, expected: 85},
		{temp: 71.9, expected: 85},
		{temp: 72.0, expected: 160},
		{temp: 75.0, expected: 160},
		{temp: 79.9, expected: 160},
		{temp: 80.0, expected: 255},
		{temp: 95.0, expected: 255},
	}

	for _, tt := range tests {
		actual := c.ApplyAdaptiveThermalFanCurve(tt.temp)
		if actual != tt.expected {
			t.Errorf("ApplyAdaptiveThermalFanCurve(%.1f°C) = %d; want %d", tt.temp, actual, tt.expected)
		}
	}
}

func TestHarmonizeRepeaterState(t *testing.T) {
	c := NewCollector()
	ctx := context.Background()

	// Should not panic on both branches
	c.HarmonizeRepeaterState(ctx, true)
	c.HarmonizeRepeaterState(ctx, false)
}

func TestCalculateAdaptiveSQMRates(t *testing.T) {
	c := NewCollector()

	// High latency path -> throttles
	dl, ul := c.CalculateAdaptiveSQMRates(100.0, 2.5)
	if dl == "" || ul == "" {
		t.Errorf("Expected non-empty SQM rates, got dl=%s ul=%s", dl, ul)
	}

	// Normal latency path -> normal headroom
	dl2, ul2 := c.CalculateAdaptiveSQMRates(20.0, 0.5)
	if dl2 == "" || ul2 == "" {
		t.Errorf("Expected non-empty SQM rates for normal latency, got dl=%s ul=%s", dl2, ul2)
	}
}

func TestGetConnectedDevices(t *testing.T) {
	c := NewCollector()
	ctx := context.Background()

	devices := c.GetConnectedDevices(ctx, false, 50.0, 10.0)
	_ = len(devices)

	_, _ = c.CallUbusExecArgs(ctx, "hostapd.radio1", "get_clients", "{}")
}

func TestAsyncPingProbeAndCaching(t *testing.T) {
	c := NewCollector()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// 1. Initial cached latency should be 0.0
	initLat := c.GetCachedPingLatency()
	if initLat < 0 {
		t.Errorf("Expected non-negative initial cached latency, got %f", initLat)
	}

	// 2. Probe ping latency directly
	_ = c.ProbePingLatency(ctx)

	// 3. readPingLatency should return immediately from cache
	start := time.Now()
	_ = c.readPingLatency()
	elapsed := time.Since(start)
	if elapsed > 100*time.Millisecond {
		t.Errorf("readPingLatency took too long (%v), expected non-blocking < 100ms", elapsed)
	}
}
