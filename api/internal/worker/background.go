package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/pypx/api/internal/cache"
	"github.com/pypx/api/internal/pypi"
	"github.com/pypx/api/internal/search"
)

// Config holds configuration for the background worker.
type Config struct {
	SimpleAPIURL      string
	TopPackagesURL    string
	IndexSyncEvery    time.Duration
}

// Worker periodically syncs the PyPI Simple API index into the search index.
type Worker struct {
	pypi       *pypi.Client
	cache      cache.Cacher
	index      *search.Index
	httpClient *http.Client
	config     Config
	wg         sync.WaitGroup
}

// New creates a new Worker. Zero-value Config fields are filled with defaults:
// SimpleAPIURL defaults to "https://pypi.org/simple/", IndexSyncEvery defaults to 6h.
func New(pypiClient *pypi.Client, c cache.Cacher, idx *search.Index, cfg Config) *Worker {
	if cfg.SimpleAPIURL == "" {
		cfg.SimpleAPIURL = "https://pypi.org/simple/"
	}
	if cfg.TopPackagesURL == "" {
		cfg.TopPackagesURL = "https://hugovk.dev/top-pypi-packages/top-pypi-packages-30-days.min.json"
	}
	if cfg.IndexSyncEvery == 0 {
		cfg.IndexSyncEvery = 6 * time.Hour
	}
	return &Worker{
		pypi:       pypiClient,
		cache:      c,
		index:      idx,
		httpClient: &http.Client{Timeout: 5 * time.Minute},
		config:     cfg,
	}
}

// SyncIndex fetches the PyPI Simple API index (JSON format) and upserts all
// package names into the search index with empty summary and zero downloads.
func (w *Worker) SyncIndex(ctx context.Context) error {
	log.Printf("worker: starting index sync from %s", w.config.SimpleAPIURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, w.config.SimpleAPIURL, nil)
	if err != nil {
		return fmt.Errorf("worker: build request: %w", err)
	}
	// Request JSON format — much faster to parse than the 100MB+ HTML page.
	req.Header.Set("Accept", "application/vnd.pypi.simple.v1+json")

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("worker: fetch simple index: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("worker: simple index returned status %d", resp.StatusCode)
	}

	var index struct {
		Projects []struct {
			Name string `json:"name"`
		} `json:"projects"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&index); err != nil {
		return fmt.Errorf("worker: decode simple index: %w", err)
	}

	const batchSize = 5000
	batch := make([]search.PackageEntry, 0, batchSize)
	count := 0

	for _, p := range index.Projects {
		batch = append(batch, search.PackageEntry{
			Name:      p.Name,
			Summary:   "",
			Downloads: 0,
		})
		if len(batch) >= batchSize {
			if err := w.index.UpsertBatch(batch); err != nil {
				log.Printf("worker: batch upsert failed at %d: %v", count, err)
			} else {
				count += len(batch)
			}
			batch = batch[:0]
		}
	}
	// Flush remaining.
	if len(batch) > 0 {
		if err := w.index.UpsertBatch(batch); err != nil {
			log.Printf("worker: final batch upsert failed: %v", err)
		} else {
			count += len(batch)
		}
	}

	log.Printf("worker: index sync complete, upserted %d packages", count)
	return nil
}

// SyncDownloads fetches the top PyPI packages dataset and enriches the search
// index with 30-day download counts so results rank by popularity.
func (w *Worker) SyncDownloads(ctx context.Context) error {
	log.Printf("worker: starting downloads sync from %s", w.config.TopPackagesURL)

	dlCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(dlCtx, http.MethodGet, w.config.TopPackagesURL, nil)
	if err != nil {
		return fmt.Errorf("worker: build downloads request: %w", err)
	}

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("worker: fetch top packages: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("worker: top packages returned status %d", resp.StatusCode)
	}

	var data struct {
		Rows []struct {
			Project       string `json:"project"`
			DownloadCount int64  `json:"download_count"`
		} `json:"rows"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return fmt.Errorf("worker: decode top packages: %w", err)
	}

	const batchSize = 5000
	batch := make([]search.PackageEntry, 0, batchSize)
	count := 0

	for _, row := range data.Rows {
		batch = append(batch, search.PackageEntry{
			Name:      row.Project,
			Downloads: row.DownloadCount,
		})
		if len(batch) >= batchSize {
			if err := w.index.UpdateDownloadsBatch(batch); err != nil {
				log.Printf("worker: downloads batch update failed at %d: %v", count, err)
			} else {
				count += len(batch)
			}
			batch = batch[:0]
		}
	}
	if len(batch) > 0 {
		if err := w.index.UpdateDownloadsBatch(batch); err != nil {
			log.Printf("worker: downloads final batch failed: %v", err)
		} else {
			count += len(batch)
		}
	}

	log.Printf("worker: downloads sync complete, updated %d packages", count)
	return nil
}

