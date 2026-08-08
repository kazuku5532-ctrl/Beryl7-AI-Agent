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
	// [Fix 5] Handle AI recommendation event to transition to StateApproving
	eventBus.Subscribe(EventActionRecommended, o.handleActionRecommended)
	eventBus.Subscribe(EventActionApproved, o.handleActionApproved)
	eventBus.Subscribe(EventActionExecuted, o.handleActionExecuted)
	// [Fix 3] Handle verification outcomes to prevent permanent StateVerifying deadlocks
	eventBus.Subscribe(EventVerificationPassed, o.handleVerificationPassed)
	eventBus.Subscribe(EventVerificationFailed, o.handleVerificationFailed)
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

	// Network-disruptive actions that cause transient disconnections (including Smart Repeater Guard actions)
	isDisruptive := strings.Contains(actionName, "restart") ||
		strings.Contains(actionName, "revert") ||
		strings.Contains(actionName, "bandwidth") ||
		strings.Contains(actionName, "channel") ||
		strings.Contains(actionName, "power") ||
		strings.Contains(actionName, "failover") ||
		strings.Contains(actionName, "align")

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

// [Fix 5] Transition to StateApproving when AI recommends an action
func (o *Orchestrator) handleActionRecommended(event *Event) {
	o.TransitionTo(StateApproving, "Action recommended by AI engine, awaiting approval gate")
}

func (o *Orchestrator) handleActionApproved(event *Event) {
	o.TransitionTo(StateExecuting, "Action approved by policy/operator")
}

func (o *Orchestrator) handleActionExecuted(event *Event) {
	o.TransitionTo(StateVerifying, "Action executed, awaiting telemetry verification")
}

// [Fix 3 & Q-Learning V2] Transition back to StateIdle when verification passes and reward Q-Table with +1.0
func (o *Orchestrator) handleVerificationPassed(event *Event) {
	if event != nil && event.Data != nil && o.skillStore != nil {
		state, _ := event.Data["state"].(string)
		action, _ := event.Data["action"].(string)
		if state != "" && action != "" {
			_ = o.skillStore.UpdateQValue(state, action, 1.0)
			logger.Info("Q-LEARNING: Action [%s] for State [%s] succeeded! Q-Value updated with Reward +1.0", action, state)
		}
	}
	o.TransitionTo(StateIdle, "Verification passed, returning to idle monitoring state")
}

// [Fix 3 & Q-Learning V2] Handle verification failures by penalizing Q-Table with soft penalty -0.5
func (o *Orchestrator) handleVerificationFailed(event *Event) {
	if event != nil && event.Data != nil {
		if o.skillStore != nil {
			state, _ := event.Data["state"].(string)
			action, _ := event.Data["action"].(string)
			if state != "" && action != "" {
				_ = o.skillStore.UpdateQValue(state, action, -0.5)
				logger.Warn("Q-LEARNING: Action [%s] for State [%s] failed! Q-Value updated with Reward -0.5 (Penalty)", action, state)
			}
		}

		if safeMode, ok := event.Data["safe_mode"].(bool); ok && safeMode {
			o.TransitionTo(StateSafeMode, "Verification failed critically, entering Safe Mode")
			return
		}
	}
	o.TransitionTo(StateIdle, "Verification failed non-critically, returning to idle state")
}

func (o *Orchestrator) handleSafeModeTriggered(event *Event) {
	o.TransitionTo(StateSafeMode, "Watchdog guardrail triggered Safe Mode")
}

// [Fix 4] Stop clears event bus subscribers to prevent ghost handlers and memory leaks on orchestrator restart
func (o *Orchestrator) Stop() {
	if o.eventBus != nil {
		o.eventBus.ClearSubscribers()
	}
	if o.writerQueue != nil {
		o.writerQueue.Stop()
	}
}
