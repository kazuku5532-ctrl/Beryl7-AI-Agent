package executor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"beryl7-agent/logger"
)

type ActionRequest struct {
	ActionName string                 `json:"action_name"`
	Target     string                 `json:"target"`
	Parameters map[string]interface{} `json:"parameters"`
}

type ActionFunc func(ctx context.Context, target string, params map[string]interface{}) error

type TelemetryProvider interface {
	AreWiFiClientsIdle(ctx context.Context) (bool, int, error)
}

type Executor struct {
	whitelist   map[string]ActionFunc
	riskMatrix  map[string]float64
	macRegex    *regexp.Regexp
	validIfaces map[string]bool
	telemetry   TelemetryProvider
}

func (e *Executor) SetTelemetryProvider(tp TelemetryProvider) {
	e.telemetry = tp
}

func New() *Executor {
	e := &Executor{
		riskMatrix: map[string]float64{
			"no_action_required":    0.50, // Low Risk
			"purge_memory_cache":    0.60, // Low-Medium Risk
			"restart_wan_interface": 0.85, // Medium Risk
			"restart_interface":     0.85, // Medium Risk
			"optimize_wifi_channel": 0.85, // Medium Risk
			"scale_tx_power_down":   0.85, // Medium Risk (Repeater Guard)
			"align_channels":        0.85, // Medium Risk (Repeater Guard)
			"ap_failover":           0.90, // Medium-High Risk (Repeater Guard)
			"set_qos_priority":      0.95, // High Risk
			"block_device":          0.95, // High Risk
			"set_wan_mac":           0.98, // Critical Risk
		},
		macRegex: regexp.MustCompile(`^([0-9A-Fa-f]{2}[:-]){5}([0-9A-Fa-f]{2})$`),
		validIfaces: map[string]bool{
			"wan":    true,
			"lan":    true,
			"wwan":   true,
			"br-lan": true,
			"eth0":   true,
			"eth1":   true,
			"ra0":    true,
			"rai0":   true,
			"wlan0":  true,
			"wlan1":  true,
		},
	}

	e.whitelist = map[string]ActionFunc{
		"no_action_required":    e.actionNoActionRequired,
		"purge_memory_cache":    e.actionPurgeMemoryCache,
		"restart_wan_interface": e.actionRestartWAN,
		"restart_interface":     e.actionRestartInterface,
		"optimize_wifi_channel": e.actionOptimizeWifiChannel,
		"set_qos_priority":      e.actionSetQOSPriority,
		"block_device":          e.actionBlockDevice,
		"set_wan_mac":           e.actionSetWANMAC,
		"boost_wifi_bandwidth":     e.actionBoostWifiBandwidth,
		"revert_wifi_bandwidth":    e.actionRevertWifiBandwidth,
		"tune_network_performance": e.actionTuneNetworkPerformance,
		"optimize_streaming_pipeline": e.actionOptimizeStreamingPipeline,
		"scale_tx_power_down":      e.actionScaleTxPowerDown,
		"align_channels":           e.actionAlignChannels,
		"ap_failover":              e.actionAPFailover,
		"enable_cake_sqm":          e.actionEnableCAKESQM,
	}

	e.riskMatrix["optimize_streaming_pipeline"] = 0.40 // Low Risk streaming pipeline tuning

	return e
}

func (e *Executor) GetActionRiskThreshold(actionName string) float64 {
	if threshold, exists := e.riskMatrix[actionName]; exists {
		return threshold
	}
	return 0.90
}

func (e *Executor) ExecuteAction(ctx context.Context, req *ActionRequest, dryRun bool) error {
	if req == nil || req.ActionName == "" {
		return fmt.Errorf("invalid action request: empty action name")
	}

	actionFn, exists := e.whitelist[req.ActionName]
	if !exists {
		logger.Error("SECURITY ERROR: Action [%s] is NOT in Whitelist!", req.ActionName)
		return fmt.Errorf("action '%s' is rejected by strict whitelist policy", req.ActionName)
	}

	if dryRun {
		logger.Info("[DRY-RUN] Execution Skipped: Action [%s] on Target [%s]", req.ActionName, req.Target)
		return nil
	}

	logger.Info("Executing Whitelisted Action [%s] on Target [%s]...", req.ActionName, req.Target)
	return actionFn(ctx, req.Target, req.Parameters)
}

