package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"beryl7-agent/ai"
	"beryl7-agent/config"
	"beryl7-agent/executor"
	"beryl7-agent/logger"
	"beryl7-agent/parser"
	"beryl7-agent/skillstore"
	"beryl7-agent/telemetry"
	"beryl7-agent/watchdog"
)

type HealthState struct {
	mu             sync.RWMutex
	Status         string    `json:"status"`
	UptimeSeconds  int64     `json:"uptime_seconds"`
	LastAction     string    `json:"last_action"`
	LastActionTime string    `json:"last_action_time"`
	WANStatus      string    `json:"wan_status"`
	CPUUsagePct    float64   `json:"cpu_usage_pct"`
	RAMUsagePct    float64   `json:"ram_usage_pct"`
	HardwareTempC  float64   `json:"hardware_temp_c"`
	LatencyMs      float64   `json:"latency_ms"`
	SafeMode       bool      `json:"safe_mode"`
	KillSwitch     bool      `json:"kill_switch"`
	StartTime      time.Time `json:"start_time"`
}

type PendingApproval struct {
	Action      string    `json:"action"`
	Reasoning   string    `json:"reasoning"`
	Confidence  float64   `json:"confidence"`
	Required    float64   `json:"required_threshold"`
	Timestamp   time.Time `json:"timestamp"`
}

