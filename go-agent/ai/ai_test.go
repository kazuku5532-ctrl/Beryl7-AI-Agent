package ai

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestCircuitBreaker(t *testing.T) {
	cb := NewCircuitBreaker(100 * time.Millisecond)
	if cb.State() != "CLOSED" {
		t.Errorf("Expected CLOSED state, got %s", cb.State())
	}

	for i := 0; i < 3; i++ {
		_ = cb.Call(func() error {
			return ErrBudgetExceeded
		})
	}

	if cb.State() != "OPEN" {
		t.Errorf("Expected OPEN state after 3 failures, got %s", cb.State())
	}

	err := cb.Call(func() error { return nil })
	if err != ErrCircuitOpen {
		t.Errorf("Expected ErrCircuitOpen, got %v", err)
	}

	time.Sleep(150 * time.Millisecond)

	_ = cb.Call(func() error { return nil })
	if cb.State() != "CLOSED" {
		t.Errorf("Expected CLOSED state after recovery, got %s", cb.State())
	}
}

func TestAPIBudget(t *testing.T) {
	client := NewClient("dummy-key")
	client.SetAPIKey("test-key")

	err := client.CheckBudgetBeforeCall(0.0001)
	if err != nil {
		t.Errorf("Expected budget check to pass, got %v", err)
	}

	client.budget.CurrentCount = 1000
	client.budget.DailyLimit = 1000
	err = client.CheckBudgetBeforeCall(0.0001)
	if err != ErrBudgetExceeded {
		t.Errorf("Expected ErrBudgetExceeded, got %v", err)
	}
}

func TestExtractJSONString(t *testing.T) {
	raw := "```json\n{\"action\":\"purge_memory_cache\",\"reasoning\":\"RAM high\",\"confidence\":0.9}\n```"
	extracted := extractJSONString(raw)
	if extracted != "{\"action\":\"purge_memory_cache\",\"reasoning\":\"RAM high\",\"confidence\":0.9}" {
		t.Errorf("Unexpected extraction: %s", extracted)
	}

	var resp AIResponse
	err := json.Unmarshal([]byte(extracted), &resp)
	if err != nil || resp.Action != "purge_memory_cache" {
		t.Errorf("Failed to parse AI response: %v", err)
	}
}

func TestExtractJSONStringRegressionAndSchemaValidation(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantAction string
		wantValid  bool
	}{
		{
			name:       "Standard Markdown Fence",
			input:      "```json\n{\"action\":\"restart_wan_interface\",\"confidence\":0.95}\n```",
			wantAction: "restart_wan_interface",
			wantValid:  true,
		},
		{
			name:       "Generic Fence",
			input:      "```\n{\"action\":\"scale_tx_power_down\",\"confidence\":0.85}\n```",
			wantAction: "scale_tx_power_down",
			wantValid:  true,
		},
		{
			name:       "Plain JSON Text",
			input:      "{\"action\":\"align_channels\",\"confidence\":0.90}",
			wantAction: "align_channels",
			wantValid:  true,
		},
		{
			name:       "Leading and Trailing Whitespace",
			input:      "  \n  {\"action\":\"purge_memory_cache\",\"confidence\":0.88} \n  ",
			wantAction: "purge_memory_cache",
			wantValid:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clean := extractJSONString(tt.input)
			var resp AIResponse
			err := json.Unmarshal([]byte(clean), &resp)
			if tt.wantValid {
				if err != nil || resp.Action != tt.wantAction {
					t.Errorf("Test %s failed: got action '%s', err %v", tt.name, resp.Action, err)
				}
			}
		})
	}
}

func TestProbeDNSAsync(t *testing.T) {
	ProbeDNSAsync()
	time.Sleep(50 * time.Millisecond)
}

func TestAnalyzeAnomalyFallback(t *testing.T) {
	client := NewClient("")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	resp, err := client.AnalyzeAnomaly(ctx, "WAN_DROP", "Link down", "syslog sample line")
	if err == nil && resp == nil {
		t.Logf("Fallback call processed gracefully")
	}
}
