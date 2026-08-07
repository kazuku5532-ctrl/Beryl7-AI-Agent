package orchestrator

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"beryl7-agent/config"
	"beryl7-agent/executor"
	"beryl7-agent/logger"
	"beryl7-agent/skillstore"
	"beryl7-agent/telemetry"
	"beryl7-agent/watchdog"
)

type OrchestrationState string

const (
	StateIdle       OrchestrationState = "idle"
	StateMonitoring OrchestrationState = "monitoring"
	StateAnalyzing  OrchestrationState = "analyzing"
	StateApproving  OrchestrationState = "approving"
	StateExecuting  OrchestrationState = "executing"
	StateVerifying  OrchestrationState = "verifying"
	StateSafeMode   OrchestrationState = "safe_mode"
)

type Orchestrator struct {
	mu           sync.RWMutex
	state        OrchestrationState
	eventBus     *EventBus
	writerQueue  *DBWriterQueue
	telemetry    *telemetry.TelemetryCollector
	execEngine   *executor.Executor
	skillStore   *skillstore.SkillStore
	watchdogInst *watchdog.Watchdog
	cfg          *config.Config
}

func NewOrchestrator(
	eventBus *EventBus,
	writerQueue *DBWriterQueue,
	collector *telemetry.TelemetryCollector,
	execEngine *executor.Executor,
	store *skillstore.SkillStore,
	wd *watchdog.Watchdog,
	cfg *config.Config,
) *Orchestrator {
	if eventBus == nil {
		eventBus = NewEventBus()
	}
	if writerQueue == nil {
		writerQueue = NewDBWriterQueue(128)
	}

	o := &Orchestrator{
		state:        StateIdle,
		eventBus:     eventBus,
		writerQueue:  writerQueue,
		telemetry:    collector,
		execEngine:   execEngine,
		skillStore:   store,
		watchdogInst: wd,
		cfg:          cfg,
	}

	// Wire state machine transitions to event bus events
	eventBus.Subscribe(EventAnomalyDetected, o.handleAnomalyDetected)
	eventBus.Subscribe(EventActionApproved, o.handleActionApproved)
	eventBus.Subscribe(EventActionExecuted, o.handleActionExecuted)
	eventBus.Subscribe(EventSafeModeTriggered, o.handleSafeModeTriggered)

	return o
}

func (o *Orchestrator) TransitionTo(newState OrchestrationState, reason string) {
	o.mu.Lock()
	oldState := o.state
	o.state = newState
	o.mu.Unlock()

	logger.Info("Orchestrator: State Transition [%s -> %s] (Reason: %s)", oldState, newState, reason)
}

func (o *Orchestrator) GetState() OrchestrationState {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.state
}

// CalculateContextFrictionPenalty calculates dynamic friction penalties based on active client count.
// If active Wi-Fi clients are connected, network-disruptive actions receive a 3x penalty to protect user experience.
func (o *Orchestrator) CalculateContextFrictionPenalty(ctx context.Context, actionName string) float64 {
	if o.telemetry == nil {
		return 1.0
	}

	isIdle, clientCount, _ := o.telemetry.AreWiFiClientsIdle(ctx)

	// Network-disruptive actions that cause transient disconnections
	isDisruptive := strings.Contains(actionName, "restart") ||
		strings.Contains(actionName, "revert") ||
		strings.Contains(actionName, "bandwidth") ||
		strings.Contains(actionName, "channel")

	if !isIdle && clientCount > 0 && isDisruptive {
		// Active user traffic present: 3x friction penalty applied to discourage disruptive actions
		logger.Warn("Orchestrator: Active Wi-Fi clients detected (%d clients). Applied 3.0x Friction Penalty to action [%s]", clientCount, actionName)
		return 3.0
	}

	// Network idle or non-disruptive action: 1.0x baseline penalty
	return 1.0
}

func (o *Orchestrator) handleAnomalyDetected(event *Event) {
	o.TransitionTo(StateAnalyzing, fmt.Sprintf("Anomaly detected from %s", event.Source))
}

func (o *Orchestrator) handleActionApproved(event *Event) {
	o.TransitionTo(StateExecuting, "Action approved by policy/operator")
}

func (o *Orchestrator) handleActionExecuted(event *Event) {
	o.TransitionTo(StateVerifying, "Action executed, awaiting telemetry verification")
}

func (o *Orchestrator) handleSafeModeTriggered(event *Event) {
	o.TransitionTo(StateSafeMode, "Watchdog guardrail triggered Safe Mode")
}

func (o *Orchestrator) Stop() {
	if o.writerQueue != nil {
		o.writerQueue.Stop()
	}
}
