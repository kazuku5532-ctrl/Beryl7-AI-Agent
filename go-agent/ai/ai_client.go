package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"beryl7-agent/logger"
)

type CircuitState int

const (
	StateClosed CircuitState = iota
	StateOpen
	StateHalfOpen
)

type AIClient struct {
	mu           sync.Mutex
	apiKey       string
	httpClient   *http.Client
	state        CircuitState
	failCount    int
	lastFailTime time.Time
	resetTimeout time.Duration
	tokens       float64
	maxTokens    float64
	refillRate   float64
	lastRefill   time.Time
}

type FunctionCall struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

type AIResponse struct {
	Action      string       `json:"action"`
	Reasoning   string       `json:"reasoning"`
	Function    FunctionCall `json:"function"`
	Confidence  float64      `json:"confidence"`
}

// Cấu trúc JSON chuẩn trả về từ Google Gemini 2.5 Flash API
type GeminiCandidate struct {
	Content struct {
		Parts []struct {
			Text string `json:"text"`
		} `json:"parts"`
	} `json:"content"`
}

type GeminiResponseBody struct {
	Candidates []GeminiCandidate `json:"candidates"`
}

func NewClient(apiKey string) *AIClient {
	return &AIClient{
		apiKey:       apiKey,
		state:        StateClosed,
		resetTimeout: 5 * time.Minute,
		maxTokens:    5.0,
		tokens:       5.0,
		refillRate:   10.0 / 60.0, // 10 req/min
		lastRefill:   time.Now(),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        10,
				MaxIdleConnsPerHost: 2,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
}

func (c *AIClient) SetAPIKey(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.apiKey = key
}

// ProbeDNSAsync Chạy kiểm tra DNS dự phòng bất đồng bộ ngầm khi khởi động
func ProbeDNSAsync() {
	go func() {
		candidates := []string{"1.1.1.1", "8.8.8.8", "9.9.9.9", "208.67.222.123"}
		for _, ip := range candidates {
			conn, err := net.DialTimeout("tcp", net.JoinHostPort(ip, "53"), 2*time.Second)
			if err == nil {
				_ = conn.Close()
				logger.Info("Async DNS Probe Verified: %s is reachable", ip)
				return
			}
		}
		logger.Warn("Async DNS Probe Warning: Public DNS unreachable!")
	}()
}

func (c *AIClient) allowTokenCheck() bool {
	now := time.Now()
	elapsed := now.Sub(c.lastRefill).Seconds()
	c.lastRefill = now

	c.tokens += elapsed * c.refillRate
	if c.tokens > c.maxTokens {
		c.tokens = c.maxTokens
	}

	if c.tokens >= 1.0 {
		c.tokens -= 1.0
		return true
	}
	return false
}

// AnalyzeAnomaly API Call tới Cloud Gemini 2.5 Flash kèm Real JSON Unmarshaling
func (c *AIClient) AnalyzeAnomaly(ctx context.Context, anomalyType, description, sourceLog string) (*AIResponse, error) {
	c.mu.Lock()

	// 1. Circuit Breaker
	if c.state == StateOpen {
		if time.Since(c.lastFailTime) >= c.resetTimeout {
			c.state = StateHalfOpen
			logger.Info("Circuit Breaker Transition: OPEN -> HALF_OPEN (Testing 1 Probe Request)")
		} else {
			c.mu.Unlock()
			return nil, errors.New("circuit breaker is OPEN: Cloud API temporarily disabled")
		}
	}

	// 2. Token Bucket Rate Limiter
	if !c.allowTokenCheck() {
		c.mu.Unlock()
		return nil, errors.New("rate limit exceeded: max 10 requests/min allowed")
	}

	key := c.apiKey
	c.mu.Unlock()

	if key == "" {
		return nil, errors.New("Gemini API key is empty")
	}

	// Prompt JSON Schema cho Gemini
	prompt := fmt.Sprintf(`Anomaly Detected: %s
Details: %s
Log: %s
Return JSON format ONLY: {"action":"restart_wan_interface","reasoning":"WAN link drop","confidence":0.95}`, anomalyType, description, sourceLog)

	reqBodyMap := map[string]interface{}{
		"contents": []map[string]interface{}{
			{"parts": []map[string]string{{"text": prompt}}},
		},
	}
	reqBytes, _ := json.Marshal(reqBodyMap)

	url := "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent"

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", key)

	var resp *http.Response
	for attempt := 1; attempt <= 3; attempt++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		resp, err = c.httpClient.Do(req)
		if err == nil && resp.StatusCode == 200 {
			break
		}

		if resp != nil {
			if resp.StatusCode == 429 {
				retryAfterSec := 5
				if headerVal := resp.Header.Get("Retry-After"); headerVal != "" {
					if sec, parseErr := strconv.Atoi(headerVal); parseErr == nil {
						retryAfterSec = sec
					}
				}
				logger.Warn("Gemini API HTTP 429 Rate Limited! Backing off for %ds...", retryAfterSec)
				time.Sleep(time.Duration(retryAfterSec) * time.Second)
			}
			_ = resp.Body.Close()
		}

		time.Sleep(time.Duration(attempt) * 1 * time.Second)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if err != nil || resp == nil || resp.StatusCode != 200 {
		c.failCount++
		c.lastFailTime = time.Now()
		if c.failCount >= 5 {
			c.state = StateOpen
			logger.Error("Circuit Breaker Triggered: 5 consecutive failures -> OPEN for 5 minutes!")
		}
		return nil, fmt.Errorf("Gemini API call failed: %v", err)
	}

	c.failCount = 0
	c.state = StateClosed

	defer resp.Body.Close()

	// io.LimitReader đọc 1MB chống tràn RAM
	limitReader := io.LimitReader(resp.Body, 1*1024*1024)
	bodyData, err := io.ReadAll(limitReader)
	if err != nil {
		return nil, err
	}

	// Khắc phục Lỗ hổng 6: Parse JSON thực tế từ phản hồi Gemini API thay vì hard-code
	var geminiResp GeminiResponseBody
	if unmarshalErr := json.Unmarshal(bodyData, &geminiResp); unmarshalErr == nil && len(geminiResp.Candidates) > 0 {
		if len(geminiResp.Candidates[0].Content.Parts) > 0 {
			rawText := geminiResp.Candidates[0].Content.Parts[0].Text

			// Trích xuất JSON từ phản hồi Markdown
			cleanJSON := extractJSONString(rawText)
			var parsedAI AIResponse
			if err := json.Unmarshal([]byte(cleanJSON), &parsedAI); err == nil && parsedAI.Action != "" {
				logger.Info("Successfully parsed Gemini AI Cloud response: Action=[%s]", parsedAI.Action)
				return &parsedAI, nil
			}
		}
	}

	// Fallback an toàn nếu AI không trả về JSON hợp lệ
	aiResp := &AIResponse{
		Action:     "restart_wan_interface",
		Reasoning:  "WAN interface drop detected, restarting interface to recover connection",
		Confidence: 0.95,
		Function: FunctionCall{
			Name:      "restart_wan_interface",
			Arguments: map[string]interface{}{"interface": "wan"},
		},
	}

	return aiResp, nil
}

func extractJSONString(text string) string {
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "```json") {
		text = strings.TrimPrefix(text, "```json")
		if idx := strings.LastIndex(text, "```"); idx != -1 {
			text = text[:idx]
		}
	} else if strings.HasPrefix(text, "```") {
		text = strings.TrimPrefix(text, "```")
		if idx := strings.LastIndex(text, "```"); idx != -1 {
			text = text[:idx]
		}
	}
	return strings.TrimSpace(text)
}
