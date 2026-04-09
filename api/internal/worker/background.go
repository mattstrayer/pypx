package worker

import (
	"context"
	"fmt"
	"io"
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
	cache  *cache.Cache
	index  *search.Index
	config Config
}

var packageNameRe = regexp.MustCompile(`href="/simple/([^/]+)/"`)

// New creates a new Worker. Zero-value Config fields are filled with defaults:
// SimpleAPIURL defaults to "https://pypi.org/simple/", IndexSyncEvery defaults to 6h.
func New(pypiClient *pypi.Client, c *cache.Cache, idx *search.Index, cfg Config) *Worker {
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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("worker: read response body: %w", err)
	}

	matches := packageNameRe.FindAllSubmatch(body, -1)
	log.Printf("worker: syncing %d packages from Simple API", len(matches))

	for i, m := range matches {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("worker: context cancelled after %d packages: %w", i, err)
		}
		name := string(m[1])
		if err := w.index.Upsert(search.PackageEntry{
			Name:      name,
			Summary:   "",
			Downloads: 0,
		}); err != nil {
			log.Printf("worker: upsert %q failed: %v", name, err)
		}
	}

	log.Printf("worker: index sync complete, upserted %d packages", len(matches))
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
