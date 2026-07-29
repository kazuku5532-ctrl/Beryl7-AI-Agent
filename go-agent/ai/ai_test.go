package ai

import (
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
	raw := "```json\n{\"action\":\"purge_memory_cache\"}\n```"
	extracted := extractJSONString(raw)
	if extracted != "{\"action\":\"purge_memory_cache\"}" {
		t.Errorf("Unexpected extraction: %s", extracted)
	}
}
