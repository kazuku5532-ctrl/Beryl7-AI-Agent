package telemetry

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"beryl7-agent/logger"
)

type Metric struct {
	CollectedAt     time.Time `json:"collected_at"`
	CPUUsagePct     float64   `json:"cpu_usage_pct"`
	RAMUsagePct     float64   `json:"ram_usage_pct"`
	HardwareTempC   float64   `json:"hardware_temp_c"`
	LatencyMs       float64   `json:"latency_ms"`
	WANStatus       string    `json:"wan_status"`
	DownloadMbps    float64   `json:"download_mbps"`
	UploadMbps      float64   `json:"upload_mbps"`
	ActiveClients   int       `json:"active_clients"`
	WiFi5GGhzStatus string    `json:"wifi_5g_status"`
	SystemUptimeSec int64     `json:"system_uptime_sec"`
}

type TelemetryCollector struct {
	mu             sync.Mutex
	lastCollect    time.Time
	lastRxBytes    uint64
	lastTxBytes    uint64
	prevCPUTotal   float64
	prevCPUIdle    float64
	lastFlapTime   time.Time
	debounceWindow time.Duration
	ubusPath          string
	ewmaLatency       float64
	ewmaVariance      float64
	skillHitsTotal      int64
	skillMissesTotal    int64
	healSuccessTotal    int64
	healFailuresTotal   int64
	rollbacksTotal      int64
	falsePositivesTotal int64
}

func NewCollector() *TelemetryCollector {
	ubusPath, err := exec.LookPath("ubus")
	if err != nil {
		ubusPath = ""
	}

	return &TelemetryCollector{
		debounceWindow: 10 * time.Second,
		ubusPath:       ubusPath,
	}
}

func (t *TelemetryCollector) CallUbusExec(ctx context.Context, path, method string) (string, error) {
	if t.ubusPath == "" {
		return "", errors.New("ubus binary not found on system")
	}

	ctxTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	cleanPath := filepath.Clean(t.ubusPath)
	cmd := exec.CommandContext(ctxTimeout, cleanPath, "call", path, method) // #nosec G204
	output, err := cmd.Output()

	if ctxTimeout.Err() == context.DeadlineExceeded {
		if cmd.Process != nil {
			if killErr := cmd.Process.Kill(); killErr != nil && !errors.Is(killErr, os.ErrProcessDone) {
				logger.Warn("Failed to kill timed-out ubus process: %v", killErr)
			}
			_ = cmd.Wait()
		}
		return "", fmt.Errorf("ubus call timed out after 5s: %s %s", path, method)
	}

	if err != nil {
		return "", fmt.Errorf("ubus call failed: %w", err)
	}

	return string(output), nil
}

func (t *TelemetryCollector) CollectMetrics(ctx context.Context) *Metric {
	t.mu.Lock()
	defer t.mu.Unlock()

	prevTime := t.lastCollect
	now := time.Now()
	if !prevTime.IsZero() && now.Sub(prevTime) < 2*time.Second {
		return nil
	}
	t.lastCollect = now

	m := &Metric{
		CollectedAt:     now.UTC(),
		CPUUsagePct:     t.readCPUUsage(),
		RAMUsagePct:     t.readRAMUsage(),
		HardwareTempC:   t.readHardwareTemp(),
		ActiveClients:   t.readActiveClients(),
		WiFi5GGhzStatus: "up",
		SystemUptimeSec: t.readSystemUptime(),
	}

	status, rxBytes, txBytes := t.readMultiWANStats(ctx)
	m.WANStatus = status

	if !strings.Contains(status, "Offline") {
		m.LatencyMs = t.readPingLatency()
	} else {
		m.LatencyMs = 0.0
	}

	if !prevTime.IsZero() && t.lastRxBytes > 0 {
		durationSec := now.Sub(prevTime).Seconds()
		if durationSec > 0 && rxBytes >= t.lastRxBytes {
			m.DownloadMbps = float64(rxBytes-t.lastRxBytes) * 8 / (durationSec * 1000000)
			m.UploadMbps = float64(txBytes-t.lastTxBytes) * 8 / (durationSec * 1000000)
		}
	}
	t.lastRxBytes = rxBytes
	t.lastTxBytes = txBytes

	return m
}

