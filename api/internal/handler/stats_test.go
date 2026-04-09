package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/pypx/api/internal/cache"
	"github.com/pypx/api/internal/handler"
	"github.com/pypx/api/internal/stats"
)

const mockOverallResponse = `{
	"package": "requests",
	"type": "overall",
	"data": [
		{"category": "with_mirrors", "downloads": 1000000},
		{"category": "without_mirrors", "downloads": 900000}
	]
}`

func TestGetStats(t *testing.T) {
	// Mock pypistats server returning overall data for all endpoints.
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockOverallResponse)) //nolint:errcheck
	}))
	defer mock.Close()

	c, err := cache.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer c.Close()

	statsClient := stats.NewClient(stats.WithBaseURL(mock.URL))
	h := handler.NewStatsHandler(statsClient, c)

	r := chi.NewRouter()
	r.Get("/api/packages/{name}/stats", h.Get)

	req := httptest.NewRequest(http.MethodGet, "/api/packages/requests/stats", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var body map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if got := body["package"]; got != "requests" {
		t.Errorf("expected package=requests, got %v", got)
	}
}
