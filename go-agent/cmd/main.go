package main

import (
	"context"
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
	// 1. Cấu hình điểm OOM Score -500 bảo vệ tiến trình Daemon không bị Linux OOM Killer xóa sổ
	setOOMScore()

	// 2. Nạp Cấu hình Cấp độ Doanh nghiệp
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// 3. Khởi tạo Logger xoay vòng /var/log/beryl7_agent.log (2MB max, Inode Safe)
	_, err = logger.Init("/var/log/beryl7_agent.log", cfg.LogLevel)
	if err != nil {
		fmt.Printf("Failed to init logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Flush()

	logger.Info("Starting Beryl 7 AI Agent v14.0 Master Evolution (Native Go)...")

	// 4. Khóa File PID Lock (/var/run/beryl7-agent.pid)
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

	// 7. Khởi tạo Telemetry & Parser
	collector := telemetry.NewCollector()
	logParser := parser.NewParser()
	execEngine := executor.New()
	aiClient := ai.NewClient(cfg.GeminiAPIKey)

	// Suppress unused variable
	_ = logParser

	// Chạy Async DNS Probe ngầm
	ai.ProbeDNSAsync()

	health := &HealthState{
		Status:        "healthy",
		LastAction:    "none",
		WANStatus:     "Active (1/1)",
		StartTime:     time.Now(),
		UptimeSeconds: 0,
	}

	// 8. Khởi chạy HTTP Health Check Server :8888 với SO_REUSEADDR Socket Listener
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
	logger.Info("Daemon initialized successfully. Engine listening on 24/7 main loop...")

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

			// Mô phỏng phát hiện rớt mạng WAN
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

				// Ưu tiên 2: Gọi Gemini 2.5 Flash API nếu local cache chưa có
				aiResp, aiErr := aiClient.AnalyzeAnomaly(ctx, "WAN_DROP", "WAN interface down", "link down")
				if aiErr == nil && aiResp != nil {
					logger.Info("Gemini AI Suggested Action: [%s] - Reasoning: %s", aiResp.Action, aiResp.Reasoning)
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
						Confidence: 0.5,
					}
					_ = store.SaveOrUpdateSkill(newSkill, execErr == nil, cfg.EMAAlpha)
				} else {
					logger.Error("Cloud AI Call Failed (%v). Falling back to Watchdog Rollback Guardrail!", aiErr)
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
			// Kiểm tra tiến trình sống/chết qua checkPIDAlive (build tag cho Linux/Windows)
			if checkPIDAlive(oldPID) {
				return fmt.Errorf("beryl7-agent is already running with PID %d", oldPID)
			}
			logger.Info("Removing stale PID file (PID %d is dead)", oldPID)
			_ = os.Remove(pidPath)
		}
	}

	return os.WriteFile(pidPath, []byte(strconv.Itoa(os.Getpid())), 0644)
}

func startHealthCheckServer(cfg *config.Config, health *HealthState) *http.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		// Yêu cầu Secret Bearer Token
		auth := r.Header.Get("Authorization")
		expectedAuth := "Bearer " + cfg.AuthToken
		if cfg.AuthToken != "" && auth != expectedAuth {
			http.Error(w, `{"error":"Unauthorized"}`, http.StatusUnauthorized)
			return
		}

		// Hàm Handler CHỈ ĐỌC từ snapshot bộ nhớ RLock(), 100% KHÔNG gọi ubus hay telemetry trực tiếp
		health.mu.RLock()
		snapshot := *health
		health.mu.RUnlock()

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(snapshot)
	})

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.HealthPort),
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	// Lắng nghe với cờ SO_REUSEADDR chống dính lỗi bind cổng khi restart service
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

		logger.Info("HTTP Health Server started on port %s", server.Addr)
		_ = server.Serve(listener)
	}()

	return server
}
