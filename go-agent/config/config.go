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
	ApproveToken      string
	LogLevel          string
	HealthPort        int
	TelemetryInterval time.Duration
	EMAAlpha          float64
	DryRun            bool
	CheckpointPath    string
	SkillStorePath    string
	DisableAutoHeal   bool
	FirmwareVersion   string
	BindHost          string
	CORSAllowedOrigins string
	RAMExhaustionPct   float64
	CPUSpikeLoad       float64
	LatencySpikeMs     float64
	LatencyZScoreThreshold float64
	BandwidthBoostMbps float64
	BandwidthRestoreMbps float64
	WiFiDisconnectCount int
	LogMaxBytes         int64
	LogBackupCount      int
	apiKeyAtomic      atomic.Value
}

type FirmwareCapability struct {
	Version         string
	MinGoVersion    string
	UbusAPIVersion  int
	SkillCompatible map[string]bool
}

var CapabilityMatrix = map[string]FirmwareCapability{
	"4.9.0": {
		Version:        "4.9.0",
		MinGoVersion:   "1.21",
		UbusAPIVersion: 1,
		SkillCompatible: map[string]bool{
			"purge_memory_cache":    true,
			"restart_wan_interface": true,
			"optimize_wifi_channel": true,
			"boost_wifi_bandwidth":  true,
		},
	},
	"5.0": {
		Version:        "5.0",
		MinGoVersion:   "1.21",
		UbusAPIVersion: 2,
		SkillCompatible: map[string]bool{
			"purge_memory_cache":    true,
			"restart_wan_interface": true,
			"optimize_wifi_channel": true,
			"boost_wifi_bandwidth":  true,
			"qos_v2_boost":          true,
		},
	},
}

