package config

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"time"
)

// Config chứa toàn bộ tham số cấu hình hệ thống
type Config struct {
	GeminiAPIKey       string
	AuthToken          string
	SkillStorePath     string
	CheckpointPath     string
	KeyFilePath        string
	HealthPort         int
	TelemetryInterval  time.Duration
	EMAAlpha           float64
	LogLevel           string
	DryRun             bool
	DisableAutoHealing bool
	MaxSkills          int
}

// ConfigSnapshot bọc con trỏ Atomic cho Hot-reload key an toàn
type ConfigSnapshot struct {
	apiKey atomic.Value // string
}

func NewSnapshot(key string) *ConfigSnapshot {
	s := &ConfigSnapshot{}
	s.apiKey.Store(key)
	return s
}

func (s *ConfigSnapshot) GetAPIKey() string {
	if val := s.apiKey.Load(); val != nil {
		return val.(string)
	}
	return ""
}

func (s *ConfigSnapshot) UpdateAPIKey(newKey string) {
	s.apiKey.Store(newKey)
}

// LoadConfig đọc cấu hình theo thứ tự ưu tiên: CLI Flags > File Key / File Env > Mặc định
func LoadConfig() (*Config, error) {
	configPath := flag.String("config", "/etc/beryl7/agent.env", "Path to agent configuration env file")
	keyPath := flag.String("keyfile", "/etc/beryl7/agent.key", "Path to secure API key file")
	dryRun := flag.Bool("dry-run", false, "Run agent in dry-run mode without modifying router network settings")
	debug := flag.Bool("debug", false, "Enable debug log level")
	port := flag.Int("port", 8888, "HTTP Health Check server port")
	flag.Parse()

	cfg := &Config{
		SkillStorePath:    "/root/skills.db",
		CheckpointPath:    "/root/.agent_checkpoint.uci",
		KeyFilePath:       *keyPath,
		HealthPort:        *port,
		TelemetryInterval: 30 * time.Second,
		EMAAlpha:          0.3,
		LogLevel:          "INFO",
		DryRun:            *dryRun,
		MaxSkills:         1000,
		AuthToken:         "beryl7-secret-health-token",
	}

	if *debug {
		cfg.LogLevel = "DEBUG"
	}

	// 1. Đọc API Key từ file an toàn (/etc/beryl7/agent.key - chmod 600)
	if keyBytes, err := os.ReadFile(*keyPath); err == nil {
		cfg.GeminiAPIKey = strings.TrimSpace(string(keyBytes))
	} else {
		// Fallback: đọc từ file env hoặc biến môi trường GEMINI_API_KEY
		if envKey := os.Getenv("GEMINI_API_KEY"); envKey != "" {
			cfg.GeminiAPIKey = envKey
		}
	}

	// 2. Đọc file config env nếu có
	if envBytes, err := os.ReadFile(*configPath); err == nil {
		lines := strings.Split(string(envBytes), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				k := strings.TrimSpace(parts[0])
				v := strings.TrimSpace(parts[1])
				v = strings.Trim(v, `"'`)
				switch k {
				case "GEMINI_API_KEY":
					if cfg.GeminiAPIKey == "" {
						cfg.GeminiAPIKey = v
					}
				case "AUTH_TOKEN":
					cfg.AuthToken = v
				case "LOG_LEVEL":
					if !*debug {
						cfg.LogLevel = v
					}
				case "DISABLE_AUTO_HEALING":
					cfg.DisableAutoHealing = (v == "true" || v == "1")
				}
			}
		}
	}

	return cfg, nil
}

// IsKillSwitchActive kiểm tra cờ tắt khẩn cấp theo đúng thứ tự ưu tiên:
// Ưu tiên 1: Biến môi trường BERYL7_DISABLE_HEALING=1
// Ưu tiên 2: File chạm /tmp/beryl7-disable tồn tại
// Ưu tiên 3: Cấu hình DisableAutoHealing=true
func IsKillSwitchActive(cfg *Config) bool {
	if os.Getenv("BERYL7_DISABLE_HEALING") == "1" || os.Getenv("BERYL7_DISABLE_HEALING") == "true" {
		return true
	}

	if _, err := os.Stat("/tmp/beryl7-disable"); err == nil {
		return true
	}

	if cfg != nil && cfg.DisableAutoHealing {
		return true
	}

	return false
}

// WriteSecureKeyFile tạo file chứa Key với mặt nạ an toàn 0600 (chỉ root đọc/ghi)
func WriteSecureKeyFile(path, key string) error {
	if err := os.MkdirAll("/etc/beryl7", 0700); err != nil {
		return fmt.Errorf("failed to create config dir: %w", err)
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to open secure key file: %w", err)
	}
	defer f.Close()

	if _, err := f.WriteString(strings.TrimSpace(key) + "\n"); err != nil {
		return fmt.Errorf("failed to write key to file: %w", err)
	}

	return f.Sync()
}
