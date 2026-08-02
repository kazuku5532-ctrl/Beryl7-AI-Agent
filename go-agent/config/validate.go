package config

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ValidateSystemDependencies checks if required OpenWrt CLI tools exist
func ValidateSystemDependencies() error {
	requiredTools := map[string]string{
		"uci":     "unified configuration interface",
		"ubus":    "network configuration interface",
		"logread": "system log interface",
	}

	var missing []string
	for tool, desc := range requiredTools {
		if _, err := exec.LookPath(tool); err != nil {
			missing = append(missing, fmt.Sprintf("%s (%s)", tool, desc))
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing system tools:\n  - %s", strings.Join(missing, "\n  - "))
	}
	return nil
}

// ValidateSystemConfiguration checks configuration values and path permissions
func ValidateSystemConfiguration(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("configuration is nil")
	}
	var errs []string
	if cfg.AuthToken == "" {
		errs = append(errs, "AUTH_TOKEN is required")
	}
	if cfg.ApproveToken != "" && cfg.ApproveToken == cfg.AuthToken {
		errs = append(errs, "APPROVE_TOKEN must be distinct from AUTH_TOKEN")
	}
	for _, path := range []string{cfg.SkillStorePath, cfg.CheckpointPath} {
		dir := filepath.Dir(path)
		if dir != "" && dir != "." {
			_ = os.MkdirAll(dir, 0700)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("configuration validation failed:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return nil
}
