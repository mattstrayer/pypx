package conda_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pypx/api/internal/conda"
)

func TestFetchCondaInfo_Available(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/package/conda-forge/numpy" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"name":"numpy","latest_version":"1.26.4"}`)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	c := conda.NewClient(conda.WithBaseURL(srv.URL))
	info, err := c.FetchCondaInfo(context.Background(), "numpy")
	if err != nil {
		t.Fatalf("FetchCondaInfo() error: %v", err)
	}
	if !info.Available {
		t.Error("Available should be true")
	}
	if info.Version != "1.26.4" {
		t.Errorf("Version = %q, want 1.26.4", info.Version)
	}
	if info.URL == "" {
		t.Error("URL should not be empty")
	}
}

func TestFetchCondaInfo_NotAvailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	c := conda.NewClient(conda.WithBaseURL(srv.URL))
	info, err := c.FetchCondaInfo(context.Background(), "some-obscure-package")
	if err != nil {
		t.Fatalf("FetchCondaInfo() error: %v", err)
	}
	if info.Available {
		t.Error("Available should be false for 404")
	}
}

func TestFetchCondaInfo_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := conda.NewClient(conda.WithBaseURL(srv.URL))
	_, err := c.FetchCondaInfo(context.Background(), "numpy")
	if err == nil {
		t.Error("expected error for 500 response, got nil")
	}
}
