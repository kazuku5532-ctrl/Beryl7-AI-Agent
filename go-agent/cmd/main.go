package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
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

	httpServer := startHealthCheckServer(cfg, health)

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

			if m.WANStatus == "Offline (0/1)" || strings.Contains(m.WANStatus, "Offline") {
				logger.Warn("WAN Network Drop Detected! Processing Auto-Healing Action...")

				// 1. Tra cứu Local SkillStore
				skill := store.GetSkill("restart_wan_interface")
				requiredLocalThreshold := execEngine.GetActionRiskThreshold("restart_wan_interface")

				if skill != nil && skill.Confidence >= requiredLocalThreshold {
					logger.Info("SkillStore Cache Hit! Executing Local Skill [%s] (Confidence=%.2f >= Required=%.2f)...", skill.Action, skill.Confidence, requiredLocalThreshold)
					actReq := &executor.ActionRequest{
						ActionName: skill.Action,
						Target:     "wan",
					}
					execErr := execEngine.ExecuteAction(ctx, actReq, cfg.DryRun)
					_ = store.SaveOrUpdateSkill(skill, execErr == nil, cfg.EMAAlpha)
					continue
				}

				// 2. Gọi Cloud AI với Log đã được sanitize
				cleanLog := logParser.SanitizeLog("kernel: eth1 link down (WAN disconnected)")
				aiResp, aiErr := aiClient.AnalyzeAnomaly(ctx, "WAN_DROP", "WAN interface down", cleanLog)

				if aiErr == nil && aiResp != nil {
					requiredThreshold := execEngine.GetActionRiskThreshold(aiResp.Action)

					// Gating Matrix: Kiểm tra xem Confidence có vượt qua Ngưỡng Rủi ro quy định hay không
					if aiResp.Confidence >= requiredThreshold {
						logger.Info("Gemini AI Action Approved: [%s] (Confidence=%.2f >= Required=%.2f) - Reasoning: %s", aiResp.Action, aiResp.Confidence, requiredThreshold, aiResp.Reasoning)
						actReq := &executor.ActionRequest{
							ActionName: aiResp.Action,
							Target:     "wan",
						}
						execErr := execEngine.ExecuteAction(ctx, actReq, cfg.DryRun)

						newSkill := &skillstore.Skill{
							ID:         aiResp.Action,
							Action:     aiResp.Action,
							Condition:  "WAN_DROP",
							Confidence: aiResp.Confidence,
						}
						_ = store.SaveOrUpdateSkill(newSkill, execErr == nil, cfg.EMAAlpha)
					} else {
						// Nếu Confidence < Required Threshold: Queue cho Operator duyệt và log cảnh báo
						logger.Warn("SECURITY GATING TRIGGERED: AI Action [%s] Confidence (%.2f) below Required Risk Threshold (%.2f)! Queued for Operator Approval.", aiResp.Action, aiResp.Confidence, requiredThreshold)
						queuePendingApproval(aiResp, requiredThreshold)
						_ = wd.ExecuteRollback()
					}
				} else {
					logger.Error("Cloud AI Call Failed (%v). Falling back to Watchdog Rollback Guardrail!", aiErr)
					_ = wd.ExecuteRollback()
				}
			}
		}
	}
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

func acquirePIDLock(pidPath string) error {
	if content, err := os.ReadFile(pidPath); err == nil && len(content) > 0 {
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

func startHealthCheckServer(cfg *config.Config, health *HealthState) *http.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		expectedAuth := "Bearer " + cfg.AuthToken

		if cfg.AuthToken != "" {
			if len(auth) == 0 || subtle.ConstantTimeCompare([]byte(auth), []byte(expectedAuth)) != 1 {
				http.Error(w, `{"error":"Unauthorized"}`, http.StatusUnauthorized)
				return
			}
		}

		health.mu.RLock()
		snapshot := *health
		health.mu.RUnlock()

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(snapshot)
	})

	server := &http.Server{
		Addr:         fmt.Sprintf("127.0.0.1:%d", cfg.HealthPort),
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

		logger.Info("HTTP Health Server started securely on loopback %s", server.Addr)
		_ = server.Serve(listener)
	}()

	return server
}
