package ai

import (
	"errors"
	"sync"
	"time"
)

var ErrCircuitOpen = errors.New("circuit breaker is OPEN: Cloud AI requests blocked to prevent overload")

type CircuitBreaker struct {
	mu          sync.Mutex
	state       string // "CLOSED", "OPEN", "HALF_OPEN"
	failCount   int
	lastFailAt  time.Time
	openTimeout time.Duration
}

func NewCircuitBreaker(timeout time.Duration) *CircuitBreaker {
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	return &CircuitBreaker{
		state:       "CLOSED",
		openTimeout: timeout,
	}
}

func (cb *CircuitBreaker) Call(fn func() error) error {
	cb.mu.Lock()
	if cb.state == "OPEN" {
		if time.Since(cb.lastFailAt) > cb.openTimeout {
			cb.state = "HALF_OPEN"
		} else {
			cb.mu.Unlock()
			return ErrCircuitOpen
		}
	}
	cb.mu.Unlock()

	err := fn()

	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.failCount++
		cb.lastFailAt = time.Now()
		if cb.failCount >= 3 {
			cb.state = "OPEN"
		}
		return err
	}

	cb.failCount = 0
	cb.state = "CLOSED"
	return nil
}

func (cb *CircuitBreaker) State() string {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}

func (cb *CircuitBreaker) Status() (string, int, time.Time) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state, cb.failCount, cb.lastFailAt
}