func runSystemCmd(ctx context.Context, binPath string, args ...string) error {
	if runtime.GOOS != "linux" {
		logger.Info("[DEV SIMULATION] Executed command: %s %v", binPath, args)
		return nil
	}

	cleanBin := filepath.Clean(binPath)
	if _, err := exec.LookPath(cleanBin); err != nil {
		logger.Error("[SYSTEM ERROR] Required OpenWrt executable %s not found on Linux!", cleanBin)
		return fmt.Errorf("executable %s not found on system path", cleanBin)
	}

	ctxTimeout, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctxTimeout, cleanBin, args...) // #nosec G204
	out, err := cmd.CombinedOutput()
	if err != nil {
		logger.Error("System command failed (%s %v): %v, Output: %s", cleanBin, args, err, string(out))
		return fmt.Errorf("command %s failed: %w", cleanBin, err)
	}
	logger.Info("System command output (%s): %s", cleanBin, strings.TrimSpace(string(out)))
	return nil
}

func (e *Executor) actionNoActionRequired(ctx context.Context, target string, params map[string]interface{}) error {
	logger.Info("Action: System healthy. No action required.")
	return nil
}

func (e *Executor) actionPurgeMemoryCache(ctx context.Context, target string, params map[string]interface{}) error {
	logger.Info("Executing Linux Kernel RAM Page Cache Purge (echo 3 > /proc/sys/vm/drop_caches)...")
	return runSystemCmd(ctx, "/sbin/sysctl", "-w", "vm.drop_caches=3")
}

func (e *Executor) actionRestartWAN(ctx context.Context, target string, params map[string]interface{}) error {
	iface := "wan"
	if target != "" && e.validIfaces[strings.ToLower(target)] {
		iface = strings.ToLower(target)
	}
	logger.Info("Executing OpenWrt ifdown/ifup on interface [%s]...", iface)
	if err := runSystemCmd(ctx, "/sbin/ifdown", iface); err != nil {
		return err
	}
	select {
	case <-time.After(1 * time.Second):
	case <-ctx.Done():
		return ctx.Err()
	}
	return runSystemCmd(ctx, "/sbin/ifup", iface)
}

func (e *Executor) actionRestartInterface(ctx context.Context, target string, params map[string]interface{}) error {
	iface := strings.ToLower(target)
	if iface == "" {
		iface, _ = params["interface_name"].(string)
		iface = strings.ToLower(iface)
	}
	if iface == "" || !e.validIfaces[iface] {
		return fmt.Errorf("unapproved interface target: %s", iface)
	}
	logger.Info("ZERO-DISRUPTION RECOVERY: Attempting non-disruptive OpenWrt interface refresh on [%s] via ifup...", iface)
	if err := runSystemCmd(ctx, "/sbin/ifup", iface); err == nil {
		return nil
	}
	logger.Warn("Interface [%s] ifup failed. Executing fallback restart via ifdown/ifup...", iface)
	_ = runSystemCmd(ctx, "/sbin/ifdown", iface) // nolint:errcheck (non-fatal, ifup follows)
	select {
	case <-time.After(500 * time.Millisecond):
	case <-ctx.Done():
		return ctx.Err()
	}
	return runSystemCmd(ctx, "/sbin/ifup", iface)
}

func (e *Executor) checkWiFiIdleGuard(ctx context.Context) error {
	if e.telemetry != nil {
		isIdle, activeClients, err := e.telemetry.AreWiFiClientsIdle(ctx)
		if err == nil && (!isIdle || activeClients > 0) {
			logger.Warn("ZERO-DISRUPTION GUARD: Wi-Fi reload blocked and deferred - %d active client(s) connected/transferring traffic.", activeClients)
			return fmt.Errorf("wifi reload deferred: active client(s) connected/transferring traffic (%d clients)", activeClients)
		}
	}
	return nil
}

