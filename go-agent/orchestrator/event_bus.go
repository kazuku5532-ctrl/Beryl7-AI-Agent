package orchestrator

import (
	"sync"
	"time"

	"beryl7-agent/logger"
)

type EventType string

const (
	EventAnomalyDetected     EventType = "anomaly_detected"
	EventMetricsUpdated      EventType = "metrics_updated"
	EventActionRecommended   EventType = "action_recommended"
	EventActionApproved      EventType = "action_approved"
	EventActionExecuted      EventType = "action_executed"
	EventVerificationPassed  EventType = "verification_passed"
	EventVerificationFailed  EventType = "verification_failed"
	EventSafeModeTriggered   EventType = "safe_mode_triggered"
)

type Event struct {
	Type        EventType              `json:"type"`
	Source      string                 `json:"source"`
	Timestamp   time.Time              `json:"timestamp"`
	Data        map[string]interface{} `json:"data"`
	Priority    int                    `json:"priority"` // 1=low, 5=critical
	Correlation string                 `json:"correlation"`
}

// [Fix 1] Deep Copy Event & Data Map to prevent Go concurrent map read and map write runtime panics
func (e *Event) Clone() *Event {
	if e == nil {
		return nil
	}
	cloned := &Event{
		Type:        e.Type,
		Source:      e.Source,
		Timestamp:   e.Timestamp,
		Priority:    e.Priority,
		Correlation: e.Correlation,
	}
	if e.Data != nil {
		cloned.Data = make(map[string]interface{}, len(e.Data))
		for k, v := range e.Data {
			cloned.Data[k] = v
		}
	}
	return cloned
}

type EventSubscriber func(event *Event)

const DefaultRingBufferSize = 256

// EventBus provides high-performance zero-allocation ring-buffer event publishing
type EventBus struct {
	mu          sync.RWMutex
	subscribers map[EventType][]EventSubscriber
	ringBuffer  [DefaultRingBufferSize]*Event
	head        int
	count       int
}

func NewEventBus() *EventBus {
	return &EventBus{
		subscribers: make(map[EventType][]EventSubscriber),
	}
}

// Subscribe registers a subscriber callback for a specific EventType
func (eb *EventBus) Subscribe(eventType EventType, subscriber EventSubscriber) {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	eb.subscribers[eventType] = append(eb.subscribers[eventType], subscriber)
}

// [Fix 4] ClearSubscribers wipes all registered subscribers to prevent ghost handlers & memory leaks on orchestrator restart/reload
func (eb *EventBus) ClearSubscribers() {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	eb.subscribers = make(map[EventType][]EventSubscriber)
}

// Publish broadcasts an event clone to subscribers and records a clone in the ring buffer (Fix 1: Concurrency Race Protection)
func (eb *EventBus) Publish(event *Event) {
	if event == nil || event.Type == "" {
		logger.Warn("EventBus: ignored invalid nil or empty event")
		return
	}

	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	// Clone event before storing in ring buffer and dispatching to prevent shared map mutations across goroutines
	eventClone := event.Clone()

	eb.mu.Lock()
	eb.ringBuffer[eb.head] = eventClone
	eb.head = (eb.head + 1) % DefaultRingBufferSize
	if eb.count < DefaultRingBufferSize {
		eb.count++
	}

	subsCopy := make([]EventSubscriber, len(eb.subscribers[event.Type]))
	copy(subsCopy, eb.subscribers[event.Type])
	eb.mu.Unlock()

	// Dispatch isolated event clones to subscribers asynchronously with panic recovery
	for _, subscriber := range subsCopy {
		sub := subscriber
		subEvent := eventClone.Clone()
		go func() {
			defer func() {
				if r := recover(); r != nil {
					logger.Error("EventBus: subscriber panic recovered [event=%s, err=%v]", subEvent.Type, r)
				}
			}()
			sub(subEvent)
		}()
	}
}

// GetRecentEvents returns up to limit recent events from the ring buffer safely cloned
func (eb *EventBus) GetRecentEvents(limit int) []*Event {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	if limit <= 0 || eb.count == 0 {
		return nil
	}
	if limit > eb.count {
		limit = eb.count
	}

	result := make([]*Event, limit)
	start := (eb.head - limit + DefaultRingBufferSize) % DefaultRingBufferSize
	for i := 0; i < limit; i++ {
		idx := (start + i) % DefaultRingBufferSize
		result[i] = eb.ringBuffer[idx].Clone()
	}
	return result
}