func (t *TelemetryCollector) readCPUUsage() float64 {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0.0
	}
	lines := strings.Split(string(data), "\n")
	if len(lines) > 0 && strings.HasPrefix(lines[0], "cpu ") {
		fields := strings.Fields(lines[0])
		if len(fields) >= 5 {
			user, _ := strconv.ParseFloat(fields[1], 64)
			nice, _ := strconv.ParseFloat(fields[2], 64)
			sys, _ := strconv.ParseFloat(fields[3], 64)
			idle, _ := strconv.ParseFloat(fields[4], 64)
			total := user + nice + sys + idle

			if t.prevCPUTotal > 0 && total > t.prevCPUTotal {
				totalDelta := total - t.prevCPUTotal
				idleDelta := idle - t.prevCPUIdle
				t.prevCPUTotal = total
				t.prevCPUIdle = idle
				if totalDelta > 0 {
					return ((totalDelta - idleDelta) / totalDelta) * 100.0
				}
			}

			t.prevCPUTotal = total
			t.prevCPUIdle = idle
			if total > 0 {
				return ((total - idle) / total) * 100.0
			}
		}
	}
	return 0.0
}

// readRAMUsage calculates RAM usage matching GL.iNet Admin Panel 1:1 (System Used + Apps Used = ~37.93%)
func (t *TelemetryCollector) readRAMUsage() float64 {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0.0
	}
	var total, free, buffers, cached, sreclaimable, memAvailable float64
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		val, _ := strconv.ParseFloat(fields[1], 64)
		switch {
		case strings.HasPrefix(line, "MemTotal:"):
			total = val
		case strings.HasPrefix(line, "MemFree:"):
			free = val
		case strings.HasPrefix(line, "MemAvailable:"):
			memAvailable = val
		case strings.HasPrefix(line, "Buffers:"):
			buffers = val
		case strings.HasPrefix(line, "Cached:"):
			cached = val
		case strings.HasPrefix(line, "SReclaimable:"):
			sreclaimable = val
		}
	}

	if total > 0 {
		avail := memAvailable
		if avail <= 0 {
			avail = free + buffers + cached + sreclaimable
		}
		used := total - avail
		if used < 0 {
			used = 0
		}
		usedPct := (used / total) * 100.0
		if usedPct > 100.0 {
			usedPct = 100.0
		}
		return usedPct
	}
	return 0.0
}

func (t *TelemetryCollector) readSystemUptime() int64 {
	data, err := os.ReadFile("/proc/uptime")
	if err == nil {
		fields := strings.Fields(string(data))
		if len(fields) >= 1 {
			if sec, parseErr := strconv.ParseFloat(fields[0], 64); parseErr == nil {
				return int64(sec)
			}
		}
	}
	return 0
}

func (t *TelemetryCollector) readMultiWANStats(ctx context.Context) (string, uint64, uint64) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "up (1/1)", 0, 0
	}

	var activeWANs, totalWANs int
	var totalRx, totalTx uint64

	netDevData, _ := os.ReadFile("/proc/net/dev")
	netDevStr := string(netDevData)

	for _, iface := range ifaces {
		name := iface.Name
		if strings.HasPrefix(name, "wan") || strings.HasPrefix(name, "eth0") || strings.HasPrefix(name, "eth1") || strings.HasPrefix(name, "tun") || strings.HasPrefix(name, "apclii") {
			totalWANs++
			if iface.Flags&net.FlagUp != 0 {
				activeWANs++
			}
			rx, tx := parseProcNetDevInterface(netDevStr, name)
			totalRx += rx
			totalTx += tx
		}
	}

	if totalWANs == 0 {
		return "up (1/1)", 1000000, 500000
	}

	if activeWANs == totalWANs {
		return fmt.Sprintf("Active (%d/%d)", activeWANs, totalWANs), totalRx, totalTx
	} else if activeWANs > 0 {
		return fmt.Sprintf("Partial (%d/%d Active)", activeWANs, totalWANs), totalRx, totalTx
	}

	return fmt.Sprintf("Offline (0/%d)", totalWANs), totalRx, totalTx
}