func (e *Executor) actionOptimizeWifiChannel(ctx context.Context, target string, params map[string]interface{}) error {
	if err := e.checkWiFiIdleGuard(ctx); err != nil {
		return err
	}

	band, _ := params["band"].(string)
	channelVal, _ := params["channel"]

	radioSection := "MT7993_1_1"
	if strings.Contains(strings.ToLower(band), "5g") || target == "rai0" || target == "radio1" || target == "MT7993_1_2" {
		radioSection = "MT7993_1_2"
	}

	chStr := ""
	if channelVal != nil {
		chStr = fmt.Sprintf("%v", channelVal)
	}
	if chStr == "" || chStr == "<nil>" {
		if radioSection == "MT7993_1_2" {
			chStr = "36"
		} else {
			chStr = "6"
		}
	}

	if ch, err := strconv.Atoi(chStr); err != nil || ch < 1 || ch > 165 {
		return fmt.Errorf("invalid Wi-Fi channel: %s", chStr)
	}

	logger.Info("Executing OpenWrt UCI Wi-Fi channel optimization: section=%s, channel=%s...", radioSection, chStr)
	_ = runSystemCmd(ctx, "/sbin/uci", "set", fmt.Sprintf("wireless.%s.channel=%s", radioSection, chStr)) // nolint:errcheck (non-fatal)
	_ = runSystemCmd(ctx, "/sbin/uci", "commit", "wireless")                                              // nolint:errcheck (non-fatal)
	return nil
}

func (e *Executor) actionSetQOSPriority(ctx context.Context, target string, params map[string]interface{}) error {
	mac, _ := params["target_mac"].(string)
	priority, _ := params["priority"].(string)

	if mac != "" && !e.macRegex.MatchString(mac) {
		return fmt.Errorf("invalid MAC address format: %s", mac)
	}

	logger.Info("Executing UCI SQM QoS Priority tuning for MAC [%s] (Priority=%s)...", mac, priority)
	_ = runSystemCmd(ctx, "/sbin/uci", "set", "sqm.wan.enabled=1")
	if strings.ToUpper(priority) == "HIGH" {
		_ = runSystemCmd(ctx, "/sbin/uci", "set", "sqm.wan.qdisc=fq_codel")
		_ = runSystemCmd(ctx, "/sbin/uci", "set", "sqm.wan.script=layer7.qos")
	}
	_ = runSystemCmd(ctx, "/sbin/uci", "commit", "sqm")
	return runSystemCmd(ctx, "/etc/init.d/sqm", "reload")
}

func (e *Executor) actionBlockDevice(ctx context.Context, target string, params map[string]interface{}) error {
	mac, _ := params["target_mac"].(string)
	if mac == "" {
		mac = target
	}
	if !e.macRegex.MatchString(mac) {
		return fmt.Errorf("invalid MAC address format: %s", mac)
	}

	ruleSection := fmt.Sprintf("block_%s", strings.ReplaceAll(mac, ":", ""))
	logger.Info("Executing Idempotent UCI Named Section Firewall MAC block rule [%s] for [%s]...", ruleSection, mac)

	// Kiểm tra Idempotent chuẩn OpenWrt UCI Named Section: uci get firewall.<ruleSection>
	checkErr := runSystemCmd(ctx, "/sbin/uci", "get", fmt.Sprintf("firewall.%s", ruleSection))
	if checkErr == nil && runtime.GOOS == "linux" {
		logger.Info("Firewall named section [%s] already exists. Skipping duplicate addition.", ruleSection)
		return nil
	}

	_ = runSystemCmd(ctx, "/sbin/uci", "set", fmt.Sprintf("firewall.%s=rule", ruleSection))
	_ = runSystemCmd(ctx, "/sbin/uci", "set", fmt.Sprintf("firewall.%s.name=%s", ruleSection, ruleSection))
	_ = runSystemCmd(ctx, "/sbin/uci", "set", fmt.Sprintf("firewall.%s.src=lan", ruleSection))
	_ = runSystemCmd(ctx, "/sbin/uci", "set", fmt.Sprintf("firewall.%s.dest=wan", ruleSection))
	_ = runSystemCmd(ctx, "/sbin/uci", "set", fmt.Sprintf("firewall.%s.src_mac=%s", ruleSection, mac))
	_ = runSystemCmd(ctx, "/sbin/uci", "set", fmt.Sprintf("firewall.%s.target=DROP", ruleSection))
	_ = runSystemCmd(ctx, "/sbin/uci", "commit", "firewall")
	return runSystemCmd(ctx, "/etc/init.d/firewall", "reload")
}

