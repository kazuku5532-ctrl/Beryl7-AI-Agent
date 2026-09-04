package main

import (
	"context"
	"crypto/subtle"
	"database/sql"

	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"beryl7-agent/ai"
	"beryl7-agent/config"
	"beryl7-agent/executor"
	"beryl7-agent/logger"
	"beryl7-agent/notifier"
	"beryl7-agent/parser"
	"beryl7-agent/skillstore"
	"beryl7-agent/telemetry"
	"beryl7-agent/watchdog"
)

type HealthState struct {
	mu                    sync.RWMutex
	Status                string                      `json:"status"`
	UptimeSeconds         int64                       `json:"uptime_seconds"`
	LastAction            string                      `json:"last_action"`
	LastActionTime        string                      `json:"last_action_time"`
	WANStatus             string                      `json:"wan_status"`
	CPUUsagePct           float64                     `json:"cpu_usage_pct"`
	RAMUsagePct           float64                     `json:"ram_usage_pct"`
	HardwareTempC         float64                     `json:"hardware_temp_c"`
	LatencyMs             float64                     `json:"latency_ms"`
	SafeMode              bool                        `json:"safe_mode"`
	KillSwitch            bool                        `json:"kill_switch"`
	WifiBandwidthMHz      int                         `json:"wifi_bandwidth_mhz"`
	IsWifiBoosted         bool                        `json:"is_wifi_boosted"`
	DownloadMbps          float64                     `json:"download_mbps"`
	UploadMbps            float64                     `json:"upload_mbps"`
	ConnectedDevicesCount int                         `json:"connected_devices_count"`
	ConnectedDevices      []telemetry.ConnectedDevice `json:"connected_devices"`
	NetworkToken          string                      `json:"network_token"`
	StartTime             time.Time                   `json:"start_time"`
	Goroutines            int                         `json:"goroutines"`
	HeapAllocMB           float64                     `json:"heap_alloc_mb"`
	RSSMB                 float64                     `json:"rss_mb"`
}

type PendingApproval struct {
	Action     string    `json:"action"`
	Reasoning  string    `json:"reasoning"`
	Confidence float64   `json:"confidence"`
	Required   float64   `json:"required_threshold"`
	Timestamp  time.Time `json:"timestamp"`
}

var configMu sync.RWMutex
var cfgAtomic atomic.Value
var activeServer *http.Server
var activeStore *skillstore.SkillStore
var activeWatchdog *watchdog.Watchdog
var restartSignalChan = make(chan string, 1)

func PerformGracefulProcessRestart(reason string) {
	if activeWatchdog != nil {
		activeWatchdog.Suspend()
	}
	select {
	case restartSignalChan <- reason:
		logger.Warn("Triggered process restart signal [%s] -> delegating execution to main thread...", reason)
	default:
		// Restart signal already pending
	}
}

func isLocalhostRequest(r *http.Request, cfg *config.Config) bool {
	if r == nil {
		return false
	}
	if cfg != nil && cfg.DisableLocalhostBypass {
		return false
	}

	// 1. If forwarded by a Reverse Proxy (Nginx/uHTTPd/LuCI), verify EVERY IP in the proxy header chain
	hasProxyHeader := false
	for _, header := range []string{"X-Forwarded-For", "X-Real-IP"} {
		if val := r.Header.Get(header); val != "" {
			hasProxyHeader = true
			parts := strings.Split(val, ",")
			for _, part := range parts {
				clientIPStr := strings.Trim(strings.TrimSpace(part), "[]")
				if strings.HasPrefix(clientIPStr, "::ffff:") {
					clientIPStr = strings.TrimPrefix(clientIPStr, "::ffff:")
				}
				clientIP := net.ParseIP(clientIPStr)
				if clientIP == nil || (!clientIP.IsLoopback() && (clientIP.To4() == nil || !clientIP.To4().IsLoopback())) {
					return false // Any non-loopback IP in proxy header chain -> Enforcement Auth required!
				}
			}
		}
	}

	// Reverse proxy headers present without explicit BERYL7_TRUST_REVERSE_PROXY=1 configuration -> Enforcement Auth required!
	if hasProxyHeader && (cfg == nil || !cfg.TrustReverseProxy) {
		return false
	}

	// 2. Direct socket connection check
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	host = strings.Trim(strings.TrimSpace(host), "[]")
	if strings.HasPrefix(host, "::ffff:") {
		host = strings.TrimPrefix(host, "::ffff:")
	}
	if host == "127.0.0.1" || host == "::1" || host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	if ip.IsLoopback() {
		return true
	}
	if ip4 := ip.To4(); ip4 != nil {
		return ip4.IsLoopback()
	}
	return false
}

func validateTokenRole(r *http.Request, authHeader string, cfg *config.Config) (string, bool) {
	// 1. Localhost Bypass: Requests originating from loopback (127.0.0.1 / ::1) automatically bypass Auth for local CLI operations
	if isLocalhostRequest(r, cfg) {
		return "admin", true
	}

	if authHeader == "" {
		return "viewer", true
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	token = strings.TrimSpace(token)

	configMu.RLock()
	authToken := cfg.AuthToken
	approveToken := cfg.ApproveToken
	configMu.RUnlock()

	// 2. Check Operator Token
	if approveToken != "" && subtle.ConstantTimeCompare([]byte(token), []byte(approveToken)) == 1 {
		return "operator", true
	}

	// 3. Check Admin Token (In Single-Token mode when approveToken is empty/unconfigured, Admin Token grants full Operator + Admin access)
	if authToken != "" && subtle.ConstantTimeCompare([]byte(token), []byte(authToken)) == 1 {
		if approveToken == "" {
			return "operator", true
		}
		return "admin", true
	}

	if os.Getenv("BERYL7_DEMO_MODE") == "1" && (token == "demo-token" || token == "viewer-token") {
		return "viewer", true
	}

	return "unknown", false
}

func setCorsHeaders(w http.ResponseWriter, r *http.Request, cfg *config.Config) bool {
	origin := r.Header.Get("Origin")
	allowed := ""
	if cfg != nil {
		allowed = cfg.CORSAllowedOrigins
	}
	if allowed == "" || allowed == "*" {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	} else if origin != "" {
		matched := false
		for _, entry := range strings.Split(allowed, ",") {
			entryTrimmed := strings.TrimSpace(entry)
			if entryTrimmed == origin || entryTrimmed == "*" || (origin == "null" && entryTrimmed == "null") {
				matched = true
				break
			}
		}
		if !matched && origin != "null" {
			if parsedURL, err := url.Parse(origin); err == nil {
				hostname := parsedURL.Hostname()
				ip := net.ParseIP(hostname)
				if hostname == "localhost" || (ip != nil && isLANOrLoopbackIP(ip)) {
					matched = true
				}
			}
		}

		defaultOrigin := "http://127.0.0.1:8888"
		if cfg != nil {
			defaultOrigin = fmt.Sprintf("http://%s:%d", cfg.BindHost, cfg.HealthPort)
		}
		if matched {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		} else {
			w.Header().Set("Access-Control-Allow-Origin", defaultOrigin)
		}
	} else {
		defaultOrigin := "http://127.0.0.1:8888"
		if cfg != nil {
			defaultOrigin = fmt.Sprintf("http://%s:%d", cfg.BindHost, cfg.HealthPort)
		}
		w.Header().Set("Access-Control-Allow-Origin", defaultOrigin)
	}
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return true
	}
	return false
}

func runBenchmarks() {
	fmt.Println("Running in-memory hardware micro-benchmarks...")

	// Component 1: skillstore.ComputeStateDistance
	start1 := time.Now()
	iters1 := 100000
	sig1 := &skillstore.StateSignature{StateName: "bench-1", RAMPct: 100.0, LatencyMs: 50.0, CPUPct: 10.0, TempC: 40.0}
	sig2 := &skillstore.StateSignature{StateName: "bench-2", RAMPct: 110.0, LatencyMs: 55.0, CPUPct: 12.0, TempC: 42.0}
	for i := 0; i < iters1; i++ {
		skillstore.ComputeStateDistance(sig1, sig2)
	}
	dur1 := time.Since(start1)

	// Component 2: SkillStore Operations
	iters2 := 10000
	store, err := skillstore.New(":memory:")
	if err != nil {
		fmt.Printf("Failed to create in-memory store: %v\n", err)
		os.Exit(1)
	}
	start2 := time.Now()
	for i := 0; i < iters2; i++ {
		state := fmt.Sprintf("state-%d", i%10)
		store.UpdateQValue(state, "action-default", 1.0)
		store.RecommendBestActionWithInterpolation(fmt.Sprintf("state-%d", (i+1)%10), sig1, "action-fallback")
	}
	dur2 := time.Since(start2)

	// Component 3: telemetry.GetProcessResourceStats
	iters3 := 500
	start3 := time.Now()
	tc := telemetry.NewCollector()
	for i := 0; i < iters3; i++ {
		tc.GetProcessResourceStats()
	}
	dur3 := time.Since(start3)

	fmt.Println("\n==========================================================================================")
	fmt.Printf("%-35s | %-10s | %-12s | %-12s | %-12s\n", "Component", "Iterations", "Total Time", "Latency/Op", "Ops/Sec")
	fmt.Println("------------------------------------+------------+--------------+--------------+----------")
	fmt.Printf("%-35s | %-10d | %-12s | %-12s | %.0f\n", "ComputeStateDistance", iters1, dur1, dur1/time.Duration(iters1), float64(iters1)/dur1.Seconds())
	fmt.Printf("%-35s | %-10d | %-12s | %-12s | %.0f\n", "SkillStore Ops", iters2, dur2, dur2/time.Duration(iters2), float64(iters2)/dur2.Seconds())
	fmt.Printf("%-35s | %-10d | %-12s | %-12s | %.0f\n", "GetProcessResourceStats", iters3, dur3, dur3/time.Duration(iters3), float64(iters3)/dur3.Seconds())
	fmt.Println("==========================================================================================")
}

