package llmgateway

import (
	"fmt"
	"sync/atomic"
	"time"
)

const (
	cbClosed   int32 = 0
	cbOpen     int32 = 1
	cbHalfOpen int32 = 2

	cbFailThreshold = 3
	cbOpenDuration  = 30 * time.Second
)

// CircuitBreaker trips open after repeated 5xx-class failures.
type CircuitBreaker struct {
	state     atomic.Int32
	failures  atomic.Int32
	openUntil atomic.Int64
}

func (cb *CircuitBreaker) Allow() error {
	switch cb.state.Load() {
	case cbOpen:
		if time.Now().UnixNano() > cb.openUntil.Load() {
			cb.state.CompareAndSwap(cbOpen, cbHalfOpen)
			return nil
		}
		return fmt.Errorf("circuit breaker open: provider temporarily unavailable")
	default:
		return nil
	}
}

func (cb *CircuitBreaker) OnSuccess() {
	cb.failures.Store(0)
	cb.state.Store(cbClosed)
}

func (cb *CircuitBreaker) OnFailure(is5xx bool) {
	if !is5xx {
		return
	}
	if cb.failures.Add(1) >= cbFailThreshold {
		cb.openUntil.Store(time.Now().Add(cbOpenDuration).UnixNano())
		cb.state.Store(cbOpen)
	}
}
