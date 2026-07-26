package executor

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"beryl7-agent/logger"
	"beryl7-agent/watchdog"
)

type ActionRequest struct {
	ActionName string            `json:"action_name"`
	Target     string            `json:"target"`
	Params     map[string]string `json:"params"`
}

type Executor struct {
	macRegex *regexp.Regexp
}

func New() *Executor {
	return &Executor{
		macRegex: regexp.MustCompile(`^([0-9A-Fa-f]{2}[:-]){5}([0-9A-Fa-f]{2})$`),
	}
}

// ExecuteAction thực thi hành động can thiệp mạng không qua shell trung gian (No-Shell Parameterized Exec)
func (e *Executor) ExecuteAction(ctx context.Context, req *ActionRequest, dryRun bool) error {
	if req == nil {
		return errors.New("action request is nil")
	}

	logger.Info("Executor: Running action [%s] on target [%s] (DryRun=%v)", req.ActionName, req.Target, dryRun)

	if dryRun {
		logger.Info("DryRun Mode: Action [%s] skipped modification.", req.ActionName)
		return nil
	}

	ctxTimeout, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	switch req.ActionName {
	case "restart_wan_interface", "restart_interface":
		return e.restartInterface(ctxTimeout, "wan")
	case "reload_wifi":
		return e.reloadWiFi(ctxTimeout)
	case "set_wan_mac":
		mac := req.Params["mac"]
		return e.setWANMac(ctxTimeout, mac)
	default:
		return e.restartInterface(ctxTimeout, "wan")
	}
}

func (e *Executor) restartInterface(ctx context.Context, iface string) error {
	if iface == "" {
		iface = "wan"
	}

	// 1. Kiểm tra tiền cú pháp uci TRƯỚC KHI thực thi
	if err := watchdog.UCISyntaxPreCheck(); err != nil {
		return fmt.Errorf("UCI syntax check failed before restart: %w", err)
	}

	logger.Info("Executing /sbin/ifup %s...", iface)
	cmd := exec.CommandContext(ctx, "/sbin/ifup", iface)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ifup failed: %s (%v)", string(output), err)
	}

	return nil
}

func (e *Executor) reloadWiFi(ctx context.Context) error {
	logger.Info("Executing /sbin/wifi reload...")
	cmd := exec.CommandContext(ctx, "/sbin/wifi", "reload")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("wifi reload failed: %s (%v)", string(output), err)
	}
	return nil
}

func (e *Executor) setWANMac(ctx context.Context, mac string) error {
	mac = strings.TrimSpace(mac)
	if !e.macRegex.MatchString(mac) {
		return fmt.Errorf("invalid MAC address format: %s (must be XX:XX:XX:XX:XX:XX)", mac)
	}

	// Direct uci set parameter, NOT through shell interpreter
	cmdSet := exec.CommandContext(ctx, "uci", "set", fmt.Sprintf("network.wan.macaddr=%s", mac))
	if err := cmdSet.Run(); err != nil {
		return fmt.Errorf("uci set mac failed: %w", err)
	}

	if err := watchdog.UCISyntaxPreCheck(); err != nil {
		return fmt.Errorf("UCI syntax check failed after set MAC: %w", err)
	}

	cmdCommit := exec.CommandContext(ctx, "uci", "commit", "network")
	if err := cmdCommit.Run(); err != nil {
		return fmt.Errorf("uci commit network failed: %w", err)
	}

	return e.restartInterface(ctx, "wan")
}
