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

func TestSearchHandlerSearchText(t *testing.T) {
	idx, err := search.NewIndex(":memory:")
	if err != nil {
		t.Fatalf("failed to create index: %v", err)
	}
	defer idx.Close()

	if err := idx.Upsert(search.PackageEntry{Name: "httpx", Summary: "The next generation HTTP client.", Downloads: 50_000_000}); err != nil {
		t.Fatalf("upsert httpx: %v", err)
	}
	if err := idx.Upsert(search.PackageEntry{Name: "httpcore", Summary: "A minimal low-level HTTP client.", Downloads: 40_000_000}); err != nil {
		t.Fatalf("upsert httpcore: %v", err)
	}

	h := handler.NewSearchHandler(idx)

	req := httptest.NewRequest(http.MethodGet, "/api/search.txt?q=http", nil)
	w := httptest.NewRecorder()
	h.SearchText(w, req)

	res := w.Result()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}

	contentType := res.Header.Get("Content-Type")
	if contentType != "text/plain; charset=utf-8" {
		t.Errorf("expected content-type 'text/plain; charset=utf-8', got %q", contentType)
	}

	body := make([]byte, 1024)
	n, err := res.Body.Read(body)
	if err != nil && err.Error() != "EOF" {
		t.Fatalf("read body: %v", err)
	}
	bodyStr := string(body[:n])

	expectedHeader := "# name\tdownloads\tsummary\n"
	if !starts(bodyStr, expectedHeader) {
		t.Errorf("body does not start with expected header.\nGot: %q\nExpected to start with: %q", bodyStr, expectedHeader)
	}
}

func starts(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
