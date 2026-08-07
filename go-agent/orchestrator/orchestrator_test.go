package orchestrator

import (
	"context"
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

func TestEventBusRingBufferRotation(t *testing.T) {
	bus := NewEventBus()

	// Fill buffer beyond 256 size to verify zero-alloc FIFO rotation
	for i := 0; i < 300; i++ {
		bus.Publish(&Event{
			Type:   EventMetricsUpdated,
			Source: "telemetry",
			Data:   map[string]interface{}{"index": i},
		})
	}

	recent := bus.GetRecentEvents(10)
	if len(recent) != 10 {
		t.Fatalf("Expected 10 recent events, got %d", len(recent))
	}
}

func TestDBWriterQueueExecution(t *testing.T) {
	queue := NewDBWriterQueue(64)
	defer queue.Stop()

	var executed int32
	success := queue.Enqueue(func() error {
		atomic.AddInt32(&executed, 1)
		return nil
	})

	if !success {
		t.Fatal("Failed to enqueue task")
	}

	time.Sleep(50 * time.Millisecond)

	if atomic.LoadInt32(&executed) != 1 {
		t.Errorf("Expected task executed 1 time, got %d", atomic.LoadInt32(&executed))
	}
}

func TestOrchestratorStateTransitions(t *testing.T) {
	bus := NewEventBus()
	queue := NewDBWriterQueue(32)
	defer queue.Stop()

	orch := NewOrchestrator(bus, queue, nil, nil, nil, nil, nil)
	if orch.GetState() != StateIdle {
		t.Errorf("Expected initial state StateIdle, got %s", orch.GetState())
	}

	bus.Publish(&Event{Type: EventAnomalyDetected, Source: "test"})
	time.Sleep(50 * time.Millisecond)

	if orch.GetState() != StateAnalyzing {
		t.Errorf("Expected StateAnalyzing after anomaly event, got %s", orch.GetState())
	}

	bus.Publish(&Event{Type: EventActionApproved, Source: "test"})
	time.Sleep(50 * time.Millisecond)

	if orch.GetState() != StateExecuting {
		t.Errorf("Expected StateExecuting after approval, got %s", orch.GetState())
	}

	bus.Publish(&Event{Type: EventSafeModeTriggered, Source: "test"})
	time.Sleep(50 * time.Millisecond)

	if orch.GetState() != StateSafeMode {
		t.Errorf("Expected StateSafeMode after safe mode event, got %s", orch.GetState())
	}
}

func TestCalculateContextFrictionPenalty(t *testing.T) {
	collector := telemetry.NewCollector()
	ctx := context.Background()

	bus := NewEventBus()
	queue := NewDBWriterQueue(32)
	defer queue.Stop()

	orch := NewOrchestrator(bus, queue, collector, nil, nil, nil, nil)

	// Non-disruptive action -> 1.0x penalty
	p1 := orch.CalculateContextFrictionPenalty(ctx, "purge_memory_cache")
	if p1 != 1.0 {
		t.Errorf("Expected 1.0x penalty for non-disruptive action, got %.1f", p1)
	}

	// Disruptive action with zero clients (idle test env) -> 1.0x penalty
	p2 := orch.CalculateContextFrictionPenalty(ctx, "restart_wan_interface")
	if p2 != 1.0 {
		t.Errorf("Expected 1.0x penalty when idle, got %.1f", p2)
	}
}

func TestEventBusPanicRecovery(t *testing.T) {
	bus := NewEventBus()

	// Panicking subscriber
	bus.Subscribe(EventAnomalyDetected, func(event *Event) {
		panic("simulated subscriber crash")
	})

	// Normal subscriber
	var normalExecuted int32
	bus.Subscribe(EventAnomalyDetected, func(event *Event) {
		atomic.StoreInt32(&normalExecuted, 1)
	})

	bus.Publish(&Event{Type: EventAnomalyDetected, Source: "test"})
	time.Sleep(50 * time.Millisecond)

	if atomic.LoadInt32(&normalExecuted) != 1 {
		t.Errorf("Expected normal subscriber to execute despite panic in sibling subscriber")
	}
}