func (e *Executor) actionSetWANMAC(ctx context.Context, target string, params map[string]interface{}) error {
	mac, _ := params["mac"].(string)
	if !e.macRegex.MatchString(mac) {
		return fmt.Errorf("invalid MAC address: %s", mac)
	}

	logger.Info("Executing UCI WAN MAC address modification to [%s]...", mac)
	_ = runSystemCmd(ctx, "/sbin/uci", "set", fmt.Sprintf("network.wan.macaddr=%s", mac))
	_ = runSystemCmd(ctx, "/sbin/uci", "commit", "network")
	_ = runSystemCmd(ctx, "/sbin/ifdown", "wan")
	select {
	case <-time.After(1 * time.Second):
	case <-ctx.Done():
		return ctx.Err()
	}
	return runSystemCmd(ctx, "/sbin/ifup", "wan")
}

func (e *Executor) actionBoostWifiBandwidth(ctx context.Context, target string, params map[string]interface{}) error {
	if err := e.checkWiFiIdleGuard(ctx); err != nil {
		return err
	}
	logger.Info("DYNAMIC BOOST TRIGGERED: Preparing 160MHz Max Wi-Fi 7 Bandwidth on MT7993_1_2...")
	_ = runSystemCmd(ctx, "/sbin/uci", "set", "wireless.MT7993_1_2.htmode=EHT160")
	_ = runSystemCmd(ctx, "/sbin/uci", "set", "wireless.MT7993_1_2.noscan=1")
	return runSystemCmd(ctx, "/sbin/uci", "commit", "wireless")
}

func (e *Executor) actionRevertWifiBandwidth(ctx context.Context, target string, params map[string]interface{}) error {
	if err := e.checkWiFiIdleGuard(ctx); err != nil {
		return err
	}
	logger.Info("DYNAMIC BOOST COMPLETED: Reverting Wi-Fi 7 to Eco 80MHz Mode on MT7993_1_2...")
	_ = runSystemCmd(ctx, "/sbin/uci", "set", "wireless.MT7993_1_2.htmode=HE80")
	_ = runSystemCmd(ctx, "/sbin/uci", "set", "wireless.MT7993_1_2.noscan=0")
	return runSystemCmd(ctx, "/sbin/uci", "commit", "wireless")
}

