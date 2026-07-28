package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestAPIRoot(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api", nil)
	rec := httptest.NewRecorder()
	APIRoot(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("content-type = %q, want application/json", ct)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["llms"] != "/llms.txt" {
		t.Errorf("llms = %q, want /llms.txt", body["llms"])
	}
	if body["source"] == "" || body["description"] == "" {
		t.Errorf("missing source/description keys: %v", body)
	}
}

// TestAPIRootRouting mounts both the bare "/api" and trailing-slash "/api/"
// routes on a chi router, mirroring main.go's registration, to prove that
// Caddy's "/api" exact-path proxy target (which forwards the request
// unmodified) reaches the handler either way requests are shaped.
func TestAPIRootRouting(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/api", APIRoot)
	r.Get("/api/", APIRoot)

	srv := httptest.NewServer(r)
	defer srv.Close()

	for _, path := range []string{"/api", "/api/"} {
		t.Run(path, func(t *testing.T) {
			resp, err := http.Get(srv.URL + path) //nolint:noctx
			if err != nil {
				t.Fatalf("GET %s: %v", path, err)
			}
			defer resp.Body.Close() //nolint:errcheck

			if resp.StatusCode != http.StatusOK {
				t.Fatalf("%s status = %d, want 200", path, resp.StatusCode)
			}

			var body map[string]string
			if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
				t.Fatalf("%s invalid JSON: %v", path, err)
			}
			if body["llms"] != "/llms.txt" {
				t.Errorf("%s llms = %q, want /llms.txt", path, body["llms"])
			}
		})
	}
}