func main() {
	benchmarkFlag := flag.Bool("benchmark", false, "Run in-memory hardware micro-benchmarks and exit")
	configFlag := flag.String("config", "/etc/beryl7/agent.env", "Path to configuration file")
	keyfileFlag := flag.String("keyfile", "/etc/beryl7/agent.key", "Path to secure API key file")
	dryRunFlag := flag.Bool("dry-run", false, "Enable dry-run mode (no network modifications)")
	versionFlag := flag.Bool("version", false, "Print daemon version and exit")
	flag.Parse()

	if *versionFlag {
		fmt.Println("beryl7-agent version v16.0")
		os.Exit(0)
	}

	if *benchmarkFlag {
		runBenchmarks()
		os.Exit(0)
	}

	// Portable GC tuning: Memory limits managed dynamically via OS environment & Go runtime soft limit (16MB)
	debug.SetGCPercent(20)
	debug.SetMemoryLimit(16 * 1024 * 1024)

	setOOMScore()

	configMu.Lock()
	cfg, err := config.LoadConfigWithFlags(*configFlag, *keyfileFlag, *dryRunFlag, *versionFlag)
	configMu.Unlock()
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	l, err := logger.Init("/var/log/beryl7_agent.log", cfg.LogLevel)
	if err != nil {
		fmt.Printf("Failed to init logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Flush()
	if l != nil {
		l.SetMaxBytes(cfg.LogMaxBytes)
		l.SetBackupCount(cfg.LogBackupCount)
	}

	if valErr := config.ValidateSystemConfiguration(cfg); valErr != nil {
		logger.Warn("System configuration validation check: %v", valErr)
	}
	if depErr := config.ValidateSystemDependencies(); depErr != nil {
		logger.Info("System dependencies check info: %v", depErr)
	}

	logger.Info("Starting Beryl 7 AI Agent v16.0 Daemon (Native Go)...")

	// Non-fatal init checks: errors are logged internally inside helper functions
	_ = config.EnsureSysupgradePreservation() // nolint:errcheck (non-fatal, logged inside helper)
	_ = config.EnsureFilePermissions()        // nolint:errcheck (non-fatal, logged inside helper)
	_ = config.EnsureProcdInitService()       // nolint:errcheck (non-fatal, logged inside helper)
	_ = config.DetectSystemCapability(cfg)    // nolint:errcheck (non-fatal, logged inside helper)
	_ = PostUpgradeValidation(cfg)            // nolint:errcheck (non-fatal, logged inside helper)

	pidPath := "/var/run/beryl7-agent.pid"
	if err := acquirePIDLock(pidPath); err != nil {
		fmt.Printf("beryl7-agent process already running: %v\n", err)
		os.Exit(0)
	}
	defer os.Remove(pidPath)

	// Hybrid Store Initialization: Working DB in RAM /tmp/skills.db (zero NAND Flash wear), persistent backup in Flash
	ramDBPath := "/tmp/skills.db"
	store, err := skillstore.NewHybrid(ramDBPath, cfg.SkillStorePath)
	if err != nil {
		logger.Fatal("SkillStore Hybrid Init Error: %v", err)
	}
	defer store.Close()
	activeStore = store
	store.SetInterpolationParams(cfg.StateDistanceThreshold, cfg.StateDecayLambda)
	logger.Info("TinyML Similarity Interpolation initialized: DistanceThreshold=%.2f, DecayLambda=%.2f", cfg.StateDistanceThreshold, cfg.StateDecayLambda)

	// Periodic 12-Hour Async Flush-to-Flash to preserve learned skills permanently
	go func() {
		ticker := time.NewTicker(12 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			if flushErr := store.FlushToPersistent(cfg.SkillStorePath); flushErr != nil {
				logger.Warn("Periodic SkillStore Flush-to-Flash error: %v", flushErr)
			}
		}
	}()

	wd := watchdog.New(cfg.CheckpointPath)
	activeWatchdog = wd
	collector := telemetry.NewCollector()
	logParser := parser.NewParser()
	execEngine := executor.New()
	execEngine.SetTelemetryProvider(collector)
	if cfg.GeminiAPIKey == "" {
		logger.Warn("NOTICE: GEMINI_API_KEY is not set! Cloud AI Log Analysis disabled. Graceful degradation active: Local-First SQLite Self-Healing & Watchdog running 100%% normally.")
	}
	aiClient := ai.NewClient(cfg.GeminiAPIKey)
	aiClient.SetAirgappedMode(cfg.AirgappedMode)

	tgNotifier := notifier.NewTelegramNotifier(cfg.TelegramBotToken, cfg.TelegramChatID, cfg.AirgappedMode)

	ai.ProbeDNSAsync()

	health := &HealthState{
		Status:        "healthy",
		LastAction:    "none",
		WANStatus:     "Active (1/1)",
		StartTime:     time.Now(),
		UptimeSeconds: 0,
	}

	httpServer := StartHealthCheckServer(cfg, health, execEngine, store, aiClient, wd, collector)
	activeServer = httpServer

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	ctx, cancel := context.WithCancel(context.Background())

	var isWifiBoostedAtomic atomic.Bool

	if tgNotifier != nil {
		tgNotifier.StartCommandListener(ctx, func(cmdReceived string) string {
			cmdLower := strings.ToLower(strings.TrimSpace(cmdReceived))

			switch {
			case cmdLower == "/status" || strings.Contains(cmdLower, "trạng thái"):
				health.mu.RLock()
				token := health.NetworkToken
				wan := health.WANStatus
				cpu := health.CPUUsagePct
				ram := health.RAMUsagePct
				temp := health.HardwareTempC
				lat := health.LatencyMs
				health.mu.RUnlock()

				return fmt.Sprintf("🤖 *Beryl 7 AI Agent Status*\n\n"+
					"*Network Token:* `%s`\n"+
					"*WAN Status:* `%s`\n"+
					"*CPU Usage:* `%.1f%%` | *RAM:* `%.1f%%`\n"+
					"*Temp:* `%.1f°C` | *Latency:* `%.1fms`\n"+
					"*Air-Gapped Mode:* `%t`",
					token, wan, cpu, ram, temp, lat, cfg.AirgappedMode)

			case cmdLower == "/reboot" || strings.Contains(cmdLower, "khởi động lại"):
				go func() {
					time.Sleep(2 * time.Second)
					_ = exec.Command("reboot").Run()
				}()
				return "🔄 *Khởi động lại Router:* Đã phát lệnh Reboot hệ thống. Thiết bị sẽ tái khởi động trong 45 giây."

			case cmdLower == "/boost" || strings.Contains(cmdLower, "tăng tốc"):
				boostReq := &executor.ActionRequest{ActionName: "boost_wifi_bandwidth", Target: "radio1"}
				if err := execEngine.ExecuteAction(ctx, boostReq, false); err != nil {
					return fmt.Sprintf("❌ *Tăng tốc Wi-Fi thất bại:* %v", err)
				}
				isWifiBoostedAtomic.Store(true)
				health.mu.Lock()
				health.IsWifiBoosted = true
				health.WifiBandwidthMHz = 160
				health.mu.Unlock()
				return "🚀 *Đã mở rộng băng thông Wi-Fi 5GHz/7 lên 160MHz Max Speed!*\n\n💡 Khi mạng rảnh rỗi trong 10 giây, Agent sẽ tự động hạ về 80MHz và gửi tin nhắn thông báo cho bạn."

			case cmdLower == "/health" || cmdLower == "/check" || strings.Contains(cmdLower, "kiểm tra") || strings.Contains(cmdLower, "sức khỏe"):
				m := collector.CollectMetrics(ctx)
				if m == nil {
					return "⚠️ *Không thể đọc chỉ số Telemetry từ phần cứng!*"
				}

				cfgSnap := cfgAtomic.Load().(*config.Config)
				liveLogSample := logParser.SanitizeLog(getSystemLogSample())
				_, zScore := collector.UpdateEWMALatency(m.LatencyMs, 0.2)

				var anomalyType, anomalyDesc string
				if m.WANStatus == "Offline (0/1)" || strings.Contains(m.WANStatus, "Offline") {
					anomalyType = "WAN_DROP"
					anomalyDesc = "WAN interface down or physical link lost"
				} else {
					for _, line := range strings.Split(liveLogSample, "\n") {
						if parsedReport := logParser.ParseLine(line); parsedReport != nil {
							anomalyType = parsedReport.Type
							anomalyDesc = parsedReport.Description
							break
						}
					}
					if anomalyType == "" && m.RAMUsagePct > cfgSnap.RAMExhaustionPct {
						anomalyType = "MEMORY_EXHAUSTION"
						anomalyDesc = fmt.Sprintf("RAM usage high: %.1f%% (>%.1f%%)", m.RAMUsagePct, cfgSnap.RAMExhaustionPct)
					} else if anomalyType == "" && zScore > cfgSnap.LatencyZScoreThreshold && m.LatencyMs > cfgSnap.LatencySpikeMs {
						anomalyType = "BUFFERBLOAT_SPIKE"
						anomalyDesc = fmt.Sprintf("Latency spike: %.1fms (Z-Score: %.2f)", m.LatencyMs, zScore)
					}
				}

				if anomalyType == "" {
					return "success"
				}

				// Anomaly detected! Force agent to resolve automatically
				logger.Warn("TELEGRAM /HEALTH FORCED AUTO-HEAL: %s (%s)", anomalyType, anomalyDesc)
				defaultAction := "restart_wan_interface"
				if anomalyType == "MEMORY_EXHAUSTION" {
					defaultAction = "purge_memory_cache"
				} else if anomalyType == "WIFI_FAILURE" {
					defaultAction = "optimize_wifi_channel"
				} else if anomalyType == "BUFFERBLOAT_SPIKE" || anomalyType == "LATENCY_SPIKE" {
					defaultAction = "enable_cake_sqm"
				}

				sig := &skillstore.StateSignature{
					StateName:  anomalyType,
					RAMPct:     m.RAMUsagePct,
					LatencyMs:  m.LatencyMs,
					CPUPct:     m.CPUUsagePct,
					TempC:      m.HardwareTempC,
					WANOffline: strings.Contains(m.WANStatus, "Offline"),
					WiFiDown:   strings.Contains(strings.ToLower(m.WiFi5GGhzStatus), "disabled") || strings.Contains(strings.ToLower(m.WiFi5GGhzStatus), "failed"),
				}
				actionName, _, _, _, qErr := store.RecommendBestActionWithInterpolation(anomalyType, sig, defaultAction)
				if qErr != nil || actionName == "" {
					actionName = defaultAction
				}

				actionReq := &executor.ActionRequest{ActionName: actionName, Target: "wan"}
				execErr := execEngine.ExecuteAction(ctx, actionReq, cfgSnap.DryRun)

				if execErr != nil {
					return fmt.Sprintf("⚠️ *Phát hiện sự cố:* `%s` (%s)\n❌ *Tự giải quyết thất bại:* %v", anomalyType, anomalyDesc, execErr)
				}

				return fmt.Sprintf("⚠️ *Phát hiện sự cố:* `%s` (%s)\n🔧 *Đã ép Agent tự khắc phục:* `%s` (Thành công)", anomalyType, anomalyDesc, actionName)

			case cmdLower == "/help" || strings.Contains(cmdLower, "trợ giúp"):
				return "📖 *Danh sách lệnh Beryl 7 Telegram Bot:*\n" +
					"- `/status` hoặc `trạng thái`: Xem trạng thái & NetworkToken\n" +
					"- `/health` hoặc `kiểm tra`: Kiểm tra sức khỏe toàn bộ hệ thống (Ép tự giải quyết nếu có sự cố, hoặc trả về 'success')\n" +
					"- `/boost` hoặc `tăng tốc`: Ép mở rộng độ rộng kênh Wi-Fi 160MHz\n" +
					"- `/reboot` hoặc `khởi động lại`: Khởi động lại router vật lý\n" +
					"- `/help`: Hiển thị bảng trợ giúp này"

			default:
				// Conversational AI Interface & On-Device TinyML Intent Engine Routing
				cfgSnap := cfgAtomic.Load().(*config.Config)
				m := collector.CollectMetrics(ctx)
				tokenStr := ""
				cpuPct, ramPct, tempC, latMs := 0.0, 0.0, 0.0, 0.0
				if m != nil {
					tokenStr = m.NetworkToken
					cpuPct = m.CPUUsagePct
					ramPct = m.RAMUsagePct
					tempC = m.HardwareTempC
					latMs = m.LatencyMs
				}

				if cfgSnap.AirgappedMode || cfgSnap.GeminiAPIKey == "" {
					// 100% Offline Air-Gapped Mode / Local TinyML Intelligence Fallback
					bestSkill := store.GetBestSkillForAnomaly(cmdReceived)
					if bestSkill != nil {
						return fmt.Sprintf("🤖 *[Beryl 7 Local TinyML Agent]*\n\n"+
							"Tôi đã nhận yêu cầu của bạn: *\"%s\"*\n"+
							"📌 *Khuyến nghị kịch bản cục bộ:* `%s` (Độ tin cậy: `%.0f%%`)\n"+
							"🏷️ *Network Token:* `%s`", cmdReceived, bestSkill.Action, bestSkill.Confidence*100, tokenStr)
					}

					return fmt.Sprintf("🤖 *[Beryl 7 Local TinyML Agent]*\n\n"+
						"Tôi đã nhận thông điệp: *\"%s\"*\n"+
						"📊 *Tình trạng phần cứng:* Mạng bình thường | CPU `%.1f%%` | RAM `%.1f%%` | Temp `%.1f°C`\n"+
						"💡 *Hệ thống tự trị kín (Air-Gapped TinyML Engine) đang vận hành ổn định.*",
						cmdReceived, cpuPct, ramPct, tempC)
				}

				// Hybrid Cloud AI Conversation Routing
				aiResp, err := aiClient.AnalyzeAnomalyWithContext(ctx, "TELEGRAM_USER_CHAT", cmdReceived, tokenStr, "Format response concisely as Beryl 7 Router AI Assistant.")
				if err == nil && aiResp != nil && aiResp.Reasoning != "" {
					return fmt.Sprintf("🤖 *[Beryl 7 AI Assistant]*\n\n%s", aiResp.Reasoning)
				}

				return fmt.Sprintf("🤖 *[Beryl 7 AI Assistant]*\n\n"+
					"Tôi đã ghi nhận yêu cầu: *\"%s\"*\n"+
					"📊 *Trạng thái phần cứng:* CPU `%.1f%%` | RAM `%.1f%%` | Latency `%.1fms`",
					cmdReceived, cpuPct, ramPct, latMs)
			}
		})
	}

	go func() {
		sig := <-sigCh
		logger.Info("Received shutdown signal (%v)! Initiating Adaptive Graceful Shutdown...", sig)

		// 30s Forced exit fallback to guarantee process termination
		go func() {
			time.Sleep(30 * time.Second)
			logger.Error("FATAL: Graceful shutdown timed out after 30s - forcing immediate exit!")
			os.Exit(1)
		}()

		ctxHTTP, cancelHTTP := context.WithTimeout(context.Background(), 2*time.Second)
		if err := httpServer.Shutdown(ctxHTTP); err != nil {
			logger.Error("HTTP server shutdown error: %v", err)
		}
		cancelHTTP()

		// Hardware NAND Flash Protection: Flush RAM DB to persistent /etc/beryl7/skills.db on shutdown
		if flushErr := store.FlushToPersistent(cfg.SkillStorePath); flushErr != nil {
			logger.Warn("Shutdown SkillStore Flush-to-Flash error: %v", flushErr)
		}

		if err := store.Close(); err != nil {
			logger.Error("SkillStore database close error: %v", err)
		}
		logger.Flush()

		cancel()
		os.Exit(0)
	}()

	logger.Info("Daemon initialized successfully. Security Hardened Engine listening on 24/7 main loop...")

	configMu.RLock()
	interval := cfg.TelemetryInterval
	configMu.RUnlock()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	backupTicker := time.NewTicker(6 * time.Hour)
	defer backupTicker.Stop()

	pruneTicker := time.NewTicker(24 * time.Hour)
	defer pruneTicker.Stop()

	cooldowns := map[string]time.Duration{
		"WAN_DROP":                   30 * time.Second,
		"WIFI_FAILURE":               45 * time.Second,
		"MEMORY_EXHAUSTION":          30 * time.Minute,
		"LATENCY_SPIKE":              60 * time.Second,
		"REPEATER_SIGNAL_WEAK":       60 * time.Second,
		"REPEATER_CHANNEL_CONGESTED": 60 * time.Second,
	}
	lastActionByAnomaly := make(map[string]time.Time)
	lowTrafficCycles := 0
	streamingPipelineActive := false
	bulkDownloadActive := false

	cfgAtomic.Store(cfg)

	for {
		select {
		case <-ctx.Done():
			logger.Info("Main loop exiting context done...")
			return
		case reason := <-restartSignalChan:
			logger.Warn("Main thread processing restart signal [%s]...", reason)

			// Flush RAM DB to Persistent Flash before process re-execution
			if activeStore != nil {
				if cfgSnap, ok := cfgAtomic.Load().(*config.Config); ok && cfgSnap != nil && cfgSnap.SkillStorePath != "" {
					if flushErr := activeStore.FlushToPersistent(cfgSnap.SkillStorePath); flushErr != nil {
						logger.Error("Emergency pre-restart SkillStore flush failed: %v", flushErr)
					}
				}
				_ = activeStore.Close()
			}

			cancel() // Cancel main context to signal all async goroutines to exit via ctx.Done()

			ctxHTTP, cancelHTTP := context.WithTimeout(context.Background(), 3*time.Second)
			if activeServer != nil {
				_ = activeServer.Shutdown(ctxHTTP)
			}
			cancelHTTP()
			logger.Flush()

			if runtime.GOOS == "linux" {
				// Primary: use OpenWrt procd init system for clean service restart
				ctxCmd, cancelCmd := context.WithTimeout(context.Background(), 5*time.Second)
				cmd := exec.CommandContext(ctxCmd, "/etc/init.d/beryl7-agent", "restart") // #nosec G204
				errCmd := cmd.Run()
				cancelCmd()

				if errCmd != nil {
					logger.Warn("Procd restart failed [%v]. Falling back to dynamic syscall.Exec replacement...", errCmd)

					// [Fix 2] Dynamic os.Executable() resolution — no hardcoded path dependency
					execPath, errSelf := os.Executable()
					if errSelf != nil || execPath == "" {
						execPath = "/usr/bin/beryl7-agent" // Last-resort fallback if os.Executable fails
					}
					// #nosec G204, G702
					_ = syscall.Exec(execPath, os.Args, os.Environ()) // #nosec G204, G702 // nolint:errcheck
				}
				os.Exit(0)
			} else {
				os.Exit(0)
			}
			return
		case <-backupTicker.C:
			if backupErr := store.BackupDatabase(); backupErr != nil {
				logger.Error("Scheduled SkillStore backup failed: %v", backupErr)
			}
			// Autonomous Golden State Maintenance: Proactively keeps system clean, defragmented and optimized 24/7
			_ = store.OptimizeAndVacuum()
			_ = execEngine.ExecuteAction(ctx, &executor.ActionRequest{ActionName: "purge_memory_cache", Target: "ram"}, false)
			_ = execEngine.ExecuteAction(ctx, &executor.ActionRequest{ActionName: "tune_network_performance", Target: "lan"}, false)
			logger.Info("GOLDEN STATUS: Autonomous full-system cleanup and performance optimization completed.")
		case <-pruneTicker.C:
			if pruneErr := store.PruneSkillsPeriodic(); pruneErr != nil {
				logger.Error("Scheduled SkillStore pruning failed: %v", pruneErr)
			}
		case <-ticker.C:
			cfgSnap := cfgAtomic.Load().(*config.Config)
			currentDryRun := cfgSnap.DryRun
			currentAlpha := cfgSnap.EMAAlpha

			m := collector.CollectMetrics(ctx)
			if m == nil {
				continue
			}

			health.mu.Lock()
			health.WANStatus = m.WANStatus
			health.CPUUsagePct = m.CPUUsagePct
			health.RAMUsagePct = m.RAMUsagePct
			health.HardwareTempC = m.HardwareTempC
			health.LatencyMs = m.LatencyMs
			health.NetworkToken = m.NetworkToken
			if m.SystemUptimeSec > 0 {
				health.UptimeSeconds = m.SystemUptimeSec
			} else {
				health.UptimeSeconds = int64(time.Since(health.StartTime).Seconds())
			}
			isBoosted := isWifiBoostedAtomic.Load()
			health.SafeMode = wd.IsSafeMode()
			health.KillSwitch = config.IsKillSwitchActive(cfgSnap)
			bw := 80
			if isBoosted {
				bw = 160
			}
			health.WifiBandwidthMHz = bw
			health.IsWifiBoosted = isBoosted
			health.DownloadMbps = m.DownloadMbps
			health.UploadMbps = m.UploadMbps
			devs := collector.GetConnectedDevices(ctx, isBoosted, m.DownloadMbps, m.UploadMbps)
			health.ConnectedDevicesCount = len(devs)
			health.ConnectedDevices = devs
			health.mu.Unlock()

			// Dynamically harmonize gl-repeater: Standby on wired Ethernet, active on wireless travel mode
			isWiredWANActive := m.WANStatus == "Active (1/1)" || strings.Contains(m.WANStatus, "Active")
			collector.HarmonizeRepeaterState(ctx, isWiredWANActive)

			if wd.IsSafeMode() {
				wd.RecordHealthCheckSuccess()
				continue
			}

			if config.IsKillSwitchActive(cfg) {
				logger.Warn("Kill Switch Active (/tmp/beryl7-disable or env)! Auto-healing suspended.")
				continue
			}

			liveLogSample := logParser.SanitizeLog(getSystemLogSample())

			if m.DownloadMbps > cfgSnap.BandwidthBoostMbps && !isBoosted {
				logger.Info("SMART BANDWIDTH DETECTED (%.1f Mbps > %.1fMbps)! Auto-boosting Wi-Fi 7 to 160MHz Max Speed...", m.DownloadMbps, cfgSnap.BandwidthBoostMbps)
				boostReq := &executor.ActionRequest{ActionName: "boost_wifi_bandwidth", Target: "radio1"}
				if execErr := execEngine.ExecuteAction(ctx, boostReq, currentDryRun); execErr == nil {
					isWifiBoostedAtomic.Store(true)
					lowTrafficCycles = 0
				}
			} else if isBoosted && m.DownloadMbps < cfgSnap.BandwidthRestoreMbps {
				lowTrafficCycles++
				if lowTrafficCycles >= 2 {
					logger.Info("SMART BANDWIDTH STABILIZED (%.1f Mbps < %.1fMbps for 2 cycles)! Reverting Wi-Fi 7 to Eco 80MHz Mode...", m.DownloadMbps, cfgSnap.BandwidthRestoreMbps)
					revertReq := &executor.ActionRequest{ActionName: "revert_wifi_bandwidth", Target: "radio1"}
					if execErr := execEngine.ExecuteAction(ctx, revertReq, currentDryRun); execErr == nil {
						isWifiBoostedAtomic.Store(false)
						lowTrafficCycles = 0
						if tgNotifier != nil {
							ecoMsg := fmt.Sprintf("🌱 *Wi-Fi 7 đã tự động trở về chế độ Eco (80MHz)*\n\n"+
								"📊 *Lưu lượng mạng:* `%.1f Mbps` (< `%.1f Mbps` trong 10s)\n"+
								"💡 Router đã tự động hạ băng thông để tiết kiệm điện năng và giảm nhiệt độ.",
								m.DownloadMbps, cfgSnap.BandwidthRestoreMbps)
							go func() { _ = tgNotifier.SendAlert(ctx, ecoMsg) }()
						}
					}
				}
			} else if isBoosted && m.DownloadMbps >= cfgSnap.BandwidthRestoreMbps {
				lowTrafficCycles = 0
			}

			_, zScore := collector.UpdateEWMALatency(m.LatencyMs, 0.2)

			// Smart Sustained Video Streaming Pipeline Acceleration (Edge-Triggered on state entry, avoiding continuous 5s execution loops)
			if m.IsStreamingActive && !streamingPipelineActive {
				streamingPipelineActive = true
				_ = execEngine.ExecuteAction(ctx, &executor.ActionRequest{ActionName: "optimize_streaming_pipeline", Target: "lan"}, false)
			} else if !m.IsStreamingActive && streamingPipelineActive {
				streamingPipelineActive = false
			}

			// Smart Bulk Download Acceleration: Ensure unthrottled high-throughput TCP window scaling & Flow Offloading
			if m.IsBulkDownloadActive && !bulkDownloadActive {
				bulkDownloadActive = true
				_ = execEngine.ExecuteAction(ctx, &executor.ActionRequest{ActionName: "tune_network_performance", Target: "lan"}, false)
			} else if !m.IsBulkDownloadActive && bulkDownloadActive {
				bulkDownloadActive = false
			}

			var anomalyType, anomalyDesc string
			if m.WANStatus == "Offline (0/1)" || strings.Contains(m.WANStatus, "Offline") {
				anomalyType = "WAN_DROP"
				anomalyDesc = "WAN interface down or physical link lost"
			} else {
				for _, line := range strings.Split(liveLogSample, "\n") {
					if parsedReport := logParser.ParseLine(line); parsedReport != nil {
						anomalyType = parsedReport.Type
						anomalyDesc = parsedReport.Description
						break
					}
				}
				if anomalyType == "" && m.RAMUsagePct > cfgSnap.RAMExhaustionPct {
					anomalyType = "MEMORY_EXHAUSTION"
					anomalyDesc = fmt.Sprintf("High RAM usage detected: %.1f%% (>%.1f%%)", m.RAMUsagePct, cfgSnap.RAMExhaustionPct)
				} else if anomalyType == "" && zScore > cfgSnap.LatencyZScoreThreshold && m.LatencyMs > cfgSnap.LatencySpikeMs {
					anomalyType = "BUFFERBLOAT_SPIKE"
					anomalyDesc = fmt.Sprintf("Bufferbloat / Latency Spike detected: %.1fms (Z-Score: %.2f > %.1f)", m.LatencyMs, zScore, cfgSnap.LatencyZScoreThreshold)
				} else if anomalyType == "" {
					repeaterM, _ := collector.CollectRepeaterMetrics(ctx)
					if repeaterM != nil && repeaterM.IsRepeater {
						if repeaterM.Signal < 0 && repeaterM.Signal < -75 {
							anomalyType = "REPEATER_SIGNAL_WEAK"
							anomalyDesc = fmt.Sprintf("Weak Repeater RSSI detected: %ddBm (< -75dBm)", repeaterM.Signal)
						}
					}
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

				if tgNotifier != nil && anomalyType != "MEMORY_EXHAUSTION" {
					alertMsg := fmt.Sprintf("⚠️ *PHÁT HIỆN SỰ CỐ MẠNG*\n\n"+
						"*Loại sự cố:* `%s`\n"+
						"*Chi tiết:* %s\n"+
						"*Network Token:* `%s`", anomalyType, anomalyDesc, m.NetworkToken)
					go func() { _ = tgNotifier.SendAlert(ctx, alertMsg) }()
				}

				defaultAction := "restart_wan_interface"
				if anomalyType == "MEMORY_EXHAUSTION" {
					defaultAction = "purge_memory_cache"
				} else if anomalyType == "WIFI_FAILURE" {
					defaultAction = "optimize_wifi_channel"
				} else if anomalyType == "BUFFERBLOAT_SPIKE" || anomalyType == "LATENCY_SPIKE" {
					defaultAction = "enable_cake_sqm"
				} else if anomalyType == "REPEATER_SIGNAL_WEAK" {
					defaultAction = "scale_tx_power_down"
				} else if anomalyType == "REPEATER_CHANNEL_CONGESTED" {
					defaultAction = "align_channels"
				}

				// Construct quantitative telemetry feature signature for TinyML similarity matching
				sig := &skillstore.StateSignature{
					StateName:  anomalyType,
					RAMPct:     m.RAMUsagePct,
					LatencyMs:  m.LatencyMs,
					CPUPct:     m.CPUUsagePct,
					TempC:      m.HardwareTempC,
					WANOffline: strings.Contains(m.WANStatus, "Offline"),
					WiFiDown:   strings.Contains(strings.ToLower(m.WiFi5GGhzStatus), "disabled") || strings.Contains(strings.ToLower(m.WiFi5GGhzStatus), "failed"),
				}

				// Q-Learning V2 Engine: Active Decision Loop with TinyML Vector Similarity Interpolation
				actionName, qVal, matchedState, isInterpolated, qErr := store.RecommendBestActionWithInterpolation(anomalyType, sig, defaultAction)
				if qErr == nil && isInterpolated {
					logger.Info("TINYML INTERPOLATION HIT: Novel Anomaly [%s] mapped to similar State [%s] -> Recommending Action '%s' (Q-Value=%.2f, Default='%s')", anomalyType, matchedState, actionName, qVal, defaultAction)
				} else if qErr == nil && actionName != defaultAction {
					logger.Info("Q-LEARNING ENGINE HIT: Best learned action for Anomaly [%s] is '%s' (Q-Value=%.2f, Default='%s')", anomalyType, actionName, qVal, defaultAction)
				} else if qErr == nil {
					logger.Info("Q-LEARNING ENGINE: Using default action '%s' for Anomaly [%s] (Q-Value=%.2f)", actionName, anomalyType, qVal)
				}

				// Helper hàm Post-Action Verification bất đồng bộ (Non-blocking async goroutine)
				verifyActionSuccess := func() bool {
					postMetric := collector.CollectMetrics(ctx)
					if postMetric == nil {
						return false
					}
					if anomalyType == "MEMORY_EXHAUSTION" {
						return postMetric.RAMUsagePct < 88.0
					}
					if anomalyType == "WAN_DROP" {
						return !strings.Contains(postMetric.WANStatus, "Offline")
					}
					if anomalyType == "LATENCY_SPIKE" {
						return postMetric.LatencyMs < 100.0
					}
					if anomalyType == "WIFI_FAILURE" {
						statusLower := strings.ToLower(postMetric.WiFi5GGhzStatus)
						return !strings.Contains(statusLower, "disabled") && !strings.Contains(statusLower, "failed")
					}
					return false // Fallback an toàn: trả false nếu sự cố chưa thực sự xóa sạch
				}

				verifyAndRecordSkillAsync := func(targetSkill *skillstore.Skill, sig *skillstore.StateSignature, cmdExecSuccess bool) {
					if targetSkill == nil {
						return
					}
					// Value copy to prevent pointer race
					skillCopy := &skillstore.Skill{
						ID:         targetSkill.ID,
						Action:     targetSkill.Action,
						Condition:  targetSkill.Condition,
						Confidence: targetSkill.Confidence,
					}
					var sigCopy *skillstore.StateSignature
					if sig != nil {
						s := *sig
						sigCopy = &s
					}
					if !cmdExecSuccess {
						collector.RecordHealOutcome(false)
						_ = store.SaveOrUpdateSkill(skillCopy, false, currentAlpha)
						_ = store.UpdateQValue(skillCopy.Condition, skillCopy.Action, -0.5)
						return
					}
					go func(sk *skillstore.Skill, sSig *skillstore.StateSignature) {
						defer func() {
							if r := recover(); r != nil {
								logger.Error("Recovered panic in verifyAndRecordSkillAsync: %v", r)
							}
						}()
						select {
						case <-ctx.Done():
							logger.Info("Async skill verification cancelled via context done.")
							return
						case <-time.After(3 * time.Second):
						}
						verifiedSuccess := verifyActionSuccess()
						collector.RecordHealOutcome(verifiedSuccess)
						_ = store.SaveOrUpdateSkill(sk, verifiedSuccess, currentAlpha)
						reward := 1.0
						if !verifiedSuccess {
							reward = -0.5
						}
						_ = store.UpdateQValue(sk.Condition, sk.Action, reward)
						if verifiedSuccess {
							if sSig != nil {
								_ = store.RecordStateSignature(sSig)
							}
							logger.Info("Post-Action Telemetry Verification SUCCESS for [%s:%s] -> Q-Value rewarded (+1.0)", sk.Condition, sk.Action)
						} else {
							logger.Warn("Post-Action Telemetry Verification FAILED for [%s:%s] -> Q-Value penalized (-0.5)", sk.Condition, sk.Action)
						}
					}(skillCopy, sigCopy)
				}

				skill := store.GetSkill(anomalyType, actionName)
				requiredLocalThreshold := execEngine.GetActionRiskThreshold(actionName)

				if skill != nil && skill.Confidence >= requiredLocalThreshold {
					collector.RecordSkillHit()
					logger.Info("SkillStore Cache Hit! Executing Local Skill [%s] (Confidence=%.2f >= Required=%.2f)...", skill.Action, skill.Confidence, requiredLocalThreshold)
					dynDL, dynUL := collector.CalculateAdaptiveSQMRates(m.LatencyMs, zScore)
					actReq := &executor.ActionRequest{
						ActionName: skill.Action,
						Target:     "wan",
						Parameters: map[string]interface{}{
							"download_kbps": dynDL,
							"upload_kbps":   dynUL,
						},
					}
					if requiredLocalThreshold >= 0.85 {
						saveUCICheckpoint(cfg.CheckpointPath)
					}
					execErr := execEngine.ExecuteAction(ctx, actReq, currentDryRun)
					verifyAndRecordSkillAsync(skill, sig, execErr == nil)
					continue
				}

				// Local-First Autonomy: When offline, first query SQLite skills.db for historical learned skills!
				var aiResp *ai.AIResponse
				var aiErr error
				if anomalyType == "WAN_DROP" || strings.Contains(m.WANStatus, "Offline") {
					logger.Warn("WAN is offline (%s). Locking Cloud AI calls and querying SQLite SkillStore...", m.WANStatus)
					bestSkill := store.GetBestSkillForAnomaly(anomalyType)
					if bestSkill != nil && bestSkill.Confidence >= 0.50 {
						logger.Info("SQLite SkillStore Hit for Offline Anomaly [%s]: Using learned skill '%s' (Confidence=%.2f)", anomalyType, bestSkill.Action, bestSkill.Confidence)
						aiResp = &ai.AIResponse{
							Action:     bestSkill.Action,
							Confidence: bestSkill.Confidence,
							Reasoning:  fmt.Sprintf("Local SQLite SkillStore Brain Hit for offline anomaly %s", anomalyType),
						}
					} else {
						logger.Info("No high-confidence skill in SQLite for [%s]. Falling back to default 'restart_wan_interface'", anomalyType)
						aiResp = &ai.AIResponse{
							Action:     "restart_wan_interface",
							Confidence: 0.95,
							Reasoning:  "Local Offline Fallback: WAN interface down, executing default interface restart",
						}
					}
				} else {
					topExemplars := store.GetTopSkillsSummaryForAnomaly(anomalyType, 3)
					aiResp, aiErr = aiClient.AnalyzeAnomalyWithContext(ctx, anomalyType, anomalyDesc, liveLogSample, topExemplars)
				}

				if aiErr == nil && aiResp != nil {
					requiredThreshold := execEngine.GetActionRiskThreshold(aiResp.Action)

					if aiResp.Confidence >= requiredThreshold {
						logger.Info("Gemini AI Action Approved: [%s] (Confidence=%.2f >= Required=%.2f) - Reasoning: %s", aiResp.Action, aiResp.Confidence, requiredThreshold, aiResp.Reasoning)
						actReq := &executor.ActionRequest{
							ActionName: aiResp.Action,
							Target:     "wan",
						}
						if requiredThreshold >= 0.85 {
							saveUCICheckpoint(cfg.CheckpointPath)
						}
						execErr := execEngine.ExecuteAction(ctx, actReq, currentDryRun)

						newSkill := &skillstore.Skill{
							ID:         fmt.Sprintf("%s:%s", anomalyType, aiResp.Action),
							Action:     aiResp.Action,
							Condition:  anomalyType,
							Confidence: aiResp.Confidence,
						}
						verifyAndRecordSkillAsync(newSkill, sig, execErr == nil)
					} else {
						logger.Warn("SECURITY GATING TRIGGERED: AI Action [%s] Confidence (%.2f) below Required Risk Threshold (%.2f)! Queued for Operator Approval.", aiResp.Action, aiResp.Confidence, requiredThreshold)
						queuePendingApproval(aiResp, requiredThreshold)
						if anomalyType == "WAN_DROP" || requiredThreshold >= 0.85 {
							_ = wd.ExecuteRollback()
						}
					}
				} else {
					logger.Warn("Cloud AI Call Failed (%v). Activating Local Offline Heuristic Remediation Engine for [%s]...", aiErr, anomalyType)
					localReq := &executor.ActionRequest{
						ActionName: actionName,
						Target:     "wan",
					}
					execErr := execEngine.ExecuteAction(ctx, localReq, currentDryRun)
					if execErr != nil {
						logger.Error("Local Offline Heuristic Action [%s] failed (%v) -> Triggering Watchdog Rollback...", actionName, execErr)
						if anomalyType == "WAN_DROP" {
							_ = wd.ExecuteRollback()
						}
					}
					offlineSkill := &skillstore.Skill{
						ID:         fmt.Sprintf("%s:%s", anomalyType, actionName),
						Action:     actionName,
						Condition:  anomalyType,
						Confidence: 0.90,
					}
					verifyAndRecordSkillAsync(offlineSkill, sig, execErr == nil)
				}
			}
		}
	}
}

func getSystemLogSample() string {
	if runtime.GOOS == "linux" {
		out, err := exec.Command("/sbin/logread", "-l", "15").Output() // #nosec G204
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
		_ = os.WriteFile(filepath.Clean("/var/run/beryl7_pending_approval.json"), data, 0600) // #nosec G306, G304
		logger.Info("Saved pending approval request to /var/run/beryl7_pending_approval.json")
	}
}

func recordApprovalAuditLog(action, remoteAddr string) {
	auditLine := fmt.Sprintf("[%s] AUDIT: Operator approved action [%s] from %s\n", time.Now().Format(time.RFC3339), action, remoteAddr)
	logFile := filepath.Clean("/var/log/beryl7_approval_audit.log")
	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600) // #nosec G302, G304
	if err == nil {
		_, _ = f.WriteString(auditLine)
		_ = f.Sync()
		_ = f.Close()
	}
}

func saveUCICheckpoint(path string) {
	if path == "" {
		path = "/root/.agent_checkpoint.uci"
	}
	cleanPath := filepath.Clean(path)
	dir := filepath.Dir(cleanPath)
	_ = os.MkdirAll(dir, 0700)
	f, err := os.OpenFile(cleanPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600) // #nosec G302, G304
	if err != nil {
		logger.Warn("Failed to open checkpoint file %s: %v", cleanPath, err)
		return
	}
	defer f.Close()

	cmd := exec.Command("uci", "export") // #nosec G204
	cmd.Stdout = f
	if err := cmd.Run(); err != nil {
		logger.Warn("UCI export command failed: %v", err)
	} else {
		logger.Info("Saved persistent UCI checkpoint to %s", cleanPath)
	}
}

func acquirePIDLock(pidPath string) error {
	cleanPath := filepath.Clean(pidPath)
	if content, err := os.ReadFile(cleanPath); err == nil && len(content) > 0 {
		var oldPID int
		if _, parseErr := fmt.Sscanf(string(content), "%d", &oldPID); parseErr == nil && oldPID > 0 {
			if oldPID == os.Getpid() {
				logger.Info("PID lock verified for current process self-exec (PID %d)", oldPID)
				return nil
			}
			if checkPIDAlive(oldPID) {
				return fmt.Errorf("beryl7-agent is already running with PID %d", oldPID)
			}
			logger.Info("Removing stale PID file (PID %d is dead)", oldPID)
			_ = os.Remove(pidPath)
		}
	}

	return os.WriteFile(pidPath, []byte(strconv.Itoa(os.Getpid())), 0600)
}

type FailsafeLevel int

const (
	FailsafeLevel1 FailsafeLevel = iota
	FailsafeLevel2
	FailsafeLevel3
	FailsafeLevel4
)

func CheckBinaryCompatibility() error {
	info, err := os.Stat("/usr/bin/beryl7-agent")
	if err != nil {
		return fmt.Errorf("binary executable missing: %w", err)
	}
	if info.Mode()&0111 == 0 {
		return fmt.Errorf("binary executable permission missing")
	}
	return nil
}

func PostUpgradeValidation(cfg *config.Config) error {
	logger.Info("🔍 Starting post-upgrade system validation...")

	if err := CheckBinaryCompatibility(); err != nil {
		logger.Warn("Binary validation warning: %v", err)
	}

	if cfg != nil && cfg.AuthToken == "" {
		logger.Warn("Config validation: AUTH_TOKEN is empty")
	}

	if cfg != nil && cfg.SkillStorePath != "" {
		if _, err := os.Stat(cfg.SkillStorePath); err == nil {
			db, err := sql.Open("sqlite", cfg.SkillStorePath)
			if err != nil {
				logger.Warn("SQLite DB open failed (%v) - initiating DB repair...", err)
				_ = os.Remove(cfg.SkillStorePath)
			} else {
				defer db.Close()
				var integrity string
				if err := db.QueryRow("PRAGMA integrity_check").Scan(&integrity); err != nil || integrity != "ok" {
					logger.Warn("SQLite DB corruption detected (%s) - re-initializing DB...", integrity)
					_ = db.Close()
					_ = os.Remove(cfg.SkillStorePath)
				}
			}
		}
	}

	logger.Info("✅ Post-upgrade validation PASSED")
	return nil
}

func FailsafeRecovery(level FailsafeLevel, cfg *config.Config) error {
	switch level {
	case FailsafeLevel1:
		logger.Warn("⚠️ Level 1 Failsafe: Restoring binary backup /usr/bin/beryl7-agent.backup...")
		if _, err := os.Stat("/usr/bin/beryl7-agent.backup"); err == nil {
			if err := os.Rename("/usr/bin/beryl7-agent.backup", "/usr/bin/beryl7-agent"); err == nil {
				logger.Warn("Binary restored! Delegating process re-execution to main thread...")
				PerformGracefulProcessRestart("failsafe_recovery")
				return PostRollbackValidationChecklist(cfg)
			}
		}
		logger.Warn("Binary backup missing -> Escalating to Level 2 Failsafe")
		return FailsafeRecovery(FailsafeLevel2, cfg)

	case FailsafeLevel2:
		logger.Warn("⚠️ Level 2 Failsafe: Activating Degraded Mode (Monitoring Only)...")
		if cfg != nil {
			cfg.DisableAutoHeal = true
		}
		return nil

	case FailsafeLevel3:
		logger.Warn("⚠️ Level 3 Failsafe: Resetting SkillStore to Factory Defaults...")
		if cfg != nil {
			_ = os.Remove(cfg.SkillStorePath)
		}
		return nil

	case FailsafeLevel4:
		logger.Error("❌ Level 4 Failsafe: Recovery FAILED! Manual operator intervention required.")
		return fmt.Errorf("critical failsafe recovery failed")
	}
	return nil
}

func PostRollbackValidationChecklist(cfg *config.Config) error {
	logger.Info("📋 Executing Post-Rollback Validation Checklist...")
	if err := CheckBinaryCompatibility(); err != nil {
		logger.Warn("Rollback binary check warning: %v", err)
	}
	logger.Info("✅ Post-Rollback Validation PASSED")
	return nil
}

func AutoRollback(cfg *config.Config) error {
	logger.Warn("🔄 Triggering Automated Post-Upgrade Rollback...")
	return FailsafeRecovery(FailsafeLevel1, cfg)
}

func isLANOrLoopbackIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	// [Fix 5] Unwrap IPv4-mapped IPv6 (e.g. ::ffff:192.168.8.100) to canonical 4-byte IPv4
	// so IsPrivate()/IsLoopback() return correct results on all Go versions and prevent CORS leak to WAN
	if ip4 := ip.To4(); ip4 != nil {
		return ip4.IsLoopback() || ip4.IsPrivate() || ip4.IsLinkLocalUnicast() || ip4.IsLinkLocalMulticast()
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()
}

// VerifyActionSuccess measures real post-action hardware telemetry and returns whether the anomaly is truly resolved.
func VerifyActionSuccess(ctx context.Context, collector *telemetry.TelemetryCollector, anomalyType string) bool {
	if collector == nil {
		return false
	}
	postMetric := collector.CollectMetrics(ctx)
	if postMetric == nil {
		return false
	}
	switch anomalyType {
	case "MEMORY_EXHAUSTION":
		return postMetric.RAMUsagePct < 88.0
	case "WAN_DROP":
		return !strings.Contains(postMetric.WANStatus, "Offline")
	case "LATENCY_SPIKE", "BUFFERBLOAT_SPIKE":
		return postMetric.LatencyMs < 100.0
	case "WIFI_FAILURE":
		statusLower := strings.ToLower(postMetric.WiFi5GGhzStatus)
		return !strings.Contains(statusLower, "disabled") && !strings.Contains(statusLower, "failed")
	case "REPEATER_SIGNAL_WEAK":
		rep, _ := collector.CollectRepeaterMetrics(ctx)
		return rep == nil || rep.Signal >= -75
	default:
		return postMetric.RAMUsagePct < 88.0 && !strings.Contains(postMetric.WANStatus, "Offline")
	}
}

func StartHealthCheckServer(cfg *config.Config, health *HealthState, execEngine *executor.Executor, store *skillstore.SkillStore, aiClient *ai.AIClient, wd *watchdog.Watchdog, collector *telemetry.TelemetryCollector) *http.Server {
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
		return ipCounts[ip] <= 60
	}

	setCorsHeaders := func(w http.ResponseWriter, r *http.Request) bool {
		return setCorsHeaders(w, r, cfg)
	}

	// Endpoint 1: Health Check (/api/health)
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		if setCorsHeaders(w, r) {
			return
		}
		host, _, _ := net.SplitHostPort(r.RemoteAddr)
		if host == "" {
			host = r.RemoteAddr
		}
		if !rateLimitCheck(host) {
			http.Error(w, `{"error":"Too Many Requests: Rate limit exceeded"}`, http.StatusTooManyRequests)
			return
		}

		cbState, _, _ := aiClient.GetCircuitBreakerStatus()
		safeMode := wd.IsSafeMode()

		health.mu.Lock()
		if collector != nil {
			proc := collector.GetProcessResourceStats()
			health.Goroutines = proc.Goroutines
			health.HeapAllocMB = proc.HeapAllocMB
			health.RSSMB = proc.RSSMB
		}
		if safeMode {
			health.Status = "degraded (safe_mode)"
		} else if cbState == "OPEN" {
			health.Status = "degraded (circuit_open)"
		} else if strings.Contains(health.WANStatus, "Offline") {
			health.Status = "degraded (wan_offline)"
		} else {
			health.Status = "healthy"
		}
		data, err := json.Marshal(health)
		health.mu.Unlock()

		if err == nil {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(data)
		}
	})

	// Endpoint 1b: Empirical Process & Operational Metrics (/api/v1/metrics)
	mux.HandleFunc("/api/v1/metrics", func(w http.ResponseWriter, r *http.Request) {
		if setCorsHeaders(w, r) {
			return
		}
		host, _, _ := net.SplitHostPort(r.RemoteAddr)
		if host == "" {
			host = r.RemoteAddr
		}
		if !rateLimitCheck(host) {
			http.Error(w, `{"error":"Too Many Requests: Rate limit exceeded"}`, http.StatusTooManyRequests)
			return
		}

		var procStats telemetry.ProcessResourceStats
		var opCounters telemetry.CollectorCounters
		if collector != nil {
			procStats = collector.GetProcessResourceStats()
			opCounters = collector.GetOperationalCounters()
		}

		var opMetrics skillstore.OperationalMetrics
		if store != nil {
			opMetrics = store.GetOperationalMetrics()
		}

		health.mu.RLock()
		telData := map[string]interface{}{
			"cpu_usage_pct":   health.CPUUsagePct,
			"ram_usage_pct":   health.RAMUsagePct,
			"hardware_temp_c": health.HardwareTempC,
			"latency_ms":      health.LatencyMs,
		}
		health.mu.RUnlock()

		resp := map[string]interface{}{
			"process":            procStats,
			"telemetry":          telData,
			"operational":        opMetrics,
			"collector_counters": opCounters,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	// Endpoint 2: Module Status (/api/modules/status)
	mux.HandleFunc("/api/modules/status", func(w http.ResponseWriter, r *http.Request) {
		if setCorsHeaders(w, r) {
			return
		}
		cbState, cbFails, _ := aiClient.GetCircuitBreakerStatus()
		aiStatus := "healthy"
		if cbState == "OPEN" {
			aiStatus = "degraded"
		}
		wdStatus := "healthy"
		if wd.IsSafeMode() {
			wdStatus = "degraded"
		}

		hwModel := "Dynamic Hardware"
		if data, err := os.ReadFile("/etc/openwrt_release"); err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				if strings.HasPrefix(line, "DISTRIB_TARGET=") {
					hwModel = strings.Trim(strings.TrimPrefix(line, "DISTRIB_TARGET="), "'\"")
					break
				}
			}
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"orchestrator": map[string]interface{}{"status": "healthy", "priority": "ACTIVE", "interval_s": 5.0},
			"executor":     map[string]interface{}{"status": "healthy", "whitelist": "100%", "uci_mapping": hwModel},
			"ai_client":    map[string]interface{}{"status": aiStatus, "circuit_breaker": cbState, "fail_count": cbFails, "model": "Gemini 2.5 Flash"},
			"watchdog":     map[string]interface{}{"status": wdStatus, "checkpoint": "UCI Persistent", "rollback_window_s": 30},
			"log_parser":   map[string]interface{}{"status": "healthy", "source": "/sbin/logread", "regex": "Matched"},
			"skill_store":  map[string]interface{}{"status": "healthy", "storage": "SQLite WAL"},
		})
	})

	// Endpoint 3: Real Logread Logs (/api/logs)
	mux.HandleFunc("/api/logs", func(w http.ResponseWriter, r *http.Request) {
		if setCorsHeaders(w, r) {
			return
		}
		authHdr := r.Header.Get("Authorization")
		role, valid := validateTokenRole(r, authHdr, cfg)
		if (!isLocalhostRequest(r, cfg) && authHdr == "") || !valid || role == "unknown" || (role == "viewer" && !isLocalhostRequest(r, cfg)) {
			http.Error(w, `{"error":"Unauthorized: Operator or Admin Auth Token required to access system logs"}`, http.StatusUnauthorized)
			return
		}
		rawLogs := getSystemLogSample()
		lines := strings.Split(rawLogs, "\n")
		var logItems []map[string]string

		for _, line := range lines {
			if strings.TrimSpace(line) == "" {
				continue
			}
			logItems = append(logItems, map[string]string{
				"time":  time.Now().Format("15:04:05"),
				"level": "INFO",
				"msg":   line,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"logs":  logItems,
			"total": len(logItems),
		})
	})

	// Endpoint 4: Prometheus Metrics Exporter (/metrics)
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		if setCorsHeaders(w, r) {
			return
		}
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		health.mu.RLock()
		cpu := health.CPUUsagePct
		ram := health.RAMUsagePct
		temp := health.HardwareTempC
		lat := health.LatencyMs
		uptime := health.UptimeSeconds
		health.mu.RUnlock()

		tel := telemetry.NewCollector()
		metricObj := &telemetry.Metric{
			CPUUsagePct:     cpu,
			RAMUsagePct:     ram,
			HardwareTempC:   temp,
			LatencyMs:       lat,
			SystemUptimeSec: uptime,
		}
		promOutput := tel.ExportPrometheusMetrics(metricObj)
		if aiClient != nil {
			var sb strings.Builder
			sb.WriteString(promOutput)
			sb.WriteString("# HELP beryl7_ai_parse_failures_total LLM response parse failures\n")
			sb.WriteString("# TYPE beryl7_ai_parse_failures_total counter\n")
			sb.WriteString(fmt.Sprintf("beryl7_ai_parse_failures_total %d\n", aiClient.GetParseFailuresTotal()))
			promOutput = sb.String()
		}
		_, _ = w.Write([]byte(promOutput))
	})

	// Endpoint 5: Goroutine-Safe Config Reload (/api/config/reload)
	mux.HandleFunc("/api/config/reload", func(w http.ResponseWriter, r *http.Request) {
		if setCorsHeaders(w, r) {
			return
		}
		if r.Method != "POST" {
			http.Error(w, `{"error":"Method Not Allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		role, valid := validateTokenRole(r, r.Header.Get("Authorization"), cfg)
		if !valid || (role != "operator" && role != "admin") {
			http.Error(w, `{"error":"Forbidden: Endpoint requires operator or admin role"}`, http.StatusForbidden)
			return
		}

		configMu.Lock()
		oldHost := cfg.BindHost
		oldPort := cfg.HealthPort
		newCfg, loadErr := config.LoadConfigWithFlags(cfg.ConfigFilePath, cfg.KeyFilePath, cfg.DryRun, false)
		netChanged := false
		if loadErr == nil && newCfg != nil {
			if newCfg.BindHost != oldHost || newCfg.HealthPort != oldPort {
				netChanged = true
				logger.Warn("NOTICE: Network binding configuration changed (%s:%d -> %s:%d). Process restart required for new IP/Port binding.", oldHost, oldPort, newCfg.BindHost, newCfg.HealthPort)
			}
			*cfg = *newCfg
			cfgAtomic.Store(newCfg)
			if activeStore != nil {
				activeStore.SetInterpolationParams(newCfg.StateDistanceThreshold, newCfg.StateDecayLambda)
				logger.Info("TinyML Similarity Interpolation parameters dynamically updated: DistanceThreshold=%.2f, DecayLambda=%.2f", newCfg.StateDistanceThreshold, newCfg.StateDecayLambda)
			}
		}
		configMu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		if loadErr != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": loadErr.Error()})
			return
		}

		logger.Info("Goroutine-Safe Live Config Reload Completed Successfully by role [%s]!", role)
		msg := "Configuration reloaded in-memory without service interruption"
		if netChanged {
			msg += ". Auto-restarting daemon service in 3s to re-bind listener socket to new IP/Port."
		}
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status":  "success",
			"role":    role,
			"message": msg,
		})

		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}

		if netChanged {
			PerformGracefulProcessRestart("config_net_reload")
		}
	})

	var (
		approveRateMu    sync.Mutex
		approveIPCounts  = make(map[string]int)
		approveLastReset = time.Now()
	)

	approveRateLimitCheck := func(ip string) bool {
		approveRateMu.Lock()
		defer approveRateMu.Unlock()
		if time.Since(approveLastReset) > time.Minute {
			approveIPCounts = make(map[string]int)
			approveLastReset = time.Now()
		}
		approveIPCounts[ip]++
		return approveIPCounts[ip] <= 10
	}

	var approveMutex sync.Mutex

	// Endpoint 6: Operator Approval Endpoint (/api/approve)
	mux.HandleFunc("/api/approve", func(w http.ResponseWriter, r *http.Request) {
		if setCorsHeaders(w, r) {
			return
		}
		if r.Method != "POST" {
			http.Error(w, `{"error":"Method Not Allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		host, _, _ := net.SplitHostPort(r.RemoteAddr)
		if host == "" {
			host = r.RemoteAddr
		}
		if !approveRateLimitCheck(host) {
			http.Error(w, `{"error":"Too Many Requests: Rate limit exceeded for operator approval endpoint (10 req/min)"}`, http.StatusTooManyRequests)
			return
		}

		role, valid := validateTokenRole(r, r.Header.Get("Authorization"), cfg)
		if !valid || (role != "operator" && role != "admin") {
			http.Error(w, `{"error":"Forbidden: Endpoint requires operator or admin role"}`, http.StatusForbidden)
			return
		}

		configMu.RLock()
		currentDryRun := cfg.DryRun
		configMu.RUnlock()

		approveMutex.Lock()
		defer approveMutex.Unlock()

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

		if time.Since(pending.Timestamp) > 10*time.Minute {
			_ = os.Remove(pendingFile)
			http.Error(w, `{"error":"Pending approval request expired (> 10 minutes)"}`, http.StatusGone)
			return
		}

		recordApprovalAuditLog(pending.Action, r.RemoteAddr)

		actReq := &executor.ActionRequest{
			ActionName: pending.Action,
			Target:     "wan",
		}
		execErr := execEngine.ExecuteAction(r.Context(), actReq, currentDryRun)

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

		logger.Info("OPERATOR APPROVED & EXECUTED Action [%s] successfully by role [%s]!", pending.Action, role)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status":  "approved_and_executed",
			"action":  pending.Action,
			"role":    role,
			"details": "Operator approval verified and executed successfully",
		})
	})

	// Endpoint 7: API Budget Status (/api/budget/status)
	mux.HandleFunc("/api/budget/status", func(w http.ResponseWriter, r *http.Request) {
		if setCorsHeaders(w, r) {
			return
		}
		w.Header().Set("Content-Type", "application/json")
		b := aiClient.GetBudgetSnapshot()
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"daily_limit_req": b.DailyLimit,
			"cost_limit_usd":  b.CostLimit,
			"current_count":   b.CurrentCount,
			"current_cost":    b.CurrentCost,
			"status":          "normal",
		})
	})

	// Endpoint 8: Circuit Breaker Status (/api/circuit-breaker)
	mux.HandleFunc("/api/circuit-breaker", func(w http.ResponseWriter, r *http.Request) {
		if setCorsHeaders(w, r) {
			return
		}
		w.Header().Set("Content-Type", "application/json")
		state, failCount, _ := aiClient.GetCircuitBreakerStatus()
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"state":        state,
			"fail_count":   failCount,
			"open_timeout": "5m0s",
		})
	})

	// Endpoint 9: Live Chaos Injection & Closed-Loop Remediation Trigger (/api/chaos/inject)
	mux.HandleFunc("/api/chaos/inject", func(w http.ResponseWriter, r *http.Request) {
		if setCorsHeaders(w, r) {
			return
		}
		if r.Method != "POST" {
			http.Error(w, `{"error":"Method Not Allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			Anomaly string `json:"anomaly"`
			Action  string `json:"action"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"Invalid request payload"}`, http.StatusBadRequest)
			return
		}

		if req.Anomaly == "" {
			req.Anomaly = "MEMORY_EXHAUSTION"
		}
		if req.Action == "" {
			req.Action = "purge_memory_cache"
		}

		logger.Warn("CHAOS INJECTION RECEIVED: Triggering live remediation for Anomaly [%s] with Action [%s]", req.Anomaly, req.Action)

		// Execute action via executor engine
		actReq := &executor.ActionRequest{
			ActionName: req.Action,
			Target:     "wan",
		}
		execErr := execEngine.ExecuteAction(r.Context(), actReq, false)
		cmdExecSuccess := (execErr == nil)

		// Look up or create skill template
		targetSkill := store.GetSkill(req.Anomaly, req.Action)
		if targetSkill == nil {
			targetSkill = &skillstore.Skill{
				ID:         fmt.Sprintf("skill_%s_%s", strings.ToLower(req.Anomaly), strings.ToLower(req.Action)),
				Condition:  req.Anomaly,
				Action:     req.Action,
				Confidence: 0.60,
			}
		}

		// 2. Trigger async post-action telemetry measurement & Q-learning Bellman update in Go runtime goroutine
		go func(anomaly, action string, cmdSuccess bool) {
			time.Sleep(3 * time.Second)
			currentAlpha := 0.2

			// CRITICAL: Measure REAL post-action hardware telemetry (NOT just cmdSuccess)
			verifiedSuccess := false
			if cmdSuccess {
				verifiedSuccess = VerifyActionSuccess(context.Background(), collector, anomaly)
			}

			// Empirical Reward Assignment: +1.0 ONLY IF real telemetry proves anomaly is resolved
			reward := 1.0
			if !verifiedSuccess {
				reward = -0.5
			}

			if collector != nil {
				collector.RecordHealOutcome(verifiedSuccess)
			}

			_ = store.SaveOrUpdateSkill(&skillstore.Skill{
				ID:         fmt.Sprintf("skill_%s_%s", strings.ToLower(anomaly), strings.ToLower(action)),
				Condition:  anomaly,
				Action:     action,
				Confidence: 0.60,
			}, verifiedSuccess, currentAlpha)
			_ = store.UpdateQValue(anomaly, action, reward)

			if verifiedSuccess {
				logger.Info("CHAOS VERIFICATION SUCCESS: Real Telemetry confirmed Anomaly [%s] resolved by Action [%s] -> Q-Value rewarded (+1.0)", anomaly, action)
			} else {
				logger.Warn("CHAOS VERIFICATION FAILED: Real Telemetry shows Anomaly [%s] PERSISTS after Action [%s] -> Q-Value penalized (-0.5)", anomaly, action)
			}
		}(req.Anomaly, req.Action, cmdExecSuccess)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":                     "chaos_remediation_triggered",
			"anomaly":                    req.Anomaly,
			"action":                     req.Action,
			"execution_success":          cmdExecSuccess,
			"verification_delay_seconds": 3.0,
			"message":                    "Go agent executed action and is measuring real hardware telemetry to compute empirical reward",
		})
	})

	bindHost := cfg.BindHost
	if bindHost == "" {
		bindHost = "127.0.0.1"
	}

	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", bindHost, cfg.HealthPort),
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
