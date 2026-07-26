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

type ActionFunc func(ctx context.Context, params map[string]string) error

type Executor struct {
	macRegex  *regexp.Regexp
	whitelist map[string]ActionFunc
}

func New() *Executor {
	e := &Executor{
		macRegex: regexp.MustCompile(`^([0-9A-Fa-f]{2}[:-]){5}([0-9A-Fa-f]{2})$`),
	}

	// Khắc phục Lỗ hổng 4: Khóa Whitelist hành động nghiêm ngặt chống Remote Code Execution
	e.whitelist = map[string]ActionFunc{
		"restart_wan_interface": func(ctx context.Context, params map[string]string) error {
			return e.restartInterface(ctx, "wan")
		},
		"restart_interface": func(ctx context.Context, params map[string]string) error {
			iface := params["interface"]
			if iface == "" {
				iface = "wan"
			}
			return e.restartInterface(ctx, iface)
		},
		"reload_wifi": func(ctx context.Context, params map[string]string) error {
			return e.reloadWiFi(ctx)
		},
		"set_wan_mac": func(ctx context.Context, params map[string]string) error {
			return e.setWANMac(ctx, params["mac"])
		},
	}

	return e
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

	// Khắc phục Lỗ hổng 4: Tra cứu Whitelist, bác bỏ 100% lệnh lạ ngoài danh mục kiểm duyệt
	actionFn, exists := e.whitelist[req.ActionName]
	if !exists {
		return fmt.Errorf("SECURITY ERROR: Action [%s] is NOT in the allowed security whitelist", req.ActionName)
	}

	return actionFn(ctxTimeout, req.Params)
}

func (e *Executor) restartInterface(ctx context.Context, iface string) error {
	// Whitelist interface name chỉ chấp nhận wan/lan/wwan
	if iface != "wan" && iface != "lan" && iface != "wwan" {
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
