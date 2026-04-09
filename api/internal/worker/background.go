package worker

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"time"

	"github.com/pypx/api/internal/cache"
	"github.com/pypx/api/internal/pypi"
	"github.com/pypx/api/internal/search"
)

// Config holds configuration for the background worker.
type Config struct {
	SimpleAPIURL   string
	IndexSyncEvery time.Duration
}

// Worker periodically syncs the PyPI Simple API index into the search index.
type Worker struct {
	pypi   *pypi.Client
	cache  cache.Cacher
	index  *search.Index
	config Config
}

var packageNameRe = regexp.MustCompile(`href="/simple/([^/]+)/"`)

// New creates a new Worker. Zero-value Config fields are filled with defaults:
// SimpleAPIURL defaults to "https://pypi.org/simple/", IndexSyncEvery defaults to 6h.
func New(pypiClient *pypi.Client, c cache.Cacher, idx *search.Index, cfg Config) *Worker {
	if cfg.SimpleAPIURL == "" {
		cfg.SimpleAPIURL = "https://pypi.org/simple/"
	}
	if cfg.IndexSyncEvery == 0 {
		cfg.IndexSyncEvery = 6 * time.Hour
	}
	return &Worker{
		pypi:   pypiClient,
		cache:  c,
		index:  idx,
		config: cfg,
	}
}

// SyncIndex fetches the PyPI Simple API index and upserts all package names
// into the search index with empty summary and zero downloads.
func (w *Worker) SyncIndex(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, w.config.SimpleAPIURL, nil)
	if err != nil {
		return fmt.Errorf("worker: build request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("worker: fetch simple index: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("worker: simple index returned status %d", resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)
	// The PyPI Simple index can have very long lines; use a 1 MiB buffer so
	// we don't hit the default 64 KiB limit while still keeping per-line
	// memory usage constant (as opposed to io.ReadAll on the 100 MB+ body).
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	count := 0
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("worker: context cancelled after %d packages: %w", count, err)
		}
		line := scanner.Bytes()
		matches := packageNameRe.FindAllSubmatch(line, -1)
		for _, m := range matches {
			name := string(m[1])
			if err := w.index.Upsert(search.PackageEntry{
				Name:      name,
				Summary:   "",
				Downloads: 0,
			}); err != nil {
				log.Printf("worker: upsert %q failed: %v", name, err)
			} else {
				count++
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("worker: reading response body: %w", err)
	}

	log.Printf("worker: index sync complete, upserted %d packages", count)
	return nil
}

// Start launches an initial SyncIndex in a goroutine and then re-syncs on
// every IndexSyncEvery tick. It returns immediately; use the context to stop.
func (w *Worker) Start(ctx context.Context) {
	go func() {
		if err := w.SyncIndex(ctx); err != nil {
			log.Printf("worker: initial sync error: %v", err)
		}
	}()

	go func() {
		ticker := time.NewTicker(w.config.IndexSyncEvery)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := w.SyncIndex(ctx); err != nil {
					log.Printf("worker: periodic sync error: %v", err)
				}
			}
		}
	}()
}