func main() {
	setOOMScore()

	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	_, err = logger.Init("/var/log/beryl7_agent.log", cfg.LogLevel)
	if err != nil {
		fmt.Printf("Failed to init logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Flush()

	logger.Info("Starting Beryl 7 AI Agent v14.1 Security Hardened Edition (Native Go)...")

	pidPath := "/var/run/beryl7-agent.pid"
	if err := acquirePIDLock(pidPath); err != nil {
		logger.Fatal("PID Lock Error: %v", err)
	}
	defer os.Remove(pidPath)

	store, err := skillstore.New(cfg.SkillStorePath)
	if err != nil {
		logger.Fatal("SkillStore Init Error: %v", err)
	}
	defer store.Close()

	wd := watchdog.New(cfg.CheckpointPath)
	collector := telemetry.NewCollector()
	logParser := parser.NewParser()
	execEngine := executor.New()
	aiClient := ai.NewClient(cfg.GeminiAPIKey)

	ai.ProbeDNSAsync()

	health := &HealthState{
		Status:        "healthy",
		LastAction:    "none",
		WANStatus:     "Active (1/1)",
		StartTime:     time.Now(),
		UptimeSeconds: 0,
	}

	httpServer := startHealthCheckServer(cfg, health, execEngine)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		sig := <-sigCh
		logger.Info("Received shutdown signal (%v)! Initiating Adaptive Graceful Shutdown...", sig)

		ctxHTTP, cancelHTTP := context.WithTimeout(context.Background(), 2*time.Second)
		_ = httpServer.Shutdown(ctxHTTP)
		cancelHTTP()

		_ = store.Close()
		logger.Flush()

		cancel()
		os.Exit(0)
	}()

	logger.Info("Daemon initialized successfully. Security Hardened Engine listening on 24/7 main loop...")

	ticker := time.NewTicker(cfg.TelemetryInterval)
	defer ticker.Stop()

	backupTicker := time.NewTicker(6 * time.Hour)
	defer backupTicker.Stop()

	cooldowns := map[string]time.Duration{
		"WAN_DROP":          90 * time.Second,
		"MEMORY_EXHAUSTION": 45 * time.Second,
		"WIFI_FAILURE":      60 * time.Second,
	}
	lastActionByAnomaly := make(map[string]time.Time)
	isWifiBoosted := false
	lowTrafficCycles := 0

	for {
		select {
		case <-ctx.Done():
			return
		case <-backupTicker.C:
			_ = store.BackupDatabase()
		case <-ticker.C:
			_ = store.PruneSkillsPeriodic()

			m := collector.CollectMetrics(ctx)
			if m == nil {
				continue
			}

			health.mu.Lock()
			health.CPUUsagePct = m.CPUUsagePct
			health.RAMUsagePct = m.RAMUsagePct
			health.HardwareTempC = m.HardwareTempC
			health.LatencyMs = m.LatencyMs
			health.WANStatus = m.WANStatus
			health.UptimeSeconds = int64(time.Since(health.StartTime).Seconds())
			health.SafeMode = wd.IsSafeMode()
			health.KillSwitch = config.IsKillSwitchActive(cfg)
			health.mu.Unlock()

			if wd.IsSafeMode() {
				wd.RecordHealthCheckSuccess()
				continue
			}

			if config.IsKillSwitchActive(cfg) {
				logger.Warn("Kill Switch Active (/tmp/beryl7-disable or env)! Auto-healing suspended.")
				continue
			}

			liveLogSample := logParser.SanitizeLog(getSystemLogSample())

			// Dynamic Adaptive Wi-Fi Boost: Tự động đẩy Max 160MHz khi nạp Buffer / Download nặng, tự hạ 80MHz Eco Mode khi xong
			if m.DownloadMbps > 80.0 && !isWifiBoosted {
				logger.Info("SMART BANDWIDTH DETECTED (%.1f Mbps > 80Mbps)! Auto-boosting Wi-Fi 7 to 160MHz Max Speed...", m.DownloadMbps)
				boostReq := &executor.ActionRequest{ActionName: "boost_wifi_bandwidth", Target: "radio1"}
				if execErr := execEngine.ExecuteAction(ctx, boostReq, cfg.DryRun); execErr == nil {
					isWifiBoosted = true
					lowTrafficCycles = 0
				}
			} else if isWifiBoosted && m.DownloadMbps < 20.0 {
				lowTrafficCycles++
				if lowTrafficCycles >= 2 {
					logger.Info("SMART BANDWIDTH STABILIZED (%.1f Mbps < 20Mbps for 2 cycles)! Reverting Wi-Fi 7 to Eco 80MHz Mode...", m.DownloadMbps)
					revertReq := &executor.ActionRequest{ActionName: "revert_wifi_bandwidth", Target: "radio1"}
					if execErr := execEngine.ExecuteAction(ctx, revertReq, cfg.DryRun); execErr == nil {
						isWifiBoosted = false
						lowTrafficCycles = 0
					}
				}
			} else if isWifiBoosted && m.DownloadMbps >= 20.0 {
				lowTrafficCycles = 0
			}

			// Ưu tiên Anomaly: WAN_DROP (Telemetry) > Log Anomaly (Parser) > MEMORY_EXHAUSTION
			var anomalyType, anomalyDesc string
			if m.WANStatus == "Offline (0/1)" || strings.Contains(m.WANStatus, "Offline") {
				anomalyType = "WAN_DROP"
				anomalyDesc = "WAN interface down or physical link lost"
			} else {
				// Quét log hệ thống qua LogParser tìm WIFI_FAILURE / DEAUTH_FLOOD
				for _, line := range strings.Split(liveLogSample, "\n") {
					if parsedReport := logParser.ParseLine(line); parsedReport != nil {
						anomalyType = parsedReport.Type
						anomalyDesc = parsedReport.Description
						break
					}
				}
				if anomalyType == "" && m.RAMUsagePct > 92.0 {
					anomalyType = "MEMORY_EXHAUSTION"
					anomalyDesc = fmt.Sprintf("High RAM usage detected: %.1f%%", m.RAMUsagePct)
				}
			}

			if anomalyType != "" {
				coolWindow := cooldowns[anomalyType]
				if coolWindow == 0 {
					coolWindow = 60 * time.Second
				}
				if time.Since(lastActionByAnomaly[anomalyType]) < coolWindow {
					continue
				}

				logger.Warn("ANOMALY DETECTED: %s (%s)! Processing Auto-Healing...", anomalyType, anomalyDesc)
				lastActionByAnomaly[anomalyType] = time.Now()

				actionName := "restart_wan_interface"
				if anomalyType == "MEMORY_EXHAUSTION" {
					actionName = "purge_memory_cache"
				} else if anomalyType == "WIFI_FAILURE" {
					actionName = "optimize_wifi_channel"
				}

				skill := store.GetSkill(anomalyType, actionName)
				requiredLocalThreshold := execEngine.GetActionRiskThreshold(actionName)

				if skill != nil && skill.Confidence >= requiredLocalThreshold {
					logger.Info("SkillStore Cache Hit! Executing Local Skill [%s] (Confidence=%.2f >= Required=%.2f)...", skill.Action, skill.Confidence, requiredLocalThreshold)
					actReq := &executor.ActionRequest{
						ActionName: skill.Action,
						Target:     "wan",
					}
					// A1: Tạo UCI Snapshot trước khi thực thi lệnh High Risk
					if requiredLocalThreshold >= 0.85 {
						_ = exec.Command("/bin/sh", "-c", "uci export > /tmp/agent_checkpoint.uci").Run()
					}
					execErr := execEngine.ExecuteAction(ctx, actReq, cfg.DryRun)
					_ = store.SaveOrUpdateSkill(skill, execErr == nil, cfg.EMAAlpha)
					continue
				}

				aiResp, aiErr := aiClient.AnalyzeAnomaly(ctx, anomalyType, anomalyDesc, liveLogSample)

				if aiErr == nil && aiResp != nil {
					requiredThreshold := execEngine.GetActionRiskThreshold(aiResp.Action)

					if aiResp.Confidence >= requiredThreshold {
						logger.Info("Gemini AI Action Approved: [%s] (Confidence=%.2f >= Required=%.2f) - Reasoning: %s", aiResp.Action, aiResp.Confidence, requiredThreshold, aiResp.Reasoning)
						actReq := &executor.ActionRequest{
							ActionName: aiResp.Action,
							Target:     "wan",
						}
						// A1: Tạo UCI Snapshot trước khi thực thi lệnh High Risk
						if requiredThreshold >= 0.85 {
							_ = exec.Command("/bin/sh", "-c", "uci export > /tmp/agent_checkpoint.uci").Run()
						}
						execErr := execEngine.ExecuteAction(ctx, actReq, cfg.DryRun)

						newSkill := &skillstore.Skill{
							ID:         fmt.Sprintf("%s:%s", anomalyType, aiResp.Action),
							Action:     aiResp.Action,
							Condition:  anomalyType,
							Confidence: aiResp.Confidence,
						}
						_ = store.SaveOrUpdateSkill(newSkill, execErr == nil, cfg.EMAAlpha)
					} else {
						logger.Warn("SECURITY GATING TRIGGERED: AI Action [%s] Confidence (%.2f) below Required Risk Threshold (%.2f)! Queued for Operator Approval.", aiResp.Action, aiResp.Confidence, requiredThreshold)
						queuePendingApproval(aiResp, requiredThreshold)
						// A2: Selective Rollback - Chỉ rollback khi là WAN_DROP hoặc High Risk action
						if anomalyType == "WAN_DROP" || requiredThreshold >= 0.85 {
							_ = wd.ExecuteRollback()
						}
					}
				} else {
					logger.Error("Cloud AI Call Failed (%v). Processing Selective Fallback Guardrail!", aiErr)
					if anomalyType == "WAN_DROP" {
						_ = wd.ExecuteRollback()
					}
				}
			}
		}
	}
}

func getSystemLogSample() string {
	if runtime.GOOS == "linux" {
		out, err := exec.Command("/sbin/logread", "-l", "15").Output()
		if err == nil && len(out) > 0 {
			return string(out)
		}
	}
	data, err := os.ReadFile("/var/log/messages")
	if err == nil && len(data) > 0 {
		lines := strings.Split(string(data), "\n")
		if len(lines) > 15 {
			return strings.Join(lines[len(lines)-15:], "\n")
		}
		return string(data)
	}
	return ""
}

func queuePendingApproval(resp *ai.AIResponse, required float64) {
	pending := PendingApproval{
		Action:     resp.Action,
		Reasoning:  resp.Reasoning,
		Confidence: resp.Confidence,
		Required:   required,
		Timestamp:  time.Now(),
	}
	data, err := json.MarshalIndent(pending, "", "  ")
	if err == nil {
		_ = os.WriteFile("/var/run/beryl7_pending_approval.json", data, 0600)
		logger.Info("Saved pending approval request to /var/run/beryl7_pending_approval.json")
	}
}

func recordApprovalAuditLog(action, remoteAddr string) {
	auditLine := fmt.Sprintf("[%s] AUDIT: Operator approved action [%s] from %s\n", time.Now().Format(time.RFC3339), action, remoteAddr)
	f, err := os.OpenFile("/var/log/beryl7_approval_audit.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err == nil {
		_, _ = f.WriteString(auditLine)
		_ = f.Sync() // Đảm bảo đồng bộ ngay lập tức vào ổ đĩa trước khi thi hành lệnh
		_ = f.Close()
	}
}

func acquirePIDLock(pidPath string) error {
	cleanPath := filepath.Clean(pidPath)
	if content, err := os.ReadFile(cleanPath); err == nil && len(content) > 0 { // #nosec G304
		var oldPID int
		if _, parseErr := fmt.Sscanf(string(content), "%d", &oldPID); parseErr == nil && oldPID > 0 {
			if checkPIDAlive(oldPID) {
				return fmt.Errorf("beryl7-agent is already running with PID %d", oldPID)
			}
			logger.Info("Removing stale PID file (PID %d is dead)", oldPID)
			_ = os.Remove(pidPath)
		}
	}

	return os.WriteFile(pidPath, []byte(strconv.Itoa(os.Getpid())), 0600)
}

func startHealthCheckServer(cfg *config.Config, health *HealthState, execEngine *executor.Executor) *http.Server {
	mux := http.NewServeMux()

	var (
		rateMu    sync.Mutex
		ipCounts  = make(map[string]int)
		lastReset = time.Now()
	)

	rateLimitCheck := func(ip string) bool {
		rateMu.Lock()
		defer rateMu.Unlock()
		if time.Since(lastReset) > time.Minute {
			ipCounts = make(map[string]int)
			lastReset = time.Now()
		}
		ipCounts[ip]++
		return ipCounts[ip] <= 30
	}

	// Endpoint 1: Health Check (AUTH_TOKEN) - D4: Fail-Closed khi AUTH_TOKEN rỗng
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		host, _, _ := net.SplitHostPort(r.RemoteAddr)
		if host == "" {
			host = r.RemoteAddr
		}
		if !rateLimitCheck(host) {
			http.Error(w, `{"error":"Too Many Requests: Rate limit exceeded (30 req/min)"}`, http.StatusTooManyRequests)
			return
		}
		if cfg.AuthToken == "" {
			http.Error(w, `{"error":"Forbidden: AUTH_TOKEN must be configured"}`, http.StatusForbidden)
			return
		}

		auth := r.Header.Get("Authorization")
		expectedAuth := "Bearer " + cfg.AuthToken

		if len(auth) == 0 || subtle.ConstantTimeCompare([]byte(auth), []byte(expectedAuth)) != 1 {
			http.Error(w, `{"error":"Unauthorized"}`, http.StatusUnauthorized)
			return
		}

		health.mu.RLock()
		snapshot := *health
		health.mu.RUnlock()

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(snapshot)
	})

	// Endpoint 2: Operator Approval Endpoint (/api/approve) - Fail-Closed Hardened
	mux.HandleFunc("/api/approve", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, `{"error":"Method Not Allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		// Fail-Closed Policy: Bắt buộc APPROVE_TOKEN phải được cấu hình riêng biệt và khác AUTH_TOKEN
		if cfg.ApproveToken == "" || cfg.ApproveToken == cfg.AuthToken {
			http.Error(w, `{"error":"Forbidden: Operator APPROVE_TOKEN must be set distinctly from AUTH_TOKEN for approval safety"}`, http.StatusForbidden)
			return
		}

		auth := r.Header.Get("Authorization")
		expectedAuth := "Bearer " + cfg.ApproveToken

		if len(auth) == 0 || subtle.ConstantTimeCompare([]byte(auth), []byte(expectedAuth)) != 1 {
			http.Error(w, `{"error":"Unauthorized: Invalid High-Privilege APPROVE_TOKEN"}`, http.StatusUnauthorized)
			return
		}

		pendingFile := "/var/run/beryl7_pending_approval.json"
		data, err := os.ReadFile(pendingFile)
		if err != nil {
			http.Error(w, `{"error":"No pending approval request found"}`, http.StatusNotFound)
			return
		}

		var pending PendingApproval
		if err := json.Unmarshal(data, &pending); err != nil {
			http.Error(w, `{"error":"Failed to parse pending approval file"}`, http.StatusInternalServerError)
			return
		}

		// D3: Hạn ngạch Expiry - Nếu yêu cầu đã quá 10 phút thì reject và xóa file
		if time.Since(pending.Timestamp) > 10*time.Minute {
			_ = os.Remove(pendingFile)
			http.Error(w, `{"error":"Pending approval request expired (> 10 minutes)"}`, http.StatusGone)
			return
		}

		// Ghi Append-Only Audit Trail Log và fsync ngay lập tức trước khi thi hành
		recordApprovalAuditLog(pending.Action, r.RemoteAddr)

		actReq := &executor.ActionRequest{
			ActionName: pending.Action,
			Target:     "wan",
		}
		execErr := execEngine.ExecuteAction(r.Context(), actReq, cfg.DryRun)

		_ = os.Remove(pendingFile)

		w.Header().Set("Content-Type", "application/json")
		if execErr != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"status": "approval_executed_with_error",
				"error":  execErr.Error(),
			})
			return
		}

		logger.Info("OPERATOR APPROVED & EXECUTED Action [%s] successfully with APPROVE_TOKEN!", pending.Action)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status":  "approved_and_executed",
			"action":  pending.Action,
			"details": "Operator approval verified and executed successfully",
		})
	})

	server := &http.Server{
		Addr:         fmt.Sprintf("0.0.0.0:%d", cfg.HealthPort),
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	go func() {
		lc := net.ListenConfig{
			Control: func(network, address string, c syscall.RawConn) error {
				var opErr error
				err := c.Control(func(fd uintptr) {
					opErr = setSoReuseAddr(fd)
				})
				if err != nil {
					return err
				}
				return opErr
			},
		}

		listener, err := lc.Listen(context.Background(), "tcp", server.Addr)
		if err != nil {
			logger.Error("HTTP Health Server listen failed: %v", err)
			return
		}

		logger.Info("HTTP Health Server started securely on %s", server.Addr)
		_ = server.Serve(listener)
	}()

	return server
}
