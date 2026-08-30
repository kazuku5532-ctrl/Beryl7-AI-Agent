package config

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
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
	TrustReverseProxy   bool
	DisableLocalhostBypass bool
	AirgappedMode       bool
	TelegramBotToken    string
	TelegramChatID      string
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
		BindHost:           "0.0.0.0",
		CORSAllowedOrigins: "http://192.168.8.1:8888,http://127.0.0.1:8888,http://localhost:8888",
		TelemetryInterval:  5 * time.Second,
		EMAAlpha:           0.3,
		DryRun:             false,
		CheckpointPath:     "/root/.agent_checkpoint.uci",
		SkillStorePath:     "/root/skills.db",
		DisableAutoHeal:    false,
		FirmwareVersion:    "4.9.0",
		RAMExhaustionPct:       95.0,
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
	if val := os.Getenv("BERYL7_TRUST_REVERSE_PROXY"); val == "1" || strings.ToLower(val) == "true" {
		cfg.TrustReverseProxy = true
	}
	if val := os.Getenv("BERYL7_DISABLE_LOCALHOST_BYPASS"); val == "1" || strings.ToLower(val) == "true" {
		cfg.DisableLocalhostBypass = true
	}
	if val := os.Getenv("BERYL7_SKILLSTORE_PATH"); val != "" {
		cfg.SkillStorePath = strings.TrimSpace(val)
	}
	if val := os.Getenv("BERYL7_CHECKPOINT_PATH"); val != "" {
		cfg.CheckpointPath = strings.TrimSpace(val)
	}
	if val := os.Getenv("BERYL7_AIRGAPPED_MODE"); val == "1" || strings.ToLower(val) == "true" {
		cfg.AirgappedMode = true
		logger.Info("AIR-GAPPED MODE ENABLED: Outbound Cloud AI traffic disabled.")
	}

	if val := os.Getenv("TELEGRAM_BOT_TOKEN"); val != "" {
		cfg.TelegramBotToken = strings.TrimSpace(val)
	}
	if val := os.Getenv("TELEGRAM_CHAT_ID"); val != "" {
		cfg.TelegramChatID = strings.TrimSpace(val)
	}

	if cfg.GeminiAPIKey == "" {
		key, err := readSecureKeyFile(cfg.KeyFilePath)
		if err == nil && key != "" {
			if strings.Contains(key, "GEMINI_API_KEY=") {
				for _, line := range strings.Split(key, "\n") {
					line = strings.TrimSpace(line)
					if strings.HasPrefix(line, "GEMINI_API_KEY=") {
						cfg.GeminiAPIKey = strings.TrimPrefix(line, "GEMINI_API_KEY=")
					} else if strings.HasPrefix(line, "TELEGRAM_BOT_TOKEN=") && cfg.TelegramBotToken == "" {
						cfg.TelegramBotToken = strings.TrimPrefix(line, "TELEGRAM_BOT_TOKEN=")
					}
				}
			} else {
				cfg.GeminiAPIKey = key
			}
			logger.Info("Loaded secure keys from keyfile [%s]", cfg.KeyFilePath)
		} else if fallbackKey, errFallback := readSecureKeyFile("/etc/beryl7/agent.key"); errFallback == nil && fallbackKey != "" {
			if strings.Contains(fallbackKey, "GEMINI_API_KEY=") {
				for _, line := range strings.Split(fallbackKey, "\n") {
					line = strings.TrimSpace(line)
					if strings.HasPrefix(line, "GEMINI_API_KEY=") {
						cfg.GeminiAPIKey = strings.TrimPrefix(line, "GEMINI_API_KEY=")
					} else if strings.HasPrefix(line, "TELEGRAM_BOT_TOKEN=") && cfg.TelegramBotToken == "" {
						cfg.TelegramBotToken = strings.TrimPrefix(line, "TELEGRAM_BOT_TOKEN=")
					}
				}
			} else {
				cfg.GeminiAPIKey = fallbackKey
			}
			logger.Info("Loaded fallback keys from [/etc/beryl7/agent.key]")
		}
	}

	if cfg.TelegramBotToken == "" {
		// Read keyfile specifically for TELEGRAM_BOT_TOKEN if keyfile has TELEGRAM_BOT_TOKEN=...
		for _, kPath := range []string{cfg.KeyFilePath, "/etc/beryl7/agent.key"} {
			if content, err := readSecureKeyFile(kPath); err == nil {
				for _, line := range strings.Split(content, "\n") {
					line = strings.TrimSpace(line)
					if strings.HasPrefix(line, "TELEGRAM_BOT_TOKEN=") {
						cfg.TelegramBotToken = strings.TrimSpace(strings.TrimPrefix(line, "TELEGRAM_BOT_TOKEN="))
						break
					}
				}
			}
			if cfg.TelegramBotToken != "" {
				break
			}
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
		"/etc/beryl7/agent.key",
		"/etc/beryl7/skills.db", // [Fix 3] Preserve Agent's self-learned remediation skills across firmware upgrades
		"/usr/bin/beryl7-agent",
		"/etc/init.d/beryl7-agent",
		"/root/.agent_checkpoint.uci",
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
		f, err := os.OpenFile(cleanSysupgrade, os.O_APPEND|os.O_WRONLY, 0644) // #nosec G302, G304
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
				if err := os.Chmod(cleanPath, mode); err == nil { // #nosec G302, G306
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
    procd_set_param file /etc/beryl7/agent.env
    procd_set_param respawn 3600 5 0
    procd_set_param env GOMEMLIMIT=15MiB GOGC=20
    procd_set_param stdout 1
    procd_set_param stderr 1
    procd_set_param oom_score_adj -500
    procd_close_instance
}

stop_service() {
    procd_send_signal "$PROG" 15
}
`
		// #nosec G306
		if err := os.WriteFile(cleanPath, []byte(content), 0755); err == nil { // #nosec G306
			logger.Info("Auto-generated procd init service script at /etc/init.d/beryl7-agent")
		}
	} else {
		_ = os.Chmod(cleanPath, 0755) // #nosec G302
	}

	// Triple-Lock Protection Level 1: Ensure boot symlink in /etc/rc.d exists
	rcSymlink := "/etc/rc.d/S99beryl7-agent"
	if _, err := os.Stat(rcSymlink); os.IsNotExist(err) {
		if _, lookErr := exec.LookPath("/etc/init.d/beryl7-agent"); lookErr == nil {
			cmd := exec.Command("/etc/init.d/beryl7-agent", "enable")
			if out, cmdErr := cmd.CombinedOutput(); cmdErr == nil {
				logger.Info("AUTO-START LOCK: Successfully enabled beryl7-agent init service via /etc/init.d/beryl7-agent enable")
			} else {
				logger.Warn("Failed to run /etc/init.d/beryl7-agent enable: %v (%s). Attempting manual symlink...", cmdErr, string(out))
				_ = os.Symlink(cleanPath, rcSymlink)
			}
		} else if _, statInit := os.Stat(cleanPath); statInit == nil {
			_ = os.Symlink(cleanPath, rcSymlink)
		}
	}

	// Triple-Lock Protection Level 3: Fail-safe Crontab Watchdog for crash/dirty power loss recovery
	crontabPath := filepath.Clean("/etc/crontabs/root")
	if cronData, err := os.ReadFile(crontabPath); err == nil { // #nosec G304
		cronContent := string(cronData)
		if !strings.Contains(cronContent, "beryl7-agent") {
			watchdogLine := "* * * * * pgrep beryl7-agent >/dev/null || /etc/init.d/beryl7-agent start\n"
			newContent := cronContent
			if !strings.HasSuffix(newContent, "\n") && len(newContent) > 0 {
				newContent += "\n"
			}
			newContent += watchdogLine
			if writeErr := os.WriteFile(crontabPath, []byte(newContent), 0600); writeErr == nil { // #nosec G306, G703
				logger.Info("AUTO-START LOCK: Successfully registered Fail-safe Cron Watchdog in /etc/crontabs/root")
				_ = exec.Command("/etc/init.d/cron", "enable").Run()
				_ = exec.Command("/etc/init.d/cron", "start").Run()
			}
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
		case "TELEGRAM_BOT_TOKEN", "BERYL7_TELEGRAM_BOT_TOKEN":
			cfg.TelegramBotToken = val
		case "TELEGRAM_CHAT_ID", "BERYL7_TELEGRAM_CHAT_ID":
			cfg.TelegramChatID = val
		case "AUTH_TOKEN":
			cfg.AuthToken = val
		case "APPROVE_TOKEN":
			cfg.ApproveToken = val
		case "LOG_LEVEL":
			cfg.LogLevel = val
		case "DISABLE_AUTO_HEALING":
			cfg.DisableAutoHeal = (val == "true" || val == "1")
		case "DRY_RUN", "BERYL7_DRY_RUN":
			cfg.DryRun = (val == "true" || val == "1")
		case "BERYL7_AIRGAPPED_MODE", "AIRGAPPED_MODE":
			cfg.AirgappedMode = (val == "true" || val == "1")
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
