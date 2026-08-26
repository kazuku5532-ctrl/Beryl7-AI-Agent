package notifier

import (
	"context"
	"encoding/json"
	"sync"
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

	n.SetAirgapped(true)
	if !n.airgapped {
		t.Errorf("Expected airgapped true after SetAirgapped(true)")
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

func TestSendAlertAndGetUpdatesWithMockServer(t *testing.T) {
	n := NewTelegramNotifier("123456:ABC", "987654321", false)
	if n == nil {
		t.Fatalf("Failed to create notifier")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_ = n.SendAlert(ctx, "Test Message")
	_, _ = n.getUpdates(ctx, 0, 1)
}

func TestPrivateChatUpdateParsing(t *testing.T) {
	var updatePrivate telegramUpdate
	jsonDataPrivate := `{"update_id": 101, "message": {"message_id": 1, "chat": {"id": 987654321, "type": "private"}, "text": "/status"}}`
	if err := json.Unmarshal([]byte(jsonDataPrivate), &updatePrivate); err != nil {
		t.Fatalf("Failed to parse private telegram update: %v", err)
	}
	if updatePrivate.Message.Chat.Type != "private" {
		t.Errorf("Expected chat type 'private', got '%s'", updatePrivate.Message.Chat.Type)
	}

	var updateGroup telegramUpdate
	jsonDataGroup := `{"update_id": 102, "message": {"message_id": 2, "chat": {"id": 987654321, "type": "group"}, "text": "/reboot"}}`
	if err := json.Unmarshal([]byte(jsonDataGroup), &updateGroup); err != nil {
		t.Fatalf("Failed to parse group telegram update: %v", err)
	}
	if updateGroup.Message.Chat.Type != "group" {
		t.Errorf("Expected chat type 'group', got '%s'", updateGroup.Message.Chat.Type)
	}
}

func TestSupergroupAndChannelUpdateParsing(t *testing.T) {
	var updateSupergroup telegramUpdate
	jsonDataSuper := `{"update_id": 103, "message": {"message_id": 3, "chat": {"id": 987654321, "type": "supergroup"}, "text": "/health"}}`
	if err := json.Unmarshal([]byte(jsonDataSuper), &updateSupergroup); err != nil {
		t.Fatalf("Failed to parse supergroup update: %v", err)
	}
	if updateSupergroup.Message.Chat.Type != "supergroup" {
		t.Errorf("Expected chat type 'supergroup', got '%s'", updateSupergroup.Message.Chat.Type)
	}

	var updateChannel telegramUpdate
	jsonDataChan := `{"update_id": 104, "message": {"message_id": 4, "chat": {"id": 987654321, "type": "channel"}, "text": "/boost"}}`
	if err := json.Unmarshal([]byte(jsonDataChan), &updateChannel); err != nil {
		t.Fatalf("Failed to parse channel update: %v", err)
	}
	if updateChannel.Message.Chat.Type != "channel" {
		t.Errorf("Expected chat type 'channel', got '%s'", updateChannel.Message.Chat.Type)
	}
}

func TestCommandCooldownConcurrentRace(t *testing.T) {
	n := NewTelegramNotifier("123456:ABC", "987654321", false)
	if n == nil {
		t.Fatalf("Failed to initialize TelegramNotifier")
	}

	const goroutines = 30
	var wg sync.WaitGroup
	allowedCount := int64(0)
	var countMu sync.Mutex

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if n.checkCommandCooldown("stress_cmd", 5*time.Second) {
				countMu.Lock()
				allowedCount++
				countMu.Unlock()
			}
		}()
	}

	wg.Wait()

	if allowedCount != 1 {
		t.Errorf("Expected exactly 1 execution allowed under rapid concurrency, got %d", allowedCount)
	}
}


