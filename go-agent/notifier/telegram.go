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
)

type TelegramNotifier struct {
	token      string
	chatID     int64
	airgapped  bool
	httpClient *http.Client
	cooldownMu sync.Mutex
	lastCmd    map[string]time.Time
}

type telegramUpdate struct {
	UpdateID int `json:"update_id"`
	Message  struct {
		MessageID int `json:"message_id"`
		Chat      struct {
			ID int64 `json:"id"`
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
		token:     cleanToken,
		chatID:    chatID,
		airgapped: airgapped,
		lastCmd:   make(map[string]time.Time),
		httpClient: &http.Client{
			Timeout: 35 * time.Second, // Timeout >= 30s to allow native long-polling
		},
	}
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

	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.token)
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

					// Strict Whitelist Validation: Reject messages from unauthorized Chat IDs
					if update.Message.Chat.ID != t.chatID {
						logger.Warn("TELEGRAM: Unauthorized command attempt rejected from Chat ID: %d", update.Message.Chat.ID)
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
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates?offset=%d&timeout=%d", t.token, offset, timeoutSec)

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
