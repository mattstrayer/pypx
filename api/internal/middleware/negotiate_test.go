package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestNegotiateText(t *testing.T) {
	r := chi.NewRouter()
	r.Use(NegotiateText)
	r.Get("/api/packages/{name}", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("json")) })
	r.Get("/api/packages/{name}.txt", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("text")) })
	r.Get("/api/health", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("json")) })
	r.Post("/api/packages/{name}", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("json")) })

	cases := []struct {
		method, accept, path, want string
	}{
		{http.MethodGet, "", "/api/packages/httpx", "json"},
		{http.MethodGet, "*/*", "/api/packages/httpx", "json"},
		{http.MethodGet, "application/json", "/api/packages/httpx", "json"},
		{http.MethodGet, "text/plain", "/api/packages/httpx", "text"},
		{http.MethodGet, "text/markdown", "/api/packages/httpx", "text"},
		{http.MethodGet, "text/plain, application/json;q=0.5", "/api/packages/httpx", "text"},
		{http.MethodGet, "application/json, text/plain;q=0.5", "/api/packages/httpx", "json"},
		{http.MethodGet, "text/html", "/api/packages/httpx", "json"},
		{http.MethodGet, "text/plain", "/api/health", "json"},
		{http.MethodGet, "text/plain", "/api/packages/httpx.txt", "text"},
		{http.MethodPost, "text/plain", "/api/packages/httpx", "json"},
	}
	for _, c := range cases {
		req := httptest.NewRequest(c.method, c.path, nil)
		if c.accept != "" {
			req.Header.Set("Accept", c.accept)
		}
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Body.String() != c.want {
			t.Errorf("method=%s Accept=%q path=%q → %q, want %q", c.method, c.accept, c.path, rec.Body.String(), c.want)
		}
		if rec.Header().Get("Vary") != "Accept" {
			t.Errorf("method=%s Accept=%q path=%q missing Vary: Accept", c.method, c.accept, c.path)
		}
	}
}

func TestPrefersText(t *testing.T) {
	cases := []struct {
		accept string
		want   bool
	}{
		{"", false},
		{"*/*", false},
		{"application/json", false},
		{"text/plain", true},
		{"text/markdown", true},
		{"text/html", false},
		{"text/*", true},
		{"application/*", false},
		{"text/plain, application/json;q=0.5", true},
		{"application/json, text/plain;q=0.5", false},
		{"  text/plain  ,  application/json ; q = 0.9 ", true},
		{"TEXT/PLAIN", true},
		{"text/plain;q=abc", true},
		{"text/plain;q=0.5, application/json;q=0.5", false},
		{"text/plain;charset=utf-8", true},
		{"application/json;q=1.0, text/plain;q=1", false},
	}
	for _, c := range cases {
		got := prefersText(c.accept)
		if got != c.want {
			t.Errorf("prefersText(%q) = %v, want %v", c.accept, got, c.want)
		}
	}
}

func TestHasTxtTwin(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"/api/packages/httpx", true},
		{"/api/packages/httpx/changelog", true},
		{"/api/packages/httpx/security", true},
		{"/api/packages/httpx/extras", true},
		{"/api/packages/httpx/stats", true},
		{"/api/packages/httpx/docs", true},
		{"/api/packages/httpx/diff", true},
		{"/api/packages/httpx/docs/Client", true},
		{"/api/search", true},
		{"/api/compare", true},
		{"/api/popular", true},
		{"/api/health", false},
		{"/api/packages/httpx/versions", false},
		{"/api/packages/httpx/dependencies", false},
		{"/api/packages/httpx.txt", false},
	}
	for _, c := range cases {
		got := hasTxtTwin(c.path)
		if got != c.want {
			t.Errorf("hasTxtTwin(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}
