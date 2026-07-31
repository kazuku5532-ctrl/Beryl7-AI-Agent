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
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"beryl7-agent/logger"
)

var macRegexGlobal = regexp.MustCompile(`(?i)([0-9a-f]{2}[:-]){5}[0-9a-f]{2}`)

func RedactMACAddresses(text string) string {
	if macRegexGlobal == nil {
		return text
	}
	return macRegexGlobal.ReplaceAllString(text, "[REDACTED_MAC]")
}

type CircuitState int

const (
	StateClosed CircuitState = iota
	StateOpen
	StateHalfOpen
)

type APIBudget struct {
	DailyLimit    int64
	CostLimit     float64
	CurrentCount  int64
	CurrentCost   float64
	LastResetTime time.Time
}

var (
	ErrBudgetExceeded = errors.New("API daily request budget exceeded")
	ErrCostExceeded   = errors.New("API daily USD cost budget exceeded")
)

type AIClient struct {
	mu           sync.Mutex
	apiKey       string
	httpClient   *http.Client
	tokens       float64
	maxTokens    float64
	refillRate   float64
	lastRefill   time.Time
	budget       APIBudget
	cb           *CircuitBreaker
}

type FunctionCall struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

type AIResponse struct {
	Action     string       `json:"action"`
	Reasoning  string       `json:"reasoning"`
	Function   FunctionCall `json:"function"`
	Confidence float64      `json:"confidence"`
}

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
		apiKey:     apiKey,
		maxTokens:  5.0,
		tokens:     5.0,
		refillRate:   10.0 / 60.0,
		lastRefill:   time.Now(),
		cb:           NewCircuitBreaker(5 * time.Minute),
		budget: APIBudget{
			DailyLimit:    1000,
			CostLimit:     3.0, // $3.00 USD/day max
			LastResetTime: time.Now(),
		},
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

func (c *AIClient) CheckBudgetBeforeCall(estimatedCost float64) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	if now.Sub(c.budget.LastResetTime) >= 24*time.Hour {
		c.budget.CurrentCount = 0
		c.budget.CurrentCost = 0
		c.budget.LastResetTime = now
	}

	if c.budget.DailyLimit > 0 && c.budget.CurrentCount >= c.budget.DailyLimit {
		return ErrBudgetExceeded
	}

	if c.budget.CostLimit > 0 && (c.budget.CurrentCost+estimatedCost) > c.budget.CostLimit {
		return ErrCostExceeded
	}

	c.budget.CurrentCount++
	c.budget.CurrentCost += estimatedCost
	return nil
}

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

func (c *AIClient) AnalyzeAnomaly(ctx context.Context, anomalyType, description, sourceLog string) (*AIResponse, error) {
	if budgetErr := c.CheckBudgetBeforeCall(0.0001); budgetErr != nil {
		logger.Warn("Cloud AI call blocked by budget limits: %v. Falling back to local skill store.", budgetErr)
		return nil, budgetErr
	}

	var resp *AIResponse
	cbErr := c.cb.Call(func() error {
		res, err := c.executeAnalyzeRequest(ctx, anomalyType, description, sourceLog)
		if err != nil {
			return err
		}
		resp = res
		return nil
	})

	return resp, cbErr
}

func (c *AIClient) executeAnalyzeRequest(ctx context.Context, anomalyType, description, sourceLog string) (*AIResponse, error) {
	c.mu.Lock()
	if !c.allowTokenCheck() {
		c.mu.Unlock()
		return nil, errors.New("rate limit exceeded: max 10 requests/min allowed")
	}
	key := c.apiKey
	c.mu.Unlock()

	if key == "" {
		return nil, errors.New("Gemini API key is empty")
	}

	prompt := fmt.Sprintf(`Anomaly Detected: %s
Details: %s
System Log: %s

Allowed actions ONLY: no_action_required, purge_memory_cache, restart_wan_interface, restart_interface, optimize_wifi_channel, block_device, set_qos_priority, set_wan_mac
Guidance:
- For MEMORY_EXHAUSTION: prefer purge_memory_cache
- For WAN_DROP: prefer restart_wan_interface
- For WIFI_FAILURE: prefer optimize_wifi_channel or restart_interface

Return JSON format ONLY: {"action":"action_name","reasoning":"clear explanation","confidence":0.0-1.0}`, anomalyType, description, sourceLog)

	prompt = RedactMACAddresses(prompt)

	reqBodyMap := map[string]interface{}{
		"contents": []map[string]interface{}{
			{"parts": []map[string]string{{"text": prompt}}},
		},
	}
	reqBytes, _ := json.Marshal(reqBodyMap)
	url := "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent"

	var httpResp *http.Response
	var err error
	for attempt := 1; attempt <= 3; attempt++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		req, reqErr := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBytes))
		if reqErr != nil {
			return nil, reqErr
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-goog-api-key", key)

		httpResp, err = c.httpClient.Do(req)
		if err == nil && httpResp.StatusCode == 200 {
			break
		}

		if httpResp != nil {
			if httpResp.StatusCode == 429 {
				retryAfterSec := 5
				if headerVal := httpResp.Header.Get("Retry-After"); headerVal != "" {
					if sec, parseErr := strconv.Atoi(headerVal); parseErr == nil {
						retryAfterSec = sec
					}
				}
				logger.Warn("Gemini API HTTP 429 Rate Limited! Backing off for %ds...", retryAfterSec)
				time.Sleep(time.Duration(retryAfterSec) * time.Second)
			}
			_ = httpResp.Body.Close()
		}

		time.Sleep(time.Duration(attempt) * 1 * time.Second)
	}

	if err != nil || httpResp == nil || httpResp.StatusCode != 200 {
		return nil, fmt.Errorf("Gemini API call failed: %v", err)
	}

	defer httpResp.Body.Close()
	limitReader := io.LimitReader(httpResp.Body, 1*1024*1024)
	bodyData, err := io.ReadAll(limitReader)
	if err != nil {
		return nil, err
	}

	var geminiResp GeminiResponseBody
	if unmarshalErr := json.Unmarshal(bodyData, &geminiResp); unmarshalErr == nil && len(geminiResp.Candidates) > 0 {
		if len(geminiResp.Candidates[0].Content.Parts) > 0 {
			rawText := geminiResp.Candidates[0].Content.Parts[0].Text
			cleanJSON := extractJSONString(rawText)
			var parsedAI AIResponse
			if err := json.Unmarshal([]byte(cleanJSON), &parsedAI); err == nil && parsedAI.Action != "" {
				logger.Info("Successfully parsed Gemini AI Cloud response: Action=[%s]", parsedAI.Action)
				return &parsedAI, nil
			}
		}
	}

	return nil, fmt.Errorf("failed to unmarshal valid AI JSON response from Gemini API payload")
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

func (c *AIClient) GetBudgetSnapshot() APIBudget {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.budget
}

func (c *AIClient) GetCircuitBreakerStatus() (string, int, time.Time) {
	if c.cb != nil {
		return c.cb.Status()
	}
	return "CLOSED", 0, time.Time{}
}