func (e *Executor) actionTuneNetworkPerformance(ctx context.Context, target string, params map[string]interface{}) error {
	logger.Info("TUNING NETWORK PERFORMANCE: Enabling Hardware NAT & Flow Offloading, Maxing TCP Socket Buffers & A-MPDU Aggregation...")
	_ = runSystemCmd(ctx, "/sbin/sysctl", "-w", "net.core.rmem_max=16777216")
	_ = runSystemCmd(ctx, "/sbin/sysctl", "-w", "net.core.wmem_max=16777216")
	_ = runSystemCmd(ctx, "/sbin/sysctl", "-w", "net.core.rmem_default=262144")
	_ = runSystemCmd(ctx, "/sbin/sysctl", "-w", "net.core.wmem_default=262144")
	_ = runSystemCmd(ctx, "/sbin/sysctl", "-w", "net.ipv4.tcp_rmem=4096 87380 16777216")
	_ = runSystemCmd(ctx, "/sbin/sysctl", "-w", "net.ipv4.tcp_wmem=4096 65536 16777216")
	_ = runSystemCmd(ctx, "/sbin/sysctl", "-w", "net.core.netdev_max_backlog=10000")
	_ = runSystemCmd(ctx, "/sbin/sysctl", "-w", "net.ipv4.tcp_max_syn_backlog=8192")
	_ = runSystemCmd(ctx, "/sbin/sysctl", "-w", "net.ipv4.tcp_window_scaling=1")
	_ = runSystemCmd(ctx, "/sbin/sysctl", "-w", "net.ipv4.tcp_sack=1")
	_ = runSystemCmd(ctx, "/sbin/sysctl", "-w", "net.ipv4.tcp_fastopen=3")
	_ = runSystemCmd(ctx, "/sbin/sysctl", "-w", "net.ipv4.tcp_slow_start_after_idle=0")                      // Prevents HLS / m3u8 browser video stalls
	_ = runSystemCmd(ctx, "/sbin/sysctl", "-w", "net.ipv4.tcp_mtu_probing=1")                              // Prevents black-hole packet drops on video CDNs
	_ = runSystemCmd(ctx, "/sbin/sysctl", "-w", "net.ipv4.tcp_adv_win_scale=1")                            // Optimizes buffer window for video streaming
	_ = runSystemCmd(ctx, "/sbin/sysctl", "-w", "net.ipv4.tcp_moderate_rcvbuf=1")
	_ = runSystemCmd(ctx, "/sbin/sysctl", "-w", "net.ipv4.tcp_autocorking=0")
	_ = runSystemCmd(ctx, "/sbin/sysctl", "-w", "net.ipv4.tcp_early_retrans=3")
	_ = runSystemCmd(ctx, "/sbin/sysctl", "-w", "net.netfilter.nf_conntrack_max=65536")                     // nolint:errcheck
	_ = runSystemCmd(ctx, "/sbin/sysctl", "-w", "net.ipv4.tcp_fin_timeout=15")                              // nolint:errcheck
	_ = runSystemCmd(ctx, "/sbin/sysctl", "-w", "net.netfilter.nf_conntrack_tcp_timeout_close_wait=10")     // nolint:errcheck
	_ = runSystemCmd(ctx, "/sbin/sysctl", "-w", "net.netfilter.nf_conntrack_tcp_timeout_fin_wait=10")        // nolint:errcheck

	// OpenWrt Hardware & Software Flow Offloading (MediaTek PPE / HNAT Acceleration)
	_ = runSystemCmd(ctx, "/sbin/uci", "set", "firewall.@defaults[0].flow_offloading=1")     // nolint:errcheck
	_ = runSystemCmd(ctx, "/sbin/uci", "set", "firewall.@defaults[0].flow_offloading_hw=1")  // nolint:errcheck

	// OpenWrt fw4/nftables compatible MTU Fix (MSS Clamping)
	_ = runSystemCmd(ctx, "/sbin/uci", "set", "firewall.wan.mtu_fix=1")       // nolint:errcheck
	_ = runSystemCmd(ctx, "/sbin/uci", "set", "firewall.@zone[1].mtu_fix=1") // nolint:errcheck
	_ = runSystemCmd(ctx, "/sbin/uci", "commit", "firewall")                 // nolint:errcheck
	if _, err := exec.LookPath("/sbin/fw4"); err == nil {
		_ = runSystemCmd(ctx, "/sbin/fw4", "reload") // nolint:errcheck
	} else if _, err := exec.LookPath("/etc/init.d/firewall"); err == nil {
		_ = runSystemCmd(ctx, "/etc/init.d/firewall", "reload") // nolint:errcheck
	}

	// Remove any UDP 443 reject rules to allow native high-speed QUIC/HTTP3 streaming
	_ = runSystemCmd(ctx, "/usr/sbin/iptables", "-D", "FORWARD", "-p", "udp", "--dport", "443", "-j", "REJECT", "--reject-with", "icmp-port-unreachable") // nolint:errcheck

	// TCP MSS Clamping to prevent video buffer stalling
	_ = runSystemCmd(ctx, "/usr/sbin/iptables", "-t", "mangle", "-D", "FORWARD", "-p", "tcp", "--tcp-flags", "SYN,RST", "SYN", "-j", "TCPMSS", "--clamp-mss-to-pmtu") // nolint:errcheck
	_ = runSystemCmd(ctx, "/usr/sbin/iptables", "-t", "mangle", "-I", "FORWARD", "-p", "tcp", "--tcp-flags", "SYN,RST", "SYN", "-j", "TCPMSS", "--clamp-mss-to-pmtu") // nolint:errcheck
	_ = runSystemCmd(ctx, "/usr/sbin/iptables", "-t", "mangle", "-D", "POSTROUTING", "-p", "tcp", "--tcp-flags", "SYN,RST", "SYN", "-j", "TCPMSS", "--clamp-mss-to-pmtu") // nolint:errcheck
	_ = runSystemCmd(ctx, "/usr/sbin/iptables", "-t", "mangle", "-I", "POSTROUTING", "-p", "tcp", "--tcp-flags", "SYN,RST", "SYN", "-j", "TCPMSS", "--clamp-mss-to-pmtu") // nolint:errcheck

	_ = runSystemCmd(ctx, "/sbin/uci", "set", "wireless.MT7993_1_2.ampdu=1")
	_ = runSystemCmd(ctx, "/sbin/uci", "set", "wireless.MT7993_1_2.wmm=1")
	_ = runSystemCmd(ctx, "/sbin/uci", "set", "wireless.MT7993_1_1.ampdu=1")
	_ = runSystemCmd(ctx, "/sbin/uci", "set", "wireless.MT7993_1_1.wmm=1")
	return runSystemCmd(ctx, "/sbin/uci", "commit", "wireless")
}

