package orchestrator

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"beryl7-agent/telemetry"
)

func TestEventBusPublishSubscribeAndRingBuffer(t *testing.T) {
	bus := NewEventBus()

	var receivedCount int32
	bus.Subscribe(EventAnomalyDetected, func(event *Event) {
		atomic.AddInt32(&receivedCount, 1)
	})

	bus.Publish(&Event{
		Type:   EventAnomalyDetected,
		Source: "test_watchdog",
		Data:   map[string]interface{}{"type": "WAN_DROP"},
	})

	time.Sleep(50 * time.Millisecond)

	if atomic.LoadInt32(&receivedCount) != 1 {
		t.Errorf("Expected 1 event received, got %d", atomic.LoadInt32(&receivedCount))
	}

	recent := bus.GetRecentEvents(10)
	if len(recent) != 1 {
		t.Fatalf("Expected 1 recent event in ring buffer, got %d", len(recent))
	}
	if recent[0].Source != "test_watchdog" {
		t.Errorf("Expected source 'test_watchdog', got '%s'", recent[0].Source)
	}
}

// TestEventBusDeepCopyMapRace verifies Fix 1: Event.Clone() prevents Go concurrent map read and map write panics
func TestEventBusDeepCopyMapRace(t *testing.T) {
	bus := NewEventBus()

	sharedData := map[string]interface{}{
		"key1": "value1",
		"key2": 100,
	}

	// Register 10 concurrent subscribers that modify their event.Data maps
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		subID := i
		wg.Add(1)
		bus.Subscribe(EventMetricsUpdated, func(event *Event) {
			defer wg.Done()
			event.Data[fmt.Sprintf("sub_%d", subID)] = true
			_ = event.Data["key1"]
		})
	}

	bus.Publish(&Event{
		Type:   EventMetricsUpdated,
		Source: "race_test",
		Data:   sharedData,
	})

	wg.Wait()

	// Verify original sharedData map was untouched by subscribers
	if len(sharedData) != 2 {
		t.Errorf("Expected original map to have 2 keys, got %d (Deep Copy Clone failed)", len(sharedData))
	}
}

// TestDBWriterQueueGracefulDrainOnStop verifies Fix 2: Queue drains all pending tasks cleanly upon Stop without task loss
func TestDBWriterQueueGracefulDrainOnStop(t *testing.T) {
	queue := NewDBWriterQueue(64)

	var executedCount int32
	for i := 0; i < 20; i++ {
		queue.Enqueue(func() error {
			time.Sleep(2 * time.Millisecond)
			atomic.AddInt32(&executedCount, 1)
			return nil
		})
	}

	// Stop queue immediately
	queue.Stop()

	if atomic.LoadInt32(&executedCount) != 20 {
		t.Errorf("Expected all 20 tasks to be drained on Stop, got %d", atomic.LoadInt32(&executedCount))
	}

	// Enqueue after Stop must safely return false
	if queue.Enqueue(func() error { return nil }) {
		t.Errorf("Expected Enqueue to return false after Stop()")
	}
}

// TestOrchestratorVerificationHandlersNoDeadlock verifies Fix 3 & Fix 5: StateMachine transitions to Approving and Idle/SafeMode correctly
func TestOrchestratorVerificationHandlersNoDeadlock(t *testing.T) {
	bus := NewEventBus()
	queue := NewDBWriterQueue(32)
	defer queue.Stop()

	orch := NewOrchestrator(bus, queue, nil, nil, nil, nil, nil)

	// Fix 5: EventActionRecommended -> StateApproving
	bus.Publish(&Event{Type: EventActionRecommended, Source: "ai_engine"})
	time.Sleep(50 * time.Millisecond)
	if orch.GetState() != StateApproving {
		t.Errorf("Expected StateApproving after EventActionRecommended, got %s", orch.GetState())
	}

	// EventActionApproved -> StateExecuting
	bus.Publish(&Event{Type: EventActionApproved, Source: "operator"})
	time.Sleep(50 * time.Millisecond)
	if orch.GetState() != StateExecuting {
		t.Errorf("Expected StateExecuting after EventActionApproved, got %s", orch.GetState())
	}

	// EventActionExecuted -> StateVerifying
	bus.Publish(&Event{Type: EventActionExecuted, Source: "executor"})
	time.Sleep(50 * time.Millisecond)
	if orch.GetState() != StateVerifying {
		t.Errorf("Expected StateVerifying after EventActionExecuted, got %s", orch.GetState())
	}

	// Fix 3: EventVerificationPassed -> StateIdle (deadlock broken!)
	bus.Publish(&Event{Type: EventVerificationPassed, Source: "verifier"})
	time.Sleep(50 * time.Millisecond)
	if orch.GetState() != StateIdle {
		t.Errorf("Expected StateIdle after EventVerificationPassed, got %s", orch.GetState())
	}

	// Test Verification Failed -> StateSafeMode when critical
	bus.Publish(&Event{Type: EventActionExecuted, Source: "executor"})
	time.Sleep(50 * time.Millisecond)
	bus.Publish(&Event{
		Type:   EventVerificationFailed,
		Source: "verifier",
		Data:   map[string]interface{}{"safe_mode": true},
	})
	time.Sleep(50 * time.Millisecond)
	if orch.GetState() != StateSafeMode {
		t.Errorf("Expected StateSafeMode after critical EventVerificationFailed, got %s", orch.GetState())
	}
}

// TestOrchestratorStopClearsSubscribers verifies Fix 4: Stop() unregisters subscribers from EventBus
func TestOrchestratorStopClearsSubscribers(t *testing.T) {
	bus := NewEventBus()
	queue := NewDBWriterQueue(32)

	orch := NewOrchestrator(bus, queue, nil, nil, nil, nil, nil)
	orch.Stop()

	// Publish event after Stop - handler should NOT fire
	bus.Publish(&Event{Type: EventAnomalyDetected, Source: "ghost_test"})
	time.Sleep(50 * time.Millisecond)

	// State should remain StateIdle because subscribers were cleared
	if orch.GetState() != StateIdle {
		t.Errorf("Expected StateIdle when event published after Stop(), got %s (ghost handler leak)", orch.GetState())
	}
}

func TestCalculateContextFrictionPenalty(t *testing.T) {
	collector := telemetry.NewCollector()
	ctx := context.Background()

	bus := NewEventBus()
	queue := NewDBWriterQueue(32)
	defer queue.Stop()

	orch := NewOrchestrator(bus, queue, collector, nil, nil, nil, nil)

	p1 := orch.CalculateContextFrictionPenalty(ctx, "purge_memory_cache")
	if p1 != 1.0 {
		t.Errorf("Expected 1.0x penalty for non-disruptive action, got %.1f", p1)
	}

	p2 := orch.CalculateContextFrictionPenalty(ctx, "restart_wan_interface")
	if p2 != 1.0 {
		t.Errorf("Expected 1.0x penalty when idle, got %.1f", p2)
	}
}
