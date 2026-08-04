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
		errs = append(errs, "AUTH_TOKEN is required -> Fix: Generate 64-hex token using 'openssl rand -hex 32' and set AUTH_TOKEN in /etc/beryl7/agent.env")
	} else if len(cfg.AuthToken) < 32 {
		errs = append(errs, "AUTH_TOKEN is too weak (< 32 chars) -> Fix: Generate 64-hex token using 'openssl rand -hex 32'")
	}
	if cfg.ApproveToken != "" && cfg.ApproveToken == cfg.AuthToken {
		errs = append(errs, "APPROVE_TOKEN must be distinct from AUTH_TOKEN -> Fix: Use a separate 64-hex token for Operator approval")
	}
	for _, path := range []string{cfg.SkillStorePath, cfg.CheckpointPath} {
		dir := filepath.Dir(path)
		if dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0700); err != nil {
				errs = append(errs, fmt.Sprintf("Cannot create directory %s: %v -> Fix: Check disk space and permissions for %s", dir, err, dir))
			}
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("configuration validation failed:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return nil
}
