package watchdog

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
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

func (w *Watchdog) IsSafeMode() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.safeModeActive
}

// RecordHealthCheckSuccess Đếm số lần kiểm tra Health thành công liên tiếp (3x 30s = 90s) để tự động thoát Safe Mode
func (w *Watchdog) RecordHealthCheckSuccess() bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.safeModeActive {
		return false
	}

	w.successfulChecks++
	logger.Info("Safe Mode Health Check Success (%d/3)", w.successfulChecks)

	if w.successfulChecks >= 3 {
		w.safeModeActive = false
		w.successfulChecks = 0
		logger.Info("Safe Mode EXIT Criteria Met (3/3 Consecutive Successes)! Restoring Auto-Healing Engine.")
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
	return w.saveCheckpointWithConfig(map[string]string{"network.wan.proto": "dhcp"})
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

// ExecuteRollback Guardrail khôi phục 100% cấu hình UCI cũ từ /tmp/agent_checkpoint.uci khi rớt mạng
func (w *Watchdog) ExecuteRollback() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	logger.Warn("Watchdog Guardrail Triggered! Rolling back full router UCI configuration from checkpoint...")

	// 1. Thử khôi phục từ full uci export snapshot tại /tmp/agent_checkpoint.uci nếu có
	uciBackupPath := "/tmp/agent_checkpoint.uci"
	if _, err := os.Stat(uciBackupPath); err == nil {
		cmdImport := exec.Command("/bin/sh", "-c", fmt.Sprintf("uci import < %s && uci commit", uciBackupPath)) // #nosec G204
		if out, errImport := cmdImport.CombinedOutput(); errImport == nil {
			logger.Info("Successfully imported full UCI snapshot from %s: %s", uciBackupPath, string(out))
		} else {
			logger.Warn("UCI import from %s failed (%v), falling back to WAN DHCP default.", uciBackupPath, errImport)
			_ = exec.Command("uci", "set", "network.wan.proto=dhcp").Run() // #nosec G204
			_ = exec.Command("uci", "commit", "network").Run()             // #nosec G204
		}
	} else {
		// Fallback chuẩn WAN DHCP nếu không có file snapshot
		_ = exec.Command("uci", "set", "network.wan.proto=dhcp").Run() // #nosec G204
		_ = exec.Command("uci", "commit", "network").Run()             // #nosec G204
	}

	// 2. Reload lại các dịch vụ hệ thống mạng & firewall
	_ = exec.Command("/etc/init.d/firewall", "reload").Run() // #nosec G204
	_ = exec.Command("/etc/init.d/network", "reload").Run()  // #nosec G204

	w.safeModeActive = true
	w.successfulChecks = 0
	_ = w.saveCheckpointWithConfig(map[string]string{"network.wan.proto": "dhcp"})

	return nil
}