func LoadConfig() (*Config, error) {
	cfg := &Config{
		ConfigFilePath:     "/etc/beryl7/agent.env",
		KeyFilePath:        "/etc/beryl7/agent.key",
		AuthToken:          "",
		ApproveToken:       "",
		LogLevel:           "INFO",
		HealthPort:         8888,
		BindHost:           "127.0.0.1",
		CORSAllowedOrigins: "http://192.168.8.1:8888,http://127.0.0.1:8888,http://localhost:8888,null",
		TelemetryInterval:  5 * time.Second,
		EMAAlpha:           0.3,
		DryRun:             false,
		CheckpointPath:     "/root/.agent_checkpoint.uci",
		SkillStorePath:     "/root/skills.db",
		DisableAutoHeal:    false,
		FirmwareVersion:    "4.9.0",
		RAMExhaustionPct:       92.0,
		CPUSpikeLoad:           1.5,
		LatencySpikeMs:         100.0,
		LatencyZScoreThreshold: 2.5,
		BandwidthBoostMbps:     80.0,
		BandwidthRestoreMbps:   20.0,
		WiFiDisconnectCount:    3,
		LogMaxBytes:            2 * 1024 * 1024,
		LogBackupCount:         5,
	}

	var showVersion bool
	if !flag.Parsed() {
		flag.StringVar(&cfg.ConfigFilePath, "config", cfg.ConfigFilePath, "Path to environment config file")
		flag.StringVar(&cfg.KeyFilePath, "keyfile", cfg.KeyFilePath, "Path to secure API key file")
		flag.BoolVar(&cfg.DryRun, "dry-run", cfg.DryRun, "Enable dry-run mode (no network modifications)")
		flag.BoolVar(&showVersion, "version", false, "Print daemon version and exit")
		flag.Parse()
	}

	if showVersion {
		fmt.Println("beryl7-agent version v16.0")
		os.Exit(0)
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

	if val := os.Getenv("HEALTH_PORT"); val != "" {
		if p, err := strconv.Atoi(val); err == nil {
			cfg.HealthPort = p
		} else {
			logger.Warn("Invalid numeric value for HEALTH_PORT: '%s'. Keeping default %d", val, cfg.HealthPort)
		}
	}
	if val := os.Getenv("BERYL7_RAM_EXHAUSTION_PCT"); val != "" {
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			cfg.RAMExhaustionPct = f
		} else {
			logger.Warn("Invalid float value for BERYL7_RAM_EXHAUSTION_PCT: '%s'. Keeping default %.1f", val, cfg.RAMExhaustionPct)
		}
	}
	if val := os.Getenv("BERYL7_CPU_SPIKE_LOAD"); val != "" {
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			cfg.CPUSpikeLoad = f
		} else {
			logger.Warn("Invalid float value for BERYL7_CPU_SPIKE_LOAD: '%s'. Keeping default %.1f", val, cfg.CPUSpikeLoad)
		}
	}
	if val := os.Getenv("BERYL7_LATENCY_SPIKE_MS"); val != "" {
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			cfg.LatencySpikeMs = f
		} else {
			logger.Warn("Invalid float value for BERYL7_LATENCY_SPIKE_MS: '%s'. Keeping default %.1f", val, cfg.LatencySpikeMs)
		}
	}
	if val := os.Getenv("BERYL7_LATENCY_ZSCORE"); val != "" {
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			cfg.LatencyZScoreThreshold = f
		} else {
			logger.Warn("Invalid float value for BERYL7_LATENCY_ZSCORE: '%s'. Keeping default %.1f", val, cfg.LatencyZScoreThreshold)
		}
	}
	if val := os.Getenv("BERYL7_BANDWIDTH_BOOST_MBPS"); val != "" {
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			cfg.BandwidthBoostMbps = f
		} else {
			logger.Warn("Invalid float value for BERYL7_BANDWIDTH_BOOST_MBPS: '%s'. Keeping default %.1f", val, cfg.BandwidthBoostMbps)
		}
	}
	if val := os.Getenv("BERYL7_BANDWIDTH_RESTORE_MBPS"); val != "" {
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			cfg.BandwidthRestoreMbps = f
		} else {
			logger.Warn("Invalid float value for BERYL7_BANDWIDTH_RESTORE_MBPS: '%s'. Keeping default %.1f", val, cfg.BandwidthRestoreMbps)
		}
	}
	if val := os.Getenv("BERYL7_WIFI_DISCONNECT_COUNT"); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			cfg.WiFiDisconnectCount = i
		} else {
			logger.Warn("Invalid integer value for BERYL7_WIFI_DISCONNECT_COUNT: '%s'. Keeping default %d", val, cfg.WiFiDisconnectCount)
		}
	}
	if val := os.Getenv("BERYL7_LOG_MAX_BYTES"); val != "" {
		if mb, err := strconv.ParseInt(val, 10, 64); err == nil && mb > 0 {
			cfg.LogMaxBytes = mb
		} else {
			logger.Warn("Invalid integer value for BERYL7_LOG_MAX_BYTES: '%s'. Keeping default %d", val, cfg.LogMaxBytes)
		}
	}
	if val := os.Getenv("BERYL7_LOG_BACKUP_COUNT"); val != "" {
		if count, err := strconv.Atoi(val); err == nil && count > 0 {
			cfg.LogBackupCount = count
		} else {
			logger.Warn("Invalid integer value for BERYL7_LOG_BACKUP_COUNT: '%s'. Keeping default %d", val, cfg.LogBackupCount)
		}
	}
	if val := os.Getenv("BERYL7_SKILLSTORE_PATH"); val != "" {
		cfg.SkillStorePath = strings.TrimSpace(val)
	}
	if val := os.Getenv("BERYL7_CHECKPOINT_PATH"); val != "" {
		cfg.CheckpointPath = strings.TrimSpace(val)
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
		logger.Info("Single-Token Mode Active: APPROVE_TOKEN is empty; AUTH_TOKEN will be used for both Admin and Operator tasks.")
	}

	return cfg, nil
}

func EnsureSysupgradePreservation() error {
	sysupgradeConf := "/etc/sysupgrade.conf"
	if _, err := os.Stat(sysupgradeConf); err != nil {
		return nil
	}

	requiredEntries := []string{
		"/etc/beryl7/agent.env",
		"/usr/bin/beryl7-agent",
		"/etc/init.d/beryl7-agent",
		"/root/.agent_checkpoint.uci",
		"/root/skills.db",
	}

	data, err := os.ReadFile(sysupgradeConf)
	if err != nil {
		return err
	}
	content := string(data)

	var missing []string
	for _, entry := range requiredEntries {
		if !strings.Contains(content, entry) {
			missing = append(missing, entry)
		}
	}

	if len(missing) > 0 {
		cleanSysupgrade := filepath.Clean(sysupgradeConf)
		f, err := os.OpenFile(cleanSysupgrade, os.O_APPEND|os.O_WRONLY, 0644) // #nosec G302 G304
		if err != nil {
			return err
		}
		defer f.Close()

		for _, entry := range missing {
			if _, err := f.WriteString(entry + "\n"); err != nil {
				return err
			}
		}
		logger.Info("Registered %d missing entries in /etc/sysupgrade.conf for firmware preservation.", len(missing))
	}
	return nil
}

