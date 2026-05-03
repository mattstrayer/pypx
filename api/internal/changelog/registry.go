package changelog

import (
	"context"
	"log"
	"sort"
	"sync"
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

// Fetch runs all sources in parallel, waits for all to finish, then returns
// the highest-priority source that produced non-empty entries.
// Never returns an error — individual source errors are logged/dropped.
func (r *Registry) Fetch(ctx context.Context) Result {
	if len(r.sources) == 0 {
		return Result{Source: "none"}
	}

	ch := make(chan outcome, len(r.sources))
	var wg sync.WaitGroup

	for i, src := range r.sources {
		wg.Add(1)
		go func(priority int, s Source) {
			defer wg.Done()
			entries, err := s.Fetch(ctx)
			if err != nil {
				log.Printf("changelog: source %T failed: %v", s, err)
			}
			ch <- outcome{priority: priority, result: Result{Source: s.Name(), Entries: entries}, err: err}
		}(i, src)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	outcomes := make([]outcome, 0, len(r.sources))
	for o := range ch {
		outcomes = append(outcomes, o)
	}

	sort.Slice(outcomes, func(i, j int) bool {
		return outcomes[i].priority < outcomes[j].priority
	})

	for _, o := range outcomes {
		if o.err == nil && len(o.result.Entries) > 0 {
			return o.result
		}
	}
	return Result{Source: "none"}
}
