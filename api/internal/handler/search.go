package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/pypx/api/internal/search"
)

// SearchHandler handles package search requests backed by an FTS5 index.
type SearchHandler struct {
	index *search.Index
}

// NewSearchHandler creates a SearchHandler using the provided index.
func NewSearchHandler(idx *search.Index) *SearchHandler {
	return &SearchHandler{index: idx}
}

// Search handles GET /api/search?q=<query>&limit=<n>.
func (h *SearchHandler) Search(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		http.Error(w, "query parameter 'q' is required", http.StatusBadRequest)
		return
	}

	limit := 20
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			limit = n
		}
	}
	if limit > 100 {
		limit = 100
	}
	if limit <= 0 {
		limit = 20
	}

	results, err := h.index.Search(query, limit)
	if err != nil {
		http.Error(w, "search failed", http.StatusInternalServerError)
		return
	}

	// Return empty array instead of null when there are no results.
	if results == nil {
		results = []search.PackageEntry{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results) //nolint:errcheck
}