func (e *Executor) actionOptimizeStreamingPipeline(ctx context.Context, target string, params map[string]interface{}) error {
	logger.Info("SMART VIDEO STREAMING PIPELINE: Optimizing video chunk delivery, unthrottled TCP window scaling & WMM Video QoS...")
	_ = runSystemCmd(ctx, "/sbin/sysctl", "-w", "net.ipv4.tcp_slow_start_after_idle=0")
	_ = runSystemCmd(ctx, "/sbin/sysctl", "-w", "net.ipv4.tcp_adv_win_scale=1")
	_ = runSystemCmd(ctx, "/sbin/sysctl", "-w", "net.ipv4.tcp_autocorking=0")
	_ = runSystemCmd(ctx, "/sbin/sysctl", "-w", "net.ipv4.tcp_early_retrans=3")

	// Ensure Apple iOS Private Relay NXDOMAIN rules are active to prevent iPhone video stalling
	appleConf := "/tmp/dnsmasq.d/apple_private_relay.conf"
	if _, err := os.Stat("/tmp/dnsmasq.d"); err == nil {
		if _, err := os.Stat(appleConf); os.IsNotExist(err) {
			_ = os.WriteFile(appleConf, []byte("server=/mask.icloud.com/\nserver=/mask-h2.icloud.com/\n"), 0600)
			_ = runSystemCmd(ctx, "/etc/init.d/dnsmasq", "reload")
		}
	}

	_ = runSystemCmd(ctx, "/sbin/uci", "set", "wireless.MT7993_1_2.wmm=1")
	_ = runSystemCmd(ctx, "/sbin/uci", "set", "wireless.MT7993_1_2.ampdu=1")
	return runSystemCmd(ctx, "/sbin/uci", "commit", "wireless")
}

func (e *Executor) TriggerWiFiReload(ctx context.Context) error {
	if err := e.checkWiFiIdleGuard(ctx); err != nil {
		logger.Warn("REPEATER GUARD: wifi reload postponed by WiFi Idle Guard (Friction Penalty 3.0x active)")
		return err
	}
	logger.Info("REPEATER GUARD: Executing wifi reload under safe idle window...")
	return runSystemCmd(ctx, "wifi", "reload")
}

func (e *Executor) actionScaleTxPowerDown(ctx context.Context, target string, params map[string]interface{}) error {
	logger.Info("REPEATER GUARD: Reducing internal Wi-Fi Tx-Power to 12dBm to suppress self-interference...")
	_ = runSystemCmd(ctx, "/sbin/uci", "set", "wireless.default_radio0.txpower=12")
	_ = runSystemCmd(ctx, "/sbin/uci", "set", "wireless.default_radio1.txpower=12")
	_ = runSystemCmd(ctx, "/sbin/uci", "commit", "wireless")
	return e.TriggerWiFiReload(ctx)
}

func (e *Executor) actionAlignChannels(ctx context.Context, target string, params map[string]interface{}) error {
	logger.Info("REPEATER GUARD: Re-aligning internal Wi-Fi LAN channel to clean non-overlapping Channel 149...")
	_ = runSystemCmd(ctx, "/sbin/uci", "set", "wireless.radio1.channel=149")
	_ = runSystemCmd(ctx, "/sbin/uci", "commit", "wireless")
	return e.TriggerWiFiReload(ctx)
}

