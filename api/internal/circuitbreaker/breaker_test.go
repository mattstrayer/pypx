package circuitbreaker_test

import (
	"errors"
	"testing"
	"time"

	"github.com/pypx/api/internal/circuitbreaker"
)

func TestAllowedWhenClosed(t *testing.T) {
	b := circuitbreaker.New(3, 30*time.Second)
	if err := b.Allow(); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if b.State() != circuitbreaker.Closed {
		t.Fatalf("expected Closed, got %v", b.State())
	}
}

func TestTripsToOpenAfterThresholdFailures(t *testing.T) {
	b := circuitbreaker.New(3, 30*time.Second)

	b.RecordFailure()
	b.RecordFailure()
	if b.State() != circuitbreaker.Closed {
		t.Fatal("expected Closed before threshold")
	}

	b.RecordFailure() // 3rd failure hits threshold
	if b.State() != circuitbreaker.Open {
		t.Fatalf("expected Open after threshold, got %v", b.State())
	}
}

func TestRejectsWhenOpen(t *testing.T) {
	b := circuitbreaker.New(1, 30*time.Second)
	b.RecordFailure()

	if b.State() != circuitbreaker.Open {
		t.Fatal("expected Open")
	}
	err := b.Allow()
	if !errors.Is(err, circuitbreaker.ErrOpen) {
		t.Fatalf("expected ErrOpen, got %v", err)
	}
}

func TestHalfOpenAfterCooldown(t *testing.T) {
	b := circuitbreaker.New(1, 10*time.Millisecond)
	b.RecordFailure()

	// Still open immediately
	if err := b.Allow(); !errors.Is(err, circuitbreaker.ErrOpen) {
		t.Fatalf("expected ErrOpen immediately after trip, got %v", err)
	}

	time.Sleep(20 * time.Millisecond)

	// After cooldown one probe is allowed
	if err := b.Allow(); err != nil {
		t.Fatalf("expected probe to be allowed after cooldown, got %v", err)
	}
	if b.State() != circuitbreaker.HalfOpen {
		t.Fatalf("expected HalfOpen, got %v", b.State())
	}
}

func TestSuccessfulProbeResetsToClose(t *testing.T) {
	b := circuitbreaker.New(1, 10*time.Millisecond)
	b.RecordFailure()
	time.Sleep(20 * time.Millisecond)
	_ = b.Allow() // enter HalfOpen

	b.RecordSuccess()
	if b.State() != circuitbreaker.Closed {
		t.Fatalf("expected Closed after successful probe, got %v", b.State())
	}
	// Subsequent requests should succeed
	if err := b.Allow(); err != nil {
		t.Fatalf("expected nil after reset, got %v", err)
	}
}

func TestFailedProbeReOpens(t *testing.T) {
	b := circuitbreaker.New(1, 10*time.Millisecond)
	b.RecordFailure()
	time.Sleep(20 * time.Millisecond)
	_ = b.Allow() // enter HalfOpen

	b.RecordFailure() // probe fails
	if b.State() != circuitbreaker.Open {
		t.Fatalf("expected Open after failed probe, got %v", b.State())
	}
}

func TestOnlyOneProbeAllowedInHalfOpen(t *testing.T) {
	b := circuitbreaker.New(1, 10*time.Millisecond)
	b.RecordFailure()
	time.Sleep(20 * time.Millisecond)

	_ = b.Allow() // first Allow transitions to HalfOpen and returns nil
	err := b.Allow() // second Allow should be rejected
	if !errors.Is(err, circuitbreaker.ErrOpen) {
		t.Fatalf("expected ErrOpen for second probe attempt, got %v", err)
	}
}

func TestSuccessResetsFailureCount(t *testing.T) {
	b := circuitbreaker.New(3, 30*time.Second)

	b.RecordFailure()
	b.RecordFailure()
	b.RecordSuccess() // reset
	b.RecordFailure()
	b.RecordFailure()

	// Only 2 failures since reset — should still be Closed
	if b.State() != circuitbreaker.Closed {
		t.Fatalf("expected Closed (failure count was reset), got %v", b.State())
	}
}
