package notifier

import (
	"context"
	"testing"
	"time"
)

func TestNewTelegramNotifier(t *testing.T) {
	// 1. Invalid or empty tokens / chat IDs should return nil
	if n := NewTelegramNotifier("", "123456", false); n != nil {
		t.Errorf("Expected nil for empty token, got %v", n)
	}

	if n := NewTelegramNotifier("invalid_token", "invalid_id", false); n != nil {
		t.Errorf("Expected nil for non-numeric chat ID, got %v", n)
	}

	if n := NewTelegramNotifier("invalid_token", "0", false); n != nil {
		t.Errorf("Expected nil for zero chat ID, got %v", n)
	}

	// 2. Valid token and chat ID should return initialized struct
	n := NewTelegramNotifier("123456:ABCdefGhIJKlmNoPQRsTUVwxyZ", "987654321", false)
	if n == nil {
		t.Fatalf("Expected valid TelegramNotifier instance, got nil")
	}

	if n.chatID != 987654321 {
		t.Errorf("Expected chatID 987654321, got %d", n.chatID)
	}
}

func TestCommandCooldown(t *testing.T) {
	n := NewTelegramNotifier("123456:ABC", "987654321", false)
	if n == nil {
		t.Fatalf("Failed to initialize TelegramNotifier")
	}

	// First execution should pass cooldown check
	if !n.checkCommandCooldown("reboot", 2*time.Second) {
		t.Errorf("Expected first command call to pass cooldown check")
	}

	// Rapid second execution should be blocked by cooldown check
	if n.checkCommandCooldown("reboot", 2*time.Second) {
		t.Errorf("Expected rapid second call to be blocked by cooldown check")
	}

	// Wait for cooldown to expire
	time.Sleep(2100 * time.Millisecond)
	if !n.checkCommandCooldown("reboot", 2*time.Second) {
		t.Errorf("Expected call after cooldown expiration to pass")
	}
}

func TestAirgappedBypass(t *testing.T) {
	n := NewTelegramNotifier("123456:ABC", "987654321", true)
	if n == nil {
		t.Fatalf("Failed to initialize TelegramNotifier")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// In air-gapped mode, SendAlert should return nil without making external requests
	if err := n.SendAlert(ctx, "Test Message"); err != nil {
		t.Errorf("Expected nil error in air-gapped mode, got %v", err)
	}
}
