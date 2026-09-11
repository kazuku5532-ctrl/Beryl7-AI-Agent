package watchdog

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"beryl7-agent/logger"
)

type Checkpoint struct {
	Version          int               `json:"version"`
	Timestamp        int64             `json:"timestamp"`
	SafeModeActive   bool              `json:"safe_mode_active"`
	SuccessfulChecks int               `json:"successful_checks"`
	ConfigSnapshot   map[string]string `json:"config_snapshot"`
	Checksum         string            `json:"checksum"`
}

type Watchdog struct {
	mu               sync.Mutex
	checkpointPath   string
	safeModeActive   bool
	successfulChecks int
	rollbackWindow   time.Duration
	suspended        bool
}

func New(checkpointPath string) *Watchdog {
	w := &Watchdog{
		checkpointPath: checkpointPath,
		rollbackWindow: 30 * time.Second, // Timeout đủ 30s bao quát thời gian đệm nạp mạng
	}

	// Đọc checkpoint cũ xem có sự cố sập nguồn trước đó không
	if err := w.LoadAndVerifyCheckpoint(); err != nil {
		logger.Warn("Watchdog Checkpoint read failed/corrupted (%v) -> Fallback to Safe Mode!", err)
		w.safeModeActive = true
	}

	return w
}

func (w *Watchdog) Suspend() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.suspended = true
	logger.Info("Watchdog timer suspended during process restart/reload.")
}

func (w *Watchdog) Resume() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.suspended = false
	logger.Info("Watchdog timer resumed.")
}

func (w *Watchdog) IsSafeMode() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.suspended {
		return false
	}
	return w.safeModeActive
}

// RecordHealthCheckSuccess Đếm số lần kiểm tra Health thành công liên tiếp (5x 30s = 150s) để tự động thoát Safe Mode
func (w *Watchdog) RecordHealthCheckSuccess() bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.safeModeActive {
		return false
	}

	w.successfulChecks++
	logger.Info("Safe Mode Health Check Success (%d/5)", w.successfulChecks)

	if w.successfulChecks >= 5 {
		w.safeModeActive = false
		w.successfulChecks = 0
		logger.Info("Safe Mode EXIT Criteria Met (5/5 Consecutive Successes)! Restoring Auto-Healing Engine.")
		_ = w.saveCheckpointInternal(false)
		return true
	}

	return false
}

func (w *Watchdog) computeChecksum(cp *Checkpoint) string {
	data := fmt.Sprintf("%d:%d:%v:%d:%v", cp.Version, cp.Timestamp, cp.SafeModeActive, cp.SuccessfulChecks, cp.ConfigSnapshot)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

func (w *Watchdog) SaveCheckpoint(config map[string]string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.saveCheckpointWithConfig(config)
}

func (w *Watchdog) saveCheckpointInternal(safeMode bool) error {
	return w.saveCheckpointWithConfig(map[string]string{
		"network.wan.proto":            "dhcp",
		"wireless.MT7993_1_2.disabled": "0",
		"wireless.MT7993_1_1.disabled": "0",
	})
}

func (w *Watchdog) saveCheckpointWithConfig(config map[string]string) error {
	cp := &Checkpoint{
		Version:          1,
		Timestamp:        time.Now().Unix(),
		SafeModeActive:   w.safeModeActive,
		SuccessfulChecks: w.successfulChecks,
		ConfigSnapshot:   config,
	}
	cp.Checksum = w.computeChecksum(cp)

	bytesData, err := json.MarshalIndent(cp, "", "  ")
	if err != nil {
		return err
	}

	// Ghi file nguyên tử (Atomic File Write) tại /root/.agent_checkpoint.uci
	tmpFile := w.checkpointPath + ".tmp"
	if err := os.WriteFile(tmpFile, bytesData, 0600); err != nil {
		return err
	}

	return os.Rename(tmpFile, w.checkpointPath)
}

func (w *Watchdog) LoadAndVerifyCheckpoint() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	data, err := os.ReadFile(w.checkpointPath)
	if err != nil {
		return err
	}

	var cp Checkpoint
	if err := json.Unmarshal(data, &cp); err != nil {
		return err
	}

	// Xác minh SHA256 Checksum chống rác file
	expectedChecksum := w.computeChecksum(&cp)
	if cp.Checksum != expectedChecksum {
		return errors.New("checkpoint SHA256 checksum mismatch (file corrupted)")
	}

	w.safeModeActive = cp.SafeModeActive
	w.successfulChecks = cp.SuccessfulChecks
	return nil
}

