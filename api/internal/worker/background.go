package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
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

// Start launches an initial SyncIndex in a goroutine and then re-syncs on
// every IndexSyncEvery tick. It returns immediately; use the context to stop.
func (w *Worker) Start(ctx context.Context) {
	go func() {
		if err := w.SyncIndex(ctx); err != nil {
			log.Printf("worker: initial sync error: %v", err)
		}
		// Run downloads sync after name sync so the meta rows exist.
		if err := w.SyncDownloads(ctx); err != nil {
			log.Printf("worker: initial downloads sync error: %v", err)
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
				if err := w.SyncDownloads(ctx); err != nil {
					log.Printf("worker: periodic downloads sync error: %v", err)
				}
			}
		}
	}()
}
