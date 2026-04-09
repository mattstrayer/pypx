package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pypx/api/internal/handler"
	"github.com/pypx/api/internal/search"
)

func TestSearch(t *testing.T) {
	idx, err := search.NewIndex(":memory:")
	if err != nil {
		t.Fatalf("failed to create index: %v", err)
	}
	defer idx.Close()

	if err := idx.Upsert(search.PackageEntry{Name: "requests", Summary: "HTTP for humans", Downloads: 1000}); err != nil {
		t.Fatalf("upsert requests: %v", err)
	}
	if err := idx.Upsert(search.PackageEntry{Name: "flask", Summary: "A simple web framework", Downloads: 500}); err != nil {
		t.Fatalf("upsert flask: %v", err)
	}

	h := handler.NewSearchHandler(idx)

	req := httptest.NewRequest(http.MethodGet, "/api/search?q=requests", nil)
	w := httptest.NewRecorder()
	h.Search(w, req)

	res := w.Result()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}

	var results []search.PackageEntry
	if err := json.NewDecoder(res.Body).Decode(&results); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Name != "requests" {
		t.Errorf("expected name 'requests', got %q", results[0].Name)
	}
}

func TestSearchEmpty(t *testing.T) {
	idx, err := search.NewIndex(":memory:")
	if err != nil {
		t.Fatalf("failed to create index: %v", err)
	}
	defer idx.Close()

	h := handler.NewSearchHandler(idx)

	req := httptest.NewRequest(http.MethodGet, "/api/search?q=", nil)
	w := httptest.NewRecorder()
	h.Search(w, req)

	res := w.Result()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", res.StatusCode)
	}
}