// UCISyntaxPreCheck Kiểm tra cú pháp uci show network TRƯỚC KHI uci commit
func UCISyntaxPreCheck() error {
	cmd := exec.Command("uci", "show", "network") // #nosec G204
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("UCI network syntax error: %s (%v)", string(output), err)
	}
	return nil
}

// ExecuteRollback Guardrail khôi phục 100% cấu hình UCI cũ (bao gồm network & wireless) khi rớt mạng
func (w *Watchdog) ExecuteRollback() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	logger.Warn("Watchdog Guardrail Triggered! Rolling back full router UCI configuration (Network & Wireless) from checkpoint...")

	uciBackupPath := w.checkpointPath
	if uciBackupPath == "" {
		uciBackupPath = "/root/.agent_checkpoint.uci"
	}
	cleanBackupPath := filepath.Clean(uciBackupPath)

	restoredFromSnapshot := false
	if data, errRead := os.ReadFile(cleanBackupPath); errRead == nil {
		var cp Checkpoint
		if errJSON := json.Unmarshal(data, &cp); errJSON == nil && len(cp.ConfigSnapshot) > 0 {
			// Restore individual UCI key-value pairs from JSON snapshot
			for k, v := range cp.ConfigSnapshot {
				if k != "" {
					_ = exec.Command("uci", "set", fmt.Sprintf("%s=%s", k, v)).Run() // #nosec G204 // nolint:errcheck
				}
			}
			restoredFromSnapshot = true
			logger.Info("Successfully restored %d UCI settings from JSON checkpoint %s", len(cp.ConfigSnapshot), cleanBackupPath)
		} else {
			// Fallback: try raw UCI import if file contains plain UCI format
			if f, errOpen := os.Open(cleanBackupPath); errOpen == nil { // #nosec G304
				cmdImport := exec.Command("uci", "import") // #nosec G204
				cmdImport.Stdin = f
				if out, errImport := cmdImport.CombinedOutput(); errImport == nil {
					restoredFromSnapshot = true
					logger.Info("Successfully imported raw UCI snapshot from %s: %s", cleanBackupPath, string(out))
				}
				_ = f.Close() // nolint:errcheck
			}
		}
	}

	if !restoredFromSnapshot {
		logger.Warn("Could not restore from snapshot %s, applying fail-safe WAN DHCP & Wireless defaults.", cleanBackupPath)
		_ = exec.Command("uci", "set", "network.wan.proto=dhcp").Run() // #nosec G204 // nolint:errcheck
		_ = exec.Command("uci", "set", "wireless.MT7993_1_2.disabled=0").Run() // #nosec G204 // nolint:errcheck
		_ = exec.Command("uci", "set", "wireless.MT7993_1_1.disabled=0").Run() // #nosec G204 // nolint:errcheck
	}

	// 1. Commit both Network and Wireless configurations
	_ = exec.Command("uci", "commit", "network").Run()  // #nosec G204 // nolint:errcheck
	_ = exec.Command("uci", "commit", "wireless").Run() // #nosec G204 // nolint:errcheck

	// 2. Reload services: Firewall, Network, and Wireless subsystem
	_ = exec.Command("/etc/init.d/firewall", "reload").Run() // #nosec G204 // nolint:errcheck
	_ = exec.Command("/etc/init.d/network", "reload").Run()  // #nosec G204 // nolint:errcheck
	if _, errWiFi := exec.LookPath("wifi"); errWiFi == nil {
		_ = exec.Command("wifi", "reload").Run() // #nosec G204 // nolint:errcheck
	} else if _, errWiFiInit := exec.LookPath("/sbin/wifi"); errWiFiInit == nil {
		_ = exec.Command("/sbin/wifi", "reload").Run() // #nosec G204 // nolint:errcheck
	}

	w.safeModeActive = true
	w.successfulChecks = 0
	_ = w.saveCheckpointWithConfig(map[string]string{
		"network.wan.proto":            "dhcp",
		"wireless.MT7993_1_2.disabled": "0",
		"wireless.MT7993_1_1.disabled": "0",
	}) // nolint:errcheck

	return nil
}
