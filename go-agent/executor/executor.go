package executor

import (
	"context"
	"fmt"
	"regexp"
	"strings"

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
			"optimize_wifi_channel": 0.85, // Medium Risk
			"set_qos_priority":      0.95, // High Risk (Requires Approval if < 0.95)
			"block_device":          0.95, // High Risk (Requires Approval if < 0.95)
			"set_wan_mac":           0.98, // Critical Risk
		},
		macRegex: regexp.MustCompile(`^([0-9A-Fa-f]{2}[:-]){5}([0-9A-Fa-f]{2})$`),
		validIfaces: map[string]bool{
			"wan":      true,
			"lan":      true,
			"wwan":     true,
			"br-lan":   true,
			"eth0":     true,
			"eth1":     true,
			"ra0":      true,
			"rai0":     true,
			"wlan0":    true,
			"wlan1":    true,
		},
	}

	// Đăng ký danh mục Whitelist mã hóa
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
	return 0.90 // Default High Risk Threshold cho các action không có trong danh mục
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

func (e *Executor) actionNoActionRequired(ctx context.Context, target string, params map[string]interface{}) error {
	logger.Info("Action: No action required.")
	return nil
}

func (e *Executor) actionRestartWAN(ctx context.Context, target string, params map[string]interface{}) error {
	if target != "" && !e.validIfaces[strings.ToLower(target)] {
		return fmt.Errorf("invalid interface target: %s", target)
	}
	logger.Info("Restarting WAN Interface safely via standard command...")
	return nil
}

func (e *Executor) actionRestartInterface(ctx context.Context, target string, params map[string]interface{}) error {
	iface := strings.ToLower(target)
	if !e.validIfaces[iface] {
		return fmt.Errorf("unapproved interface target: %s", iface)
	}
	logger.Info("Restarting interface [%s]...", iface)
	return nil
}

func (e *Executor) actionOptimizeWifiChannel(ctx context.Context, target string, params map[string]interface{}) error {
	logger.Info("Optimizing Wi-Fi channel...")
	return nil
}

func (e *Executor) actionSetQOSPriority(ctx context.Context, target string, params map[string]interface{}) error {
	logger.Info("Setting QoS priority...")
	return nil
}

func (e *Executor) actionBlockDevice(ctx context.Context, target string, params map[string]interface{}) error {
	mac, _ := params["target_mac"].(string)
	if mac != "" && !e.macRegex.MatchString(mac) {
		return fmt.Errorf("invalid MAC address format: %s", mac)
	}
	logger.Info("Blocking MAC device [%s]...", mac)
	return nil
}

func (e *Executor) actionSetWANMAC(ctx context.Context, target string, params map[string]interface{}) error {
	mac, _ := params["mac"].(string)
	if !e.macRegex.MatchString(mac) {
		return fmt.Errorf("invalid MAC address: %s", mac)
	}
	logger.Info("Setting WAN MAC address to [%s]...", mac)
	return nil
}
