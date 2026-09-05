package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"beryl7-agent/logger"
	"beryl7-agent/skillstore"
)

type TelegramNotifier struct {
	token       string
	chatID      int64
	airgapped   bool
	httpClient  *http.Client
	cooldownMu  sync.Mutex
	lastCmd     map[string]time.Time
	baseURL     string
	retryDelays []time.Duration
}

type telegramUpdate struct {
	UpdateID int `json:"update_id"`
	Message  struct {
		MessageID int `json:"message_id"`
		Chat      struct {
			ID   int64  `json:"id"`
			Type string `json:"type"`
		} `json:"chat"`
		Text string `json:"text"`
	} `json:"message"`
}

type telegramResponse struct {
	Ok     bool             `json:"ok"`
	Result []telegramUpdate `json:"result"`
}

func NewTelegramNotifier(token string, chatIDStr string, airgapped bool) *TelegramNotifier {
	cleanToken := strings.TrimSpace(token)
	cleanChatID := strings.TrimSpace(chatIDStr)

	chatID, err := strconv.ParseInt(cleanChatID, 10, 64)
	if err != nil || cleanToken == "" || chatID == 0 {
		logger.Info("TELEGRAM: Bot disabled or missing valid configuration (TOKEN/CHAT_ID).")
		return nil
	}

	return &TelegramNotifier{
		token:       cleanToken,
		chatID:      chatID,
		airgapped:   airgapped,
		lastCmd:     make(map[string]time.Time),
		baseURL:     "https://api.telegram.org",
		retryDelays: []time.Duration{5 * time.Second, 10 * time.Second, 20 * time.Second, 30 * time.Second, 60 * time.Second},
		httpClient: &http.Client{
			Timeout: 35 * time.Second, // Timeout >= 30s to allow native long-polling
		},
	}
}

func (t *TelegramNotifier) SetBaseURL(url string) {
	if t == nil {
		return
	}
	t.baseURL = url
}

func (t *TelegramNotifier) SetRetryDelays(delays []time.Duration) {
	if t == nil {
		return
	}
	t.retryDelays = delays
}

func (t *TelegramNotifier) SetHTTPClient(client *http.Client) {
	if t == nil {
		return
	}
	t.httpClient = client
}

func (t *TelegramNotifier) SetAirgapped(enabled bool) {
	if t == nil {
		return
	}
	t.airgapped = enabled
}

func (t *TelegramNotifier) SendAlert(ctx context.Context, message string) error {
	if t == nil || t.airgapped {
		return nil
	}

	apiURL := fmt.Sprintf("%s/bot%s/sendMessage", t.baseURL, t.token)
	payload := map[string]interface{}{
		"chat_id":    t.chatID,
		"text":       message,
		"parse_mode": "Markdown",
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("telegram send failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram API returned HTTP status %d", resp.StatusCode)
	}

	return nil
}

func (t *TelegramNotifier) SendAlertWithBackoff(ctx context.Context, message string, maxWait time.Duration) error {
	if t == nil || t.airgapped {
		return nil
	}

	ctxTimeout, cancel := context.WithTimeout(ctx, maxWait)
	defer cancel()

	var err error
	for _, delay := range t.retryDelays {
		err = t.SendAlert(ctxTimeout, message)
		if err == nil {
			return nil
		}
		logger.Warn("TELEGRAM: Alert delivery failed: %v. Retrying in %v...", err, delay)

		select {
		case <-ctxTimeout.Done():
			logger.Warn("TELEGRAM: Alert delivery expired after %v backoff limit: %v", maxWait, err)
			return ctxTimeout.Err()
		case <-time.After(delay):
		}
	}

	// Keep trying every 60s until deadline
	for {
		err = t.SendAlert(ctxTimeout, message)
		if err == nil {
			return nil
		}
		logger.Warn("TELEGRAM: Alert delivery failed: %v. Retrying in %v...", err, 60*time.Second)

		select {
		case <-ctxTimeout.Done():
			logger.Warn("TELEGRAM: Alert delivery expired after %v backoff limit: %v", maxWait, err)
			return ctxTimeout.Err()
		case <-time.After(60 * time.Second):
		}
	}
}

func (t *TelegramNotifier) SendPowerLossRecoveryAlert(ctx context.Context, rebootTime time.Time, wanStatus string) error {
	msg := fmt.Sprintf("⚠️ *CẢNH BÁO: Mất điện đột ngột / Sập nguồn!*\n\n"+
		"Beryl 7 AI Agent vừa khởi động lại sau một sự cố tắt nguồn không an toàn.\n"+
		"🕒 *Thời điểm khởi động:* `%s`\n"+
		"🌐 *Trạng thái WAN hiện tại:* `%s`\n\n"+
		"💡 Agent đã tự động khôi phục cấu hình an toàn.", rebootTime.Format(time.RFC1123), wanStatus)

	return t.SendAlertWithBackoff(ctx, msg, 10*time.Minute)
}