// SyncTopSummaries fetches PyPI metadata for the top N packages by downloads
// and backfills summary text into packages_meta. The simple-index sync only
// captures package names, so without this step the popular endpoint returns
// empty summaries.
func (w *Worker) SyncTopSummaries(ctx context.Context, topN int) error {
	if topN <= 0 {
		topN = 50
	}
	log.Printf("worker: starting summary sync for top %d packages", topN)

	top, err := w.index.TopByDownloads(topN)
	if err != nil {
		return fmt.Errorf("worker: read top packages: %w", err)
	}
	if len(top) == 0 {
		log.Printf("worker: no top packages to sync summaries for (downloads not synced yet)")
		return nil
	}

	const concurrency = 5
	sem := make(chan struct{}, concurrency)
	var mu sync.Mutex
	updates := make([]search.PackageEntry, 0, len(top))
	var wg sync.WaitGroup

	for _, entry := range top {
		entry := entry
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()

			fetchCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
			defer cancel()

			resp, err := w.pypi.FetchPackage(fetchCtx, entry.Name)
			if err != nil {
				log.Printf("worker: summary fetch %q: %v", entry.Name, err)
				return
			}
			summary := resp.Info.Summary
			if summary == "" {
				return
			}
			mu.Lock()
			updates = append(updates, search.PackageEntry{Name: entry.Name, Summary: summary})
			mu.Unlock()
		}()
	}
	wg.Wait()

	if err := w.index.UpdateSummariesBatch(updates); err != nil {
		return fmt.Errorf("worker: update summaries: %w", err)
	}

	// Invalidate cached popular responses so the next request re-renders with
	// the freshly populated summaries instead of waiting out the 6h TTL.
	for limit := 1; limit <= 50; limit++ {
		if err := w.cache.Delete(fmt.Sprintf("popular:%d", limit)); err != nil {
			log.Printf("worker: invalidate popular:%d: %v", limit, err)
		}
	}

	log.Printf("worker: summary sync complete, updated %d packages", len(updates))
	return nil
}

// Start launches an initial SyncIndex in a goroutine and then re-syncs on
// every IndexSyncEvery tick. It returns immediately; use the context to stop.
// Call Wait after cancelling the context to ensure all in-flight DB writes complete.
func (w *Worker) Start(ctx context.Context) {
	w.wg.Add(2)

	go func() {
		defer w.wg.Done()
		if err := w.SyncIndex(ctx); err != nil {
			log.Printf("worker: initial sync error: %v", err)
		}
		// Run downloads sync after name sync so the meta rows exist.
		if err := w.SyncDownloads(ctx); err != nil {
			log.Printf("worker: initial downloads sync error: %v", err)
		}
		// Backfill summaries for top packages so the popular endpoint has them.
		if err := w.SyncTopSummaries(ctx, 50); err != nil {
			log.Printf("worker: initial summary sync error: %v", err)
		}
	}()

	go func() {
		defer w.wg.Done()
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
				if err := w.SyncDownloads(ctx); err != nil {
					log.Printf("worker: periodic downloads sync error: %v", err)
				}
				if err := w.SyncTopSummaries(ctx, 50); err != nil {
					log.Printf("worker: periodic summary sync error: %v", err)
				}
			}
		}
	}()
}

// Wait blocks until all goroutines started by Start have returned.
// Call this after cancelling the worker context to ensure in-flight DB writes complete.
func (w *Worker) Wait() {
	w.wg.Wait()
}
