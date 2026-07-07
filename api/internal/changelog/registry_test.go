package changelog_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pypx/api/internal/changelog"
)

type stubSource struct {
	name    string
	entries []changelog.Entry
	err     error
	delay   time.Duration
	// gotCtxErr, when non-nil, receives ctx.Err() if Fetch is cancelled during
	// its delay. Buffered (cap 1) so the send never blocks.
	gotCtxErr chan error
}

func (s *stubSource) Name() string { return s.name }
func (s *stubSource) Fetch(ctx context.Context) ([]changelog.Entry, error) {
	if s.delay > 0 {
		select {
		case <-time.After(s.delay):
		case <-ctx.Done():
			if s.gotCtxErr != nil {
				s.gotCtxErr <- ctx.Err()
			}
			return nil, ctx.Err()
		}
	}
	return s.entries, s.err
}

func TestRegistry_ReturnsHighestPriorityNonEmpty(t *testing.T) {
	sources := []changelog.Source{
		&stubSource{name: "empty", entries: nil},
		&stubSource{name: "has_data", entries: []changelog.Entry{{Version: "1.0.0"}}},
		&stubSource{name: "also_data", entries: []changelog.Entry{{Version: "2.0.0"}}},
	}
	reg := changelog.NewRegistry(sources...)
	result := reg.Fetch(context.Background())

	if result.Source != "has_data" {
		t.Errorf("result.Source = %q, want has_data", result.Source)
	}
	if len(result.Entries) != 1 || result.Entries[0].Version != "1.0.0" {
		t.Errorf("unexpected entries: %+v", result.Entries)
	}
}

func TestRegistry_HigherPriorityWinsEvenIfSlower(t *testing.T) {
	sources := []changelog.Source{
		&stubSource{name: "slow_winner", entries: []changelog.Entry{{Version: "1.0.0"}}, delay: 50 * time.Millisecond},
		&stubSource{name: "fast_loser", entries: []changelog.Entry{{Version: "2.0.0"}}},
	}
	reg := changelog.NewRegistry(sources...)
	result := reg.Fetch(context.Background())

	if result.Source != "slow_winner" {
		t.Errorf("result.Source = %q, want slow_winner", result.Source)
	}
}

func TestRegistry_AllEmpty_ReturnsNone(t *testing.T) {
	sources := []changelog.Source{
		&stubSource{name: "a", entries: nil},
		&stubSource{name: "b", entries: nil},
	}
	reg := changelog.NewRegistry(sources...)
	result := reg.Fetch(context.Background())

	if result.Source != "none" {
		t.Errorf("result.Source = %q, want none", result.Source)
	}
	if len(result.Entries) != 0 {
		t.Errorf("expected empty entries, got %+v", result.Entries)
	}
}

func TestRegistry_ErrorSourceSkipped(t *testing.T) {
	sources := []changelog.Source{
		&stubSource{name: "erroring", err: errors.New("network failure")},
		&stubSource{name: "good", entries: []changelog.Entry{{Version: "1.0.0"}}},
	}
	reg := changelog.NewRegistry(sources...)
	result := reg.Fetch(context.Background())

	if result.Source != "good" {
		t.Errorf("result.Source = %q, want good", result.Source)
	}
}

func TestRegistry_NoSources_ReturnsNone(t *testing.T) {
	reg := changelog.NewRegistry()
	result := reg.Fetch(context.Background())
	if result.Source != "none" {
		t.Errorf("result.Source = %q, want none", result.Source)
	}
}

func TestRegistry_CancelsLosersOnWin(t *testing.T) {
	slowLoser := &stubSource{
		name:      "slow_loser",
		entries:   []changelog.Entry{{Version: "2.0.0"}},
		delay:     5 * time.Second,
		gotCtxErr: make(chan error, 1),
	}
	sources := []changelog.Source{
		&stubSource{name: "winner", entries: []changelog.Entry{{Version: "1.0.0"}}, delay: 10 * time.Millisecond},
		slowLoser,
	}
	reg := changelog.NewRegistry(sources...)

	start := time.Now()
	result := reg.Fetch(context.Background())
	elapsed := time.Since(start)

	if result.Source != "winner" {
		t.Errorf("result.Source = %q, want winner", result.Source)
	}
	if elapsed > time.Second {
		t.Errorf("fetch took %v, expected it not to wait for the 5s loser", elapsed)
	}

	select {
	case err := <-slowLoser.gotCtxErr:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("loser observed ctx error %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("slow loser was not cancelled after winner resolved")
	}
}

func TestRegistry_ReturnsEarlyWithoutWaitingForLosers(t *testing.T) {
	sources := []changelog.Source{
		&stubSource{name: "winner", entries: []changelog.Entry{{Version: "1.0.0"}}},
		&stubSource{name: "slow_a", entries: []changelog.Entry{{Version: "2.0.0"}}, delay: 5 * time.Second},
		&stubSource{name: "slow_b", entries: []changelog.Entry{{Version: "3.0.0"}}, delay: 5 * time.Second},
	}
	reg := changelog.NewRegistry(sources...)

	start := time.Now()
	result := reg.Fetch(context.Background())
	elapsed := time.Since(start)

	if result.Source != "winner" {
		t.Errorf("result.Source = %q, want winner", result.Source)
	}
	if elapsed > time.Second {
		t.Errorf("fetch took %v, expected early return well under 1s", elapsed)
	}
}

func TestRegistry_SlowHighPriorityStillWins_FastLoserNotReturned(t *testing.T) {
	sources := []changelog.Source{
		&stubSource{name: "slow_winner", entries: []changelog.Entry{{Version: "1.0.0"}}, delay: 100 * time.Millisecond},
		&stubSource{name: "fast_loser", entries: []changelog.Entry{{Version: "2.0.0"}}, gotCtxErr: make(chan error, 1)},
	}
	reg := changelog.NewRegistry(sources...)

	start := time.Now()
	result := reg.Fetch(context.Background())
	elapsed := time.Since(start)

	if result.Source != "slow_winner" {
		t.Errorf("result.Source = %q, want slow_winner", result.Source)
	}
	if elapsed < 90*time.Millisecond {
		t.Errorf("fetch took %v, expected it to wait for the slow priority-0 source (~100ms)", elapsed)
	}
}

func TestRegistry_RunsInParallel(t *testing.T) {
	sources := []changelog.Source{
		&stubSource{name: "a", entries: nil, delay: 100 * time.Millisecond},
		&stubSource{name: "b", entries: nil, delay: 100 * time.Millisecond},
		&stubSource{name: "c", entries: []changelog.Entry{{Version: "1.0.0"}}, delay: 100 * time.Millisecond},
	}
	reg := changelog.NewRegistry(sources...)
	start := time.Now()
	result := reg.Fetch(context.Background())
	elapsed := time.Since(start)

	if elapsed > 250*time.Millisecond {
		t.Errorf("fetch took %v, expected parallel execution (~100ms)", elapsed)
	}
	if result.Source != "c" {
		t.Errorf("result.Source = %q, want c", result.Source)
	}
}