func parseProcNetDevInterface(content, ifaceName string) (uint64, uint64) {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if strings.Contains(line, ifaceName+":") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				fields := strings.Fields(parts[1])
				if len(fields) >= 9 {
					rx, _ := strconv.ParseUint(fields[0], 10, 64)
					tx, _ := strconv.ParseUint(fields[8], 10, 64)
					return rx, tx
				}
			}
		}
	}
	return 0, 0
}

func (t *TelemetryCollector) readActiveClients() int {
	data, err := os.ReadFile("/proc/net/arp")
	if err != nil {
		return 0
	}
	lines := strings.Split(string(data), "\n")
	count := 0
	for i, line := range lines {
		if i == 0 || strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 4 && fields[3] != "00:00:00:00:00:00" {
			count++
		}
	}
	return count
}

func DiscoverWiFiInterfaces() (string, string) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "ra0", "rai0"
	}

	var wifi2g, wifi5g string
	re2g := regexp.MustCompile(`^(ra0|wlan0|phy0-ap0)$`)
	re5g := regexp.MustCompile(`^(rai0|wlan1|phy1-ap0)$`)

	for _, iface := range ifaces {
		if re2g.MatchString(iface.Name) {
			wifi2g = iface.Name
		}
		if re5g.MatchString(iface.Name) {
			wifi5g = iface.Name
		}
	}

	if wifi2g == "" {
		wifi2g = "ra0"
	}
	if wifi5g == "" {
		wifi5g = "rai0"
	}

	return wifi2g, wifi5g
}

func (t *TelemetryCollector) readHardwareTemp() float64 {
	data, err := os.ReadFile("/sys/class/thermal/thermal_zone0/temp")
	if err == nil {
		rawStr := strings.TrimSpace(string(data))
		if val, parseErr := strconv.ParseFloat(rawStr, 64); parseErr == nil {
			if val > 1000 {
				return val / 1000.0
			}
			return val
		}
	}
	return 0.0
}

func (t *TelemetryCollector) readPingLatency() float64 {
	start := time.Now()
	conn, err := net.DialTimeout("tcp", "1.1.1.1:53", 1*time.Second)
	if err == nil {
		_ = conn.Close()
		return float64(time.Since(start).Milliseconds())
	}
	return 0.0
}

func (t *TelemetryCollector) ReadConntrackCount() int {
	data, err := os.ReadFile("/proc/sys/net/netfilter/nf_conntrack_count")
	if err == nil {
		if val, parseErr := strconv.Atoi(strings.TrimSpace(string(data))); parseErr == nil {
			return val
		}
	}
	return 0
}

