package executor

import (
	"context"
	"fmt"
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

type Executor struct {
	whitelist   map[string]ActionFunc
	riskMatrix  map[string]float64
	macRegex    *regexp.Regexp
	validIfaces map[string]bool
}

func New() *Executor {
	e := &Executor{
		riskMatrix: map[string]float64{
			"no_action_required":    0.50, // Low Risk
			"restart_wan_interface": 0.85, // Medium Risk
			"restart_interface":     0.85, // Medium Risk
			"optimize_wifi_channel": 0.85, // Medium Risk
			"set_qos_priority":      0.95, // High Risk (Requires Approval if < 0.95)
			"block_device":          0.95, // High Risk (Requires Approval if < 0.95)
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
		"restart_wan_interface": e.actionRestartWAN,
		"restart_interface":     e.actionRestartInterface,
		"optimize_wifi_channel": e.actionOptimizeWifiChannel,
		"set_qos_priority":      e.actionSetQOSPriority,
		"block_device":          e.actionBlockDevice,
		"set_wan_mac":           e.actionSetWANMAC,
	}

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
		logger.Warn("[SYSTEM WARNING] Executable %s not found on OS. Simulating success.", cleanBin)
		return nil
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

func (e *Executor) actionRestartWAN(ctx context.Context, target string, params map[string]interface{}) error {
	iface := "wan"
	if target != "" && e.validIfaces[strings.ToLower(target)] {
		iface = strings.ToLower(target)
	}
	logger.Info("Executing OpenWrt ifdown/ifup on interface [%s]...", iface)
	if err := runSystemCmd(ctx, "/sbin/ifdown", iface); err != nil {
		return err
	}
	time.Sleep(1 * time.Second)
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
	logger.Info("Executing OpenWrt interface restart on [%s]...", iface)
	_ = runSystemCmd(ctx, "/sbin/ifdown", iface)
	time.Sleep(1 * time.Second)
	return runSystemCmd(ctx, "/sbin/ifup", iface)
}

func (e *Executor) actionOptimizeWifiChannel(ctx context.Context, target string, params map[string]interface{}) error {
	band, _ := params["band"].(string)
	channelVal, _ := params["channel"]

	radioSection := "radio0"
	if strings.Contains(strings.ToLower(band), "5g") || target == "rai0" || target == "radio1" {
		radioSection = "radio1"
	}

	chStr := fmt.Sprintf("%v", channelVal)
	if ch, err := strconv.Atoi(chStr); err != nil || ch < 1 || ch > 165 {
		return fmt.Errorf("invalid Wi-Fi channel: %s", chStr)
	}

	logger.Info("Executing OpenWrt UCI Wi-Fi channel optimization: section=%s, channel=%s...", radioSection, chStr)
	_ = runSystemCmd(ctx, "/sbin/uci", "set", fmt.Sprintf("wireless.%s.channel=%s", radioSection, chStr))
	_ = runSystemCmd(ctx, "/sbin/uci", "commit", "wireless")
	return runSystemCmd(ctx, "/sbin/wifi", "reload")
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

	logger.Info("Executing Idempotent OpenWrt Firewall MAC block rule for [%s]...", mac)
	ruleName := fmt.Sprintf("block_%s", strings.ReplaceAll(mac, ":", ""))

	// Kiểm tra Idempotent: Nếu rule đã tồn tại thì không add trùng lặp gây rác flash
	checkErr := runSystemCmd(ctx, "/sbin/uci", "get", fmt.Sprintf("firewall.%s", ruleName))
	if checkErr == nil && runtime.GOOS == "linux" {
		logger.Info("Firewall rule [%s] already exists. Skipping duplicate addition.", ruleName)
		return nil
	}

	_ = runSystemCmd(ctx, "/sbin/uci", "add", "firewall", "rule")
	_ = runSystemCmd(ctx, "/sbin/uci", "set", fmt.Sprintf("firewall.@rule[-1].name=%s", ruleName))
	_ = runSystemCmd(ctx, "/sbin/uci", "set", "firewall.@rule[-1].src=lan")
	_ = runSystemCmd(ctx, "/sbin/uci", "set", "firewall.@rule[-1].dest=wan")
	_ = runSystemCmd(ctx, "/sbin/uci", "set", fmt.Sprintf("firewall.@rule[-1].src_mac=%s", mac))
	_ = runSystemCmd(ctx, "/sbin/uci", "set", "firewall.@rule[-1].target=DROP")
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
	time.Sleep(1 * time.Second)
	return runSystemCmd(ctx, "/sbin/ifup", "wan")
}
