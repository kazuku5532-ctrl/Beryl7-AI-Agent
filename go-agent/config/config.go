package config

import (
	"bufio"
	"flag"
	"fmt"
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
		AuthToken:         "", // Xóa bỏ token mặc định hard-coded
		LogLevel:          "INFO",
		HealthPort:        8888,
		TelemetryInterval: 5 * time.Second,
		EMAAlpha:          0.3,
		DryRun:            false,
		CheckpointPath:    "/root/.agent_checkpoint.uci",
		SkillStorePath:    "/root/skills.db",
		DisableAutoHeal:   false,
	}

	// 1. Đọc cờ dòng lệnh (CLI Flags - Ưu tiên cao nhất)
	flag.StringVar(&cfg.ConfigFilePath, "config", cfg.ConfigFilePath, "Path to environment config file")
	flag.StringVar(&cfg.KeyFilePath, "keyfile", cfg.KeyFilePath, "Path to secure API key file")
	flag.BoolVar(&cfg.DryRun, "dry-run", cfg.DryRun, "Enable dry-run mode (no network modifications)")
	flag.Parse()

	// 2. Đọc file cấu hình môi trường /etc/beryl7/agent.env
	if err := parseEnvFile(cfg.ConfigFilePath, cfg); err != nil {
		logger.Warn("Could not parse config file %s: %v. Using defaults.", cfg.ConfigFilePath, err)
	}

	// 3. Đọc biến môi trường hệ thống OS Environment
	if val := os.Getenv("GEMINI_API_KEY"); val != "" {
		cfg.GeminiAPIKey = strings.TrimSpace(val)
	}
	if val := os.Getenv("AUTH_TOKEN"); val != "" {
		cfg.AuthToken = strings.TrimSpace(val)
	}
	if val := os.Getenv("LOG_LEVEL"); val != "" {
		cfg.LogLevel = strings.TrimSpace(val)
	}
	if val := os.Getenv("BERYL7_DISABLE_HEALING"); val == "1" || strings.ToLower(val) == "true" {
		cfg.DisableAutoHeal = true
	}

	// 4. Đọc file khóa bảo mật an toàn /etc/beryl7/agent.key (chmod 0600)
	if cfg.GeminiAPIKey == "" {
		key, err := readSecureKeyFile(cfg.KeyFilePath)
		if err == nil && key != "" {
			cfg.GeminiAPIKey = key
		}
	}

	cfg.apiKeyAtomic.Store(cfg.GeminiAPIKey)

	// Yêu cầu bắt buộc AUTH_TOKEN hoặc tự động đọc từ /etc/beryl7/agent.env
	if cfg.AuthToken == "" {
		logger.Warn("AUTH_TOKEN is empty in config! Health check endpoint will require setting AUTH_TOKEN.")
	}

	return cfg, nil
}

func readSecureKeyFile(filePath string) (string, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return "", err
	}

	// Kiểm tra quyền file chmod 0600 (Chỉ owner có quyền đọc)
	if info.Mode().Perm()&0077 != 0 {
		logger.Warn("SECURITY WARNING: Key file %s has overly permissive permissions (%o). Expected 0600!", filePath, info.Mode().Perm())
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(content)), nil
}

func parseEnvFile(filePath string, cfg *Config) error {
	file, err := os.Open(filePath)
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

	// 1. Kill Switch File Level
	if _, err := os.Stat("/tmp/beryl7-disable"); err == nil {
		return true
	}

	// 2. Kill Switch Env Level
	if os.Getenv("BERYL7_DISABLE_HEALING") == "1" {
		return true
	}

	// 3. Kill Switch UCI Section Level
	if _, err := os.Stat(filepath.Join("/etc/config", "beryl7_disable")); err == nil {
		return true
	}

	return false
}
