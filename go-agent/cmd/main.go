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

func main() {
	// 1. OOM Score -500 bảo vệ Daemon không bị Linux Kernel OOM Killer xóa sổ
	setOOMScore()

	// 2. Nạp Cấu hình
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// 3. Khởi tạo Logger xoay vòng /var/log/beryl7_agent.log
	_, err = logger.Init("/var/log/beryl7_agent.log", cfg.LogLevel)
	if err != nil {
		fmt.Printf("Failed to init logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Flush()

	logger.Info("Starting Beryl 7 AI Agent v14.1 Security Hardened Edition (Native Go)...")

	// 4. Khóa File PID Lock (/var/run/beryl7-agent.pid) với chmod 0600 bảo mật cao
	pidPath := "/var/run/beryl7-agent.pid"
	if err := acquirePIDLock(pidPath); err != nil {
		logger.Fatal("PID Lock Error: %v", err)
	}
	defer os.Remove(pidPath)

	// 5. Khởi tạo SQLite Pure-Go SkillStore
	store, err := skillstore.New(cfg.SkillStorePath)
	if err != nil {
		logger.Fatal("SkillStore Init Error: %v", err)
	}
	defer store.Close()

	// 6. Khởi tạo Watchdog Checkpoint Engine
	wd := watchdog.New(cfg.CheckpointPath)

	// 7. Khởi tạo Telemetry, Parser, Executor & AI Client
	collector := telemetry.NewCollector()
	logParser := parser.NewParser()
	execEngine := executor.New()
	aiClient := ai.NewClient(cfg.GeminiAPIKey)

	// Chạy Async DNS Probe ngầm
	ai.ProbeDNSAsync()

	health := &HealthState{
		Status:        "healthy",
		LastAction:    "none",
		WANStatus:     "Active (1/1)",
		StartTime:     time.Now(),
		UptimeSeconds: 0,
	}

	// 8. Khởi chạy HTTP Health Check Server (Bind 127.0.0.1 Loopback Only)
	httpServer := startHealthCheckServer(cfg, health)

	// 9. Lập lịch ngầm Shutdown Signal Trap (SIGTERM / SIGINT)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		sig := <-sigCh
		logger.Info("Received shutdown signal (%v)! Initiating Adaptive Graceful Shutdown...", sig)

		// Pha 1: Đóng HTTP Server (2s)
		ctxHTTP, cancelHTTP := context.WithTimeout(context.Background(), 2*time.Second)
		_ = httpServer.Shutdown(ctxHTTP)
		cancelHTTP()

		// Pha 2: Fsync SQLite Database (2s)
		_ = store.Close()

		// Pha 3: Flush Logger (1s)
		logger.Flush()

		cancel()
		os.Exit(0)
	}()

	// 10. Vòng lặp chính của Daemon (24/7 Main Daemon Loop)
	logger.Info("Daemon initialized successfully. Security Hardened Engine listening on 24/7 main loop...")

	ticker := time.NewTicker(cfg.TelemetryInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Prune kỹ năng định kỳ 24h hoặc khi >= 1000 skills
			_ = store.PruneSkillsPeriodic()

			// Thu thập Telemetry
			m := collector.CollectMetrics(ctx)
			if m == nil {
				continue
			}

			// Cập nhật trạng thái Health State cho HTTP Handler
			health.mu.Lock()
			health.CPUUsagePct = m.CPUUsagePct
			health.RAMUsagePct = m.RAMUsagePct
			health.WANStatus = m.WANStatus
			health.UptimeSeconds = int64(time.Since(health.StartTime).Seconds())
			health.SafeMode = wd.IsSafeMode()
			health.KillSwitch = config.IsKillSwitchActive(cfg)
			health.mu.Unlock()

			// Nếu Safe Mode đang bật, đếm số lần Health Check thành công để tự thoát Safe Mode
			if wd.IsSafeMode() {
				wd.RecordHealthCheckSuccess()
				continue
			}

			// Nếu Kill Switch đang bật, ngắt tự can thiệp mạng
			if config.IsKillSwitchActive(cfg) {
				logger.Warn("Kill Switch Active (/tmp/beryl7-disable or env)! Auto-healing suspended.")
				continue
			}

			// Phát hiện rớt mạng WAN
			if m.WANStatus == "Offline (0/1)" || strings.Contains(m.WANStatus, "Offline") {
				logger.Warn("WAN Network Drop Detected! Processing Auto-Healing Action...")

				// Ưu tiên 1: Tra cứu tri thức Local từ SQLite SkillStore (< 1ms)
				skill := store.GetSkill("restart_wan_interface")
				if skill != nil && skill.Confidence >= 0.6 {
					logger.Info("SkillStore Cache Hit! Executing Local Skill [%s] (Confidence=%.2f)...", skill.Action, skill.Confidence)
					actReq := &executor.ActionRequest{
						ActionName: skill.Action,
						Target:     "wan",
					}
					execErr := execEngine.ExecuteAction(ctx, actReq, cfg.DryRun)
					_ = store.SaveOrUpdateSkill(skill, execErr == nil, cfg.EMAAlpha)
					continue
				}

				// Sanitize log rác và nhạy cảm trước khi gửi tới AI Cloud
				cleanLog := logParser.SanitizeLog("kernel: eth1 link down (WAN disconnected)")

				// Ưu tiên 2: Gọi Gemini 2.5 Flash API với log đã được làm sạch
				aiResp, aiErr := aiClient.AnalyzeAnomaly(ctx, "WAN_DROP", "WAN interface down", cleanLog)

				// Kiểm tra Confidence Threshold (>= 0.50) trước khi tự động thực thi
				if aiErr == nil && aiResp != nil && aiResp.Confidence >= 0.50 {
					logger.Info("Gemini AI Suggested Action: [%s] (Confidence=%.2f) - Reasoning: %s", aiResp.Action, aiResp.Confidence, aiResp.Reasoning)
					actReq := &executor.ActionRequest{
						ActionName: aiResp.Action,
						Target:     "wan",
					}
					execErr := execEngine.ExecuteAction(ctx, actReq, cfg.DryRun)

					// Lưu kỹ năng mới vào SQLite
					newSkill := &skillstore.Skill{
						ID:         aiResp.Action,
						Action:     aiResp.Action,
						Condition:  "WAN_DROP",
						Confidence: aiResp.Confidence,
					}
					_ = store.SaveOrUpdateSkill(newSkill, execErr == nil, cfg.EMAAlpha)
				} else {
					logger.Error("Cloud AI Call Failed or Confidence Low (%v). Falling back to Watchdog Rollback Guardrail!", aiErr)
					_ = wd.ExecuteRollback()
				}
			}
		}
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

	// Khắc phục theo Đề xuất docx: Bind giao diện loopback 127.0.0.1 thay vì 0.0.0.0
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