func (e *Executor) actionAPFailover(ctx context.Context, target string, params map[string]interface{}) error {
	bssid, _ := params["bssid"].(string)
	if bssid == "" || !e.macRegex.MatchString(bssid) {
		logger.Warn("REPEATER GUARD: ap_failover rejected: missing or invalid target BSSID address")
		return fmt.Errorf("ap_failover: missing or invalid target BSSID address")
	}
	logger.Info("REPEATER GUARD: Roaming/Failing over to stronger BSSID AP [%s]...", bssid)
	_ = runSystemCmd(ctx, "/sbin/uci", "set", fmt.Sprintf("wireless.@wifi-iface[0].bssid=%s", bssid))
	_ = runSystemCmd(ctx, "/sbin/uci", "commit", "wireless")
	return e.TriggerWiFiReload(ctx)
}

func (e *Executor) actionEnableCAKESQM(ctx context.Context, target string, params map[string]interface{}) error {
	downKbps := "200000"
	upKbps := "200000"

	if d, ok := params["download_kbps"].(string); ok && d != "" {
		if _, err := strconv.Atoi(d); err == nil {
			downKbps = d
		} else {
			logger.Warn("SQM: Invalid non-numeric download rate '%s', defaulting to 200000", d)
		}
	}

	if u, ok := params["upload_kbps"].(string); ok && u != "" {
		if _, err := strconv.Atoi(u); err == nil {
			upKbps = u
		} else {
			logger.Warn("SQM: Invalid non-numeric upload rate '%s', defaulting to 200000", u)
		}
	}

	logger.Info("EXECUTING CAKE/FQ-CODEL SQM: Eliminating Bufferbloat (shaper limits: DL=%s Kbps, UL=%s Kbps)...", downKbps, upKbps)
	_ = runSystemCmd(ctx, "/sbin/uci", "set", "sqm.wan.enabled=1")
	_ = runSystemCmd(ctx, "/sbin/uci", "set", "sqm.wan.interface=wan")
	_ = runSystemCmd(ctx, "/sbin/uci", "set", "sqm.wan.qdisc=cake")
	_ = runSystemCmd(ctx, "/sbin/uci", "set", "sqm.wan.script=piece_of_cake.qos")
	_ = runSystemCmd(ctx, "/sbin/uci", "set", fmt.Sprintf("sqm.wan.download=%s", downKbps))
	_ = runSystemCmd(ctx, "/sbin/uci", "set", fmt.Sprintf("sqm.wan.upload=%s", upKbps))

	// Update anonymous section @sqm[0] for OpenWrt default config compatibility
	_ = runSystemCmd(ctx, "/sbin/uci", "set", "sqm.@sqm[0].enabled=1")
	_ = runSystemCmd(ctx, "/sbin/uci", "set", "sqm.@sqm[0].interface=wan")
	_ = runSystemCmd(ctx, "/sbin/uci", "set", "sqm.@sqm[0].qdisc=cake")
	_ = runSystemCmd(ctx, "/sbin/uci", "set", "sqm.@sqm[0].script=piece_of_cake.qos")
	_ = runSystemCmd(ctx, "/sbin/uci", "set", fmt.Sprintf("sqm.@sqm[0].download=%s", downKbps))
	_ = runSystemCmd(ctx, "/sbin/uci", "set", fmt.Sprintf("sqm.@sqm[0].upload=%s", upKbps))
	_ = runSystemCmd(ctx, "/sbin/uci", "commit", "sqm")
	_ = runSystemCmd(ctx, "/sbin/sysctl", "-w", "net.core.default_qdisc=fq_codel")

	if _, err := exec.LookPath("/etc/init.d/sqm"); err == nil {
		_ = runSystemCmd(ctx, "/etc/init.d/sqm", "enable") // nolint:errcheck
		// Zero-disruption: Use reload first for seamless in-kernel shaper updates without link drops
		if reloadErr := runSystemCmd(ctx, "/etc/init.d/sqm", "reload"); reloadErr != nil {
			_ = runSystemCmd(ctx, "/etc/init.d/sqm", "restart") // nolint:errcheck
		}
	}
	return nil
}
