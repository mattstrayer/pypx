package stats

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchOverall(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data": [{"category": "2024-01", "downloads": 50000000}, {"category": "2024-02", "downloads": 52000000}], "package": "requests", "type": "overall_downloads"}`))
	}))
	defer srv.Close()

	client := NewClient(WithBaseURL(srv.URL))
	resp, err := client.FetchOverall(context.Background(), "requests")
	if err != nil {
		t.Fatalf("FetchOverall returned error: %v", err)
	}

	if len(resp.Data) != 2 {
		t.Fatalf("expected 2 data points, got %d", len(resp.Data))
	}
	if resp.Data[0].Downloads != 50000000 {
		t.Errorf("expected first downloads=50000000, got %d", resp.Data[0].Downloads)
	}
}

func TestFetchPythonVersionStats(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data": [{"category": "3.12", "downloads": 30000000}, {"category": "3.11", "downloads": 20000000}], "package": "requests", "type": "python_minor_downloads"}`))
	}))
	defer srv.Close()

	client := NewClient(WithBaseURL(srv.URL))
	resp, err := client.FetchPythonVersions(context.Background(), "requests")
	if err != nil {
		t.Fatalf("FetchPythonVersions returned error: %v", err)
	}

	if len(resp.Data) != 2 {
		t.Fatalf("expected 2 data points, got %d", len(resp.Data))
	}
	if resp.Data[0].Category != "3.12" {
		t.Errorf("expected first category=3.12, got %q", resp.Data[0].Category)
	}
	if resp.Data[0].Downloads != 30000000 {
		t.Errorf("expected first downloads=30000000, got %d", resp.Data[0].Downloads)
	}
	if resp.Data[1].Downloads != 20000000 {
		t.Errorf("expected second downloads=20000000, got %d", resp.Data[1].Downloads)
	}
}
