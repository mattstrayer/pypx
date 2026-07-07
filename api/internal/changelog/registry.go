package changelog

import (
	"context"
	"errors"
	"log"
)

// Registry fires all sources concurrently and returns the highest-priority
// (lowest index) non-empty result.
type Registry struct {
	sources []Source
}

// NewRegistry constructs a Registry. Sources are provided in priority order:
// index 0 is highest priority.
func NewRegistry(sources ...Source) *Registry {
	return &Registry{sources: sources}
}

type outcome struct {
	priority int
	result   Result
	err      error
}

// Fetch runs all sources in parallel and resolves the winner incrementally:
// as each source finishes it scans priorities from highest (index 0) upward,
// returning as soon as the highest-priority source that could still win has
// produced non-empty entries. Once a winner is found, the deferred cancel stops
// any still-running lower-priority sources (they can no longer win), which frees
// GitHub/GitLab rate-limit budget and bounds latency by the winner, not the
// slowest source.
//
// Correctness: a source at index i is returned only when every index < i has
// already resolved to empty-or-error. Every still-running source therefore has
// an index > i and cannot beat i, so cancellation only ever hits guaranteed
// losers. This preserves the invariant that a lower index never loses to a
// higher one — in particular Fetch still waits for a slow priority-0 source
// before returning a fast priority-1 result.
//
// Never returns an error — individual source errors are logged/dropped.
func (r *Registry) Fetch(ctx context.Context) Result {
	if len(r.sources) == 0 {
		return Result{Source: "none"}
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Buffered to len(sources) so a goroutine can always send its outcome and
	// exit even after Fetch has returned early — no blocked/leaked goroutines.
	ch := make(chan outcome, len(r.sources))

	for i, src := range r.sources {
		go func(priority int, s Source) {
			entries, err := s.Fetch(ctx)
			// Suppress spurious logs from losers we cancelled after a winner.
			if err != nil && !errors.Is(err, context.Canceled) {
				log.Printf("changelog: source %T failed: %v", s, err)
			}
			ch <- outcome{priority: priority, result: Result{Source: s.Name(), Entries: entries}, err: err}
		}(i, src)
	}

	resolved := make([]*outcome, len(r.sources))
	for range r.sources {
		o := <-ch
		resolved[o.priority] = &o

		// Decision scan: walk from highest priority upward. Stop at the first
		// unresolved index (that source could still win — keep waiting). Return
		// the first resolved index with non-empty entries (it is the winner).
		decided := true
		for i := range resolved {
			if resolved[i] == nil {
				decided = false
				break
			}
			if resolved[i].err == nil && len(resolved[i].result.Entries) > 0 {
				return resolved[i].result
			}
		}
		if decided {
			// All sources resolved, none produced entries.
			return Result{Source: "none"}
		}
	}
	return Result{Source: "none"}
}