func EnsureFilePermissions() error {
	files := map[string]os.FileMode{
		"/etc/beryl7/agent.env":    0600,
		"/usr/bin/beryl7-agent":    0755,
		"/etc/init.d/beryl7-agent": 0755,
		"/root/skills.db":          0600,
	}

	for path, mode := range files {
		cleanPath := filepath.Clean(path)
		if info, err := os.Stat(cleanPath); err == nil {
			if info.Mode().Perm() != mode {
				if err := os.Chmod(cleanPath, mode); err == nil { // #nosec G302 G306
					logger.Info("Restored permissions for %s to %04o", path, mode)
				}
			}
		}
	}
	return nil
}

func EnsureProcdInitService() error {
	initScript := "/etc/init.d/beryl7-agent"
	cleanPath := filepath.Clean(initScript)
	if _, err := os.Stat(cleanPath); os.IsNotExist(err) {
		content := `#!/bin/sh /etc/rc.common

START=99
STOP=15
USE_PROCD=1
PROG=/usr/bin/beryl7-agent

start_service() {
    procd_open_instance
    procd_set_param command "$PROG" -config /etc/beryl7/agent.env
    procd_set_param respawn 3600 5 0
    procd_set_param stdout 1
    procd_set_param stderr 1
    procd_close_instance
}
`
		if err := os.WriteFile(cleanPath, []byte(content), 0755); err == nil { // #nosec G306 G302
			logger.Info("Auto-generated procd init service script at /etc/init.d/beryl7-agent")
		}
	}
	return nil
}

func DetectSystemCapability(cfg *Config) string {
	if data, err := os.ReadFile("/etc/glversion"); err == nil && len(data) > 0 {
		version := strings.TrimSpace(string(data))
		cfg.FirmwareVersion = version
		logger.Info("Detected GL.iNet Firmware Version: %s", version)
		return version
	}

	if data, err := os.ReadFile("/etc/openwrt_release"); err == nil && len(data) > 0 {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "DISTRIB_RELEASE=") {
				ver := strings.Trim(strings.TrimPrefix(line, "DISTRIB_RELEASE="), `"'`)
				cfg.FirmwareVersion = ver
				logger.Info("Detected OpenWrt Release Version: %s", ver)
				return ver
			}
		}
	}
	return cfg.FirmwareVersion
}

func DryRunUpgradeCheck(targetVersion string) []string {
	warnings := []string{}
	cap, exists := CapabilityMatrix[targetVersion]
	if !exists {
		warnings = append(warnings, fmt.Sprintf("Target firmware version %s not listed in CapabilityMatrix", targetVersion))
		return warnings
	}

	if cap.UbusAPIVersion > 1 {
		warnings = append(warnings, fmt.Sprintf("Target version %s uses ubus API v%d (requires updated ubus RPC handler)", targetVersion, cap.UbusAPIVersion))
	}
	return warnings
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
		case "BIND_HOST":
			cfg.BindHost = val
		case "CORS_ORIGINS":
			cfg.CORSAllowedOrigins = val
		case "BERYL7_RAM_EXHAUSTION_PCT":
			if f, err := strconv.ParseFloat(val, 64); err == nil {
				cfg.RAMExhaustionPct = f
			}
		case "BERYL7_CPU_SPIKE_LOAD":
			if f, err := strconv.ParseFloat(val, 64); err == nil {
				cfg.CPUSpikeLoad = f
			}
		case "BERYL7_LATENCY_SPIKE_MS":
			if f, err := strconv.ParseFloat(val, 64); err == nil {
				cfg.LatencySpikeMs = f
			}
		case "BERYL7_LATENCY_ZSCORE":
			if f, err := strconv.ParseFloat(val, 64); err == nil {
				cfg.LatencyZScoreThreshold = f
			}
		case "BERYL7_BANDWIDTH_BOOST_MBPS":
			if f, err := strconv.ParseFloat(val, 64); err == nil {
				cfg.BandwidthBoostMbps = f
			}
		case "BERYL7_BANDWIDTH_RESTORE_MBPS":
			if f, err := strconv.ParseFloat(val, 64); err == nil {
				cfg.BandwidthRestoreMbps = f
			}
		case "BERYL7_WIFI_DISCONNECT_COUNT":
			if i, err := strconv.Atoi(val); err == nil {
				cfg.WiFiDisconnectCount = i
			}
		case "BERYL7_LOG_MAX_BYTES":
			if mb, err := strconv.ParseInt(val, 10, 64); err == nil && mb > 0 {
				cfg.LogMaxBytes = mb
			}
		case "BERYL7_LOG_BACKUP_COUNT":
			if count, err := strconv.Atoi(val); err == nil && count > 0 {
				cfg.LogBackupCount = count
			}
		case "BERYL7_SKILLSTORE_PATH":
			cfg.SkillStorePath = val
		case "BERYL7_CHECKPOINT_PATH":
			cfg.CheckpointPath = val
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