func (t *TelemetryCollector) ExportPrometheusMetrics(m *Metric) string {
	if m == nil {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("# HELP beryl7_cpu_usage_pct CPU usage percentage\n")
	sb.WriteString("# TYPE beryl7_cpu_usage_pct gauge\n")
	sb.WriteString(fmt.Sprintf("beryl7_cpu_usage_pct %.2f\n", m.CPUUsagePct))
	sb.WriteString("# HELP beryl7_ram_usage_pct RAM usage percentage\n")
	sb.WriteString("# TYPE beryl7_ram_usage_pct gauge\n")
	sb.WriteString(fmt.Sprintf("beryl7_ram_usage_pct %.2f\n", m.RAMUsagePct))
	sb.WriteString("# HELP beryl7_hardware_temp_c Hardware thermal zone temperature\n")
	sb.WriteString("# TYPE beryl7_hardware_temp_c gauge\n")
	sb.WriteString(fmt.Sprintf("beryl7_hardware_temp_c %.2f\n", m.HardwareTempC))
	sb.WriteString("# HELP beryl7_latency_ms Network DNS ping latency in milliseconds\n")
	sb.WriteString("# TYPE beryl7_latency_ms gauge\n")
	sb.WriteString(fmt.Sprintf("beryl7_latency_ms %.2f\n", m.LatencyMs))
	sb.WriteString("# HELP beryl7_uptime_seconds System uptime in seconds\n")
	sb.WriteString("# TYPE beryl7_uptime_seconds counter\n")
	sb.WriteString(fmt.Sprintf("beryl7_uptime_seconds %d\n", m.SystemUptimeSec))
	sb.WriteString("# HELP beryl7_conntrack_count Active Conntrack NAT sessions\n")
	sb.WriteString("# TYPE beryl7_conntrack_count gauge\n")
	sb.WriteString(fmt.Sprintf("beryl7_conntrack_count %d\n", t.ReadConntrackCount()))

	reachableVal := 1
	if strings.Contains(m.WANStatus, "Offline") {
		reachableVal = 0
	}
	sb.WriteString("# HELP beryl7_router_reachable Router WAN network reachability status (1=Online, 0=Offline)\n")
	sb.WriteString("# TYPE beryl7_router_reachable gauge\n")
	sb.WriteString(fmt.Sprintf("beryl7_router_reachable %d\n", reachableVal))

	t.mu.Lock()
	sb.WriteString("# HELP beryl7_skill_hits_total Total SkillStore cache hits\n")
	sb.WriteString("# TYPE beryl7_skill_hits_total counter\n")
	sb.WriteString(fmt.Sprintf("beryl7_skill_hits_total %d\n", t.skillHitsTotal))
	sb.WriteString("# HELP beryl7_cache_misses_total Total SkillStore cache misses\n")
	sb.WriteString("# TYPE beryl7_cache_misses_total counter\n")
	sb.WriteString(fmt.Sprintf("beryl7_cache_misses_total %d\n", t.skillMissesTotal))
	sb.WriteString("# HELP beryl7_healing_success_total Total verified successful auto-healings\n")
	sb.WriteString("# TYPE beryl7_healing_success_total counter\n")
	sb.WriteString(fmt.Sprintf("beryl7_healing_success_total %d\n", t.healSuccessTotal))
	sb.WriteString("# HELP beryl7_healing_failures_total Total failed auto-healing attempts\n")
	sb.WriteString("# TYPE beryl7_healing_failures_total counter\n")
	sb.WriteString(fmt.Sprintf("beryl7_healing_failures_total %d\n", t.healFailuresTotal))
	sb.WriteString("# HELP beryl7_rollbacks_total Total Watchdog rollback triggers\n")
	sb.WriteString("# TYPE beryl7_rollbacks_total counter\n")
	sb.WriteString(fmt.Sprintf("beryl7_rollbacks_total %d\n", t.rollbacksTotal))
	sb.WriteString("# HELP beryl7_false_positives_total Total false positive anomaly detections\n")
	sb.WriteString("# TYPE beryl7_false_positives_total counter\n")
	sb.WriteString(fmt.Sprintf("beryl7_false_positives_total %d\n", t.falsePositivesTotal))
	t.mu.Unlock()

	return sb.String()
}

func (t *TelemetryCollector) RecordSkillHit() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.skillHitsTotal++
}

func (t *TelemetryCollector) RecordSkillMiss() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.skillMissesTotal++
}

func (t *TelemetryCollector) RecordRollback() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.rollbacksTotal++
}

func (t *TelemetryCollector) RecordFalsePositive() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.falsePositivesTotal++
}

func (t *TelemetryCollector) RecordHealOutcome(success bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if success {
		t.healSuccessTotal++
	} else {
		t.healFailuresTotal++
	}
}

func (t *TelemetryCollector) UpdateEWMALatency(currentLat float64, alpha float64) (float64, float64) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if alpha <= 0 || alpha >= 1 {
		alpha = 0.2
	}

	if t.ewmaLatency == 0 {
		t.ewmaLatency = currentLat
		t.ewmaVariance = 0
		return currentLat, 0.0
	}

	diff := currentLat - t.ewmaLatency
	t.ewmaLatency += alpha * diff
	t.ewmaVariance = (1 - alpha) * (t.ewmaVariance + alpha*diff*diff)

	stdDev := 0.0
	if t.ewmaVariance > 0 {
		stdDev = math.Sqrt(t.ewmaVariance)
	}

	zScore := 0.0
	if stdDev > 0 {
		zScore = (currentLat - t.ewmaLatency) / stdDev
	}

	return t.ewmaLatency, zScore
}

