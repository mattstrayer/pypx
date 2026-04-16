package circuitbreaker

import (
	"errors"
	"sync"
	"time"
)

// ErrOpen is returned by Allow when the circuit breaker is open.
var ErrOpen = errors.New("circuit breaker is open")

// State represents the current state of a Breaker.
type State int

const (
	Closed   State = iota // normal operation
	Open                  // tripped; requests rejected
	HalfOpen              // cooldown expired; one probe allowed
)

// Breaker is a simple three-state circuit breaker.
// It trips to Open after threshold consecutive failures, then transitions to
// HalfOpen after the cooldown period to allow one probe request.
type Breaker struct {
	mu           sync.Mutex
	state        State
	failures     int
	threshold    int
	cooldown     time.Duration
	lastFailedAt time.Time
}

// New creates a Breaker that opens after threshold consecutive failures
// and remains open for cooldown before allowing a probe.
func New(threshold int, cooldown time.Duration) *Breaker {
	return &Breaker{
		threshold: threshold,
		cooldown:  cooldown,
	}
}

// Allow returns nil if the request should proceed, or ErrOpen if the breaker
// is open and not yet ready to probe.
func (b *Breaker) Allow() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	switch b.state {
	case Closed:
		return nil
	case Open:
		if time.Since(b.lastFailedAt) > b.cooldown {
			b.state = HalfOpen
			return nil // allow one probe
		}
		return ErrOpen
	case HalfOpen:
		return ErrOpen // only one probe at a time
	}
	return nil
}

// RecordSuccess resets the breaker to Closed and clears the failure count.
func (b *Breaker) RecordSuccess() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures = 0
	b.state = Closed
}

// RecordFailure increments the failure count. If failures reach the threshold
// the breaker trips to Open.
func (b *Breaker) RecordFailure() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures++
	b.lastFailedAt = time.Now()
	if b.failures >= b.threshold {
		b.state = Open
	}
}

// State returns the current state of the breaker.
func (b *Breaker) State() State {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.state
}