func (t *TelegramNotifier) SendMilestoneAlert(ctx context.Context, metrics skillstore.OperationalMetrics, threshold int) error {
	msg := fmt.Sprintf("📊 *Beryl 7 AI Agent - Milestone Report*\n\n"+
		"Threshold reached: %d\n"+
		"Total Q-Updates: `%d`\n"+
		"Verified Successes: `%d`\n"+
		"Verified Failures: `%d`\n"+
		"Interpolations: `%d`\n"+
		"Exact Matches: `%d`\n"+
		"Fallback Defaults: `%d`\n\n"+
		"Data is ready for user review.",
		threshold,
		metrics.TotalQUpdates,
		metrics.VerifiedSuccesses,
		metrics.VerifiedFailures,
		metrics.Interpolations,
		metrics.ExactMatchCount,
		metrics.FallbackDefaultCount)

	return t.SendAlert(ctx, msg)
}

func (t *TelegramNotifier) SendTelemetryReadinessAlert(ctx context.Context, oldestUnix, newestUnix, totalRecords int64) error {
	startDate := time.Unix(oldestUnix, 0).UTC().Format("2006-01-02 15:04:05 UTC")
	endDate := time.Unix(newestUnix, 0).UTC().Format("2006-01-02 15:04:05 UTC")

	msg := fmt.Sprintf("📈 *Beryl 7 AI Agent - Telemetry Data Readiness (14 Days)*\n\n"+
		"Hệ thống đã tích lũy đủ 14 ngày dữ liệu telemetry liên tục.\n"+
		"🕒 *Thời gian bắt đầu:* `%s`\n"+
		"🕒 *Thời gian kết thúc:* `%s`\n"+
		"📊 *Tổng số bản ghi:* `%d`\n\n"+
		"💡 Dữ liệu chuỗi thời gian (time-series) đã sẵn sàng phục vụ Phân tích Dự đoán (Predictive Analysis - Phase 2b).",
		startDate, endDate, totalRecords)

	return t.SendAlert(ctx, msg)
}

func (t *TelegramNotifier) checkCommandCooldown(cmdKey string, cooldownDur time.Duration) bool {
	t.cooldownMu.Lock()
	defer t.cooldownMu.Unlock()

	now := time.Now()
	if last, exists := t.lastCmd[cmdKey]; exists {
		if now.Sub(last) < cooldownDur {
			return false // In cooldown period
		}
	}
	t.lastCmd[cmdKey] = now
	return true
}

func (t *TelegramNotifier) StartCommandListener(ctx context.Context, execCmd func(cmd string) string) {
	if t == nil {
		return
	}

	if t.airgapped {
		logger.Info("TELEGRAM: Listener bypassed due to Air-Gapped Mode.")
		return
	}

	logger.Info("TELEGRAM: Active Long Polling command listener started (Operator Chat ID: %d)...", t.chatID)
	offset := 0

	go func() {
		for {
			select {
			case <-ctx.Done():
				logger.Info("TELEGRAM: Command listener shutting down gracefully.")
				return
			default:
				if t.airgapped {
					time.Sleep(5 * time.Second)
					continue
				}

				updates, err := t.getUpdates(ctx, offset, 30)
				if err != nil {
					if ctx.Err() != nil {
						return
					}
					logger.Debug("TELEGRAM: Long polling connection retry (WAN jitter/disconnect): %v", err)
					time.Sleep(5 * time.Second)
					continue
				}

				for _, update := range updates {
					if update.UpdateID >= offset {
						offset = update.UpdateID + 1
					}

					// Strict Whitelist Validation: Reject messages from unauthorized Chat IDs or non-private chats
					if update.Message.Chat.ID != t.chatID {
						logger.Warn("TELEGRAM: Unauthorized command attempt rejected from Chat ID: %d", update.Message.Chat.ID)
						continue
					}
					if update.Message.Chat.Type != "" && update.Message.Chat.Type != "private" {
						logger.Warn("TELEGRAM: Unauthorized non-private chat type rejected (%s) for Chat ID: %d", update.Message.Chat.Type, update.Message.Chat.ID)
						continue
					}

					text := strings.TrimSpace(update.Message.Text)
					if text == "" {
						continue
					}

					cmdLower := strings.ToLower(text)
					logger.Info("TELEGRAM: Received authorized command from Operator: '%s'", text)

					// Anti-Spam / Cooldown check for sensitive execution actions
					if (strings.HasPrefix(cmdLower, "/reboot") || strings.Contains(cmdLower, "khởi động")) && !t.checkCommandCooldown("reboot", 30*time.Second) {
						_ = t.SendAlert(ctx, "⏳ *Lệnh Reboot đang trong thời gian chờ (Cooldown 30s).* Vui lòng thử lại sau.")
						continue
					}

					if (strings.HasPrefix(cmdLower, "/boost") || strings.Contains(cmdLower, "tăng tốc")) && !t.checkCommandCooldown("boost", 10*time.Second) {
						_ = t.SendAlert(ctx, "⏳ *Lệnh Boost đang trong thời gian chờ (Cooldown 10s).* Vui lòng thử lại sau.")
						continue
					}

					// Execute command via callback and send formatted response
					responseMsg := execCmd(text)
					if responseMsg != "" {
						_ = t.SendAlert(ctx, responseMsg)
					}
				}
			}
		}
	}()
}

func (t *TelegramNotifier) getUpdates(ctx context.Context, offset int, timeoutSec int) ([]telegramUpdate, error) {
	apiURL := fmt.Sprintf("%s/bot%s/getUpdates?offset=%d&timeout=%d", t.baseURL, t.token, offset, timeoutSec)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP status %d", resp.StatusCode)
	}

	var res telegramResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}

	return res.Result, nil
}
