package config

import (
	"bufio"
	"flag"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"beryl7-agent/logger"
)

type Config struct {
	ConfigFilePath    string
	KeyFilePath       string
	GeminiAPIKey      string
	AuthToken         string
	ApproveToken      string
	LogLevel          string
	HealthPort        int
	TelemetryInterval time.Duration
	EMAAlpha          float64
	DryRun            bool
	CheckpointPath    string
	SkillStorePath    string
	DisableAutoHeal   bool
	apiKeyAtomic      atomic.Value
}

func LoadConfig() (*Config, error) {
	cfg := &Config{
		ConfigFilePath:    "/etc/beryl7/agent.env",
		KeyFilePath:       "/etc/beryl7/agent.key",
		AuthToken:         "",
		ApproveToken:      "",
		LogLevel:          "INFO",
		HealthPort:        8888,
		TelemetryInterval: 5 * time.Second,
		EMAAlpha:          0.3,
		DryRun:            false,
		CheckpointPath:    "/root/.agent_checkpoint.uci",
		SkillStorePath:    "/root/skills.db",
		DisableAutoHeal:   false,
	}

	if !flag.Parsed() {
		flag.StringVar(&cfg.ConfigFilePath, "config", cfg.ConfigFilePath, "Path to environment config file")
		flag.StringVar(&cfg.KeyFilePath, "keyfile", cfg.KeyFilePath, "Path to secure API key file")
		flag.BoolVar(&cfg.DryRun, "dry-run", cfg.DryRun, "Enable dry-run mode (no network modifications)")
		flag.Parse()
	}

	if err := parseEnvFile(cfg.ConfigFilePath, cfg); err != nil {
		logger.Warn("Could not parse config file %s: %v. Using defaults.", cfg.ConfigFilePath, err)
	}

	if val := os.Getenv("GEMINI_API_KEY"); val != "" {
		cfg.GeminiAPIKey = strings.TrimSpace(val)
	}
	if val := os.Getenv("AUTH_TOKEN"); val != "" {
		cfg.AuthToken = strings.TrimSpace(val)
	}
	if val := os.Getenv("APPROVE_TOKEN"); val != "" {
		cfg.ApproveToken = strings.TrimSpace(val)
	}
	if val := os.Getenv("LOG_LEVEL"); val != "" {
		cfg.LogLevel = strings.TrimSpace(val)
	}
	if val := os.Getenv("BERYL7_DISABLE_HEALING"); val == "1" || strings.ToLower(val) == "true" {
		cfg.DisableAutoHeal = true
	}

	if cfg.GeminiAPIKey == "" {
		key, err := readSecureKeyFile(cfg.KeyFilePath)
		if err == nil && key != "" {
			cfg.GeminiAPIKey = key
		}
	}

	cfg.apiKeyAtomic.Store(cfg.GeminiAPIKey)

	if cfg.AuthToken == "" {
		logger.Warn("AUTH_TOKEN is empty in config! Health check endpoint will require setting AUTH_TOKEN.")
	}

	if cfg.ApproveToken == "" || cfg.ApproveToken == cfg.AuthToken {
		logger.Warn("SECURITY WARNING: APPROVE_TOKEN is not set or identical to AUTH_TOKEN! /api/approve endpoint will be disabled (Fail-Closed).")
	}

	return cfg, nil
}

func readSecureKeyFile(filePath string) (string, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return "", err
	}

	if info.Mode().Perm()&0077 != 0 {
		logger.Warn("SECURITY WARNING: Key file %s has overly permissive permissions (%o). Expected 0600!", filePath, info.Mode().Perm())
	}

	cleanPath := filepath.Clean(filePath)
	content, err := os.ReadFile(cleanPath) // #nosec G304
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(content)), nil
}

func parseEnvFile(filePath string, cfg *Config) error {
	cleanPath := filepath.Clean(filePath)
	file, err := os.Open(cleanPath) // #nosec G304
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		val := strings.Trim(strings.TrimSpace(parts[1]), `"'`)

		switch key {
		case "GEMINI_API_KEY":
			cfg.GeminiAPIKey = val
		case "AUTH_TOKEN":
			cfg.AuthToken = val
		case "APPROVE_TOKEN":
			cfg.ApproveToken = val
		case "LOG_LEVEL":
			cfg.LogLevel = val
		case "DISABLE_AUTO_HEALING":
			cfg.DisableAutoHeal = (val == "true" || val == "1")
		case "HEALTH_PORT":
			if p, err := strconv.Atoi(val); err == nil {
				cfg.HealthPort = p
			}
		}
	}

	return scanner.Err()
}

func (c *Config) GetAPIKeySnapshot() string {
	if val := c.apiKeyAtomic.Load(); val != nil {
		return val.(string)
	}
	return c.GeminiAPIKey
}

func IsKillSwitchActive(cfg *Config) bool {
	if cfg.DisableAutoHeal {
		return true
	}

	if _, err := os.Stat("/tmp/beryl7-disable"); err == nil {
		return true
	}

	if os.Getenv("BERYL7_DISABLE_HEALING") == "1" {
		return true
	}

	if _, err := os.Stat(filepath.Join("/etc/config", "beryl7_disable")); err == nil {
		return true
	}

	return false
}
