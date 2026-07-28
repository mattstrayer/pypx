package middleware

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"testing"

	"github.com/go-chi/chi/v5"
)

func newTestRouter() *chi.Mux {
	r := chi.NewRouter()
	r.Use(NegotiateText)
	r.Get("/api/packages/{name}", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("json")) })
	r.Get("/api/packages/{name}.txt", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("text")) })
	r.Get("/api/packages/{name}/docs/{symbol}", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("json")) })
	r.Get("/api/health", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("json")) })
	r.Post("/api/packages/{name}", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("json")) })
	return r
}

func TestNegotiateText(t *testing.T) {
	r := newTestRouter()

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
		// No .txt twin for /docs/{symbol} — must never be rewritten, even
		// though the handler here would happily return "json" either way;
		// this pins the route-table contract, not just the response body.
		{http.MethodGet, "text/plain", "/api/packages/httpx/docs/Client", "json"},
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
		if !slices.Contains(rec.Header().Values("Vary"), "Accept") {
			t.Errorf("method=%s Accept=%q path=%q missing Vary: Accept (got %v)", c.method, c.accept, c.path, rec.Header().Values("Vary"))
		}
	}
}

// TestNegotiateTextDocsSymbolNotRewritten pins IMPORTANT-1: there must be no
// hidden route match for the JSON-only /docs/{symbol} route.
func TestNegotiateTextDocsSymbolNotRewritten(t *testing.T) {
	r := newTestRouter()
	req := httptest.NewRequest(http.MethodGet, "/api/packages/httpx/docs/Client", nil)
	req.Header.Set("Accept", "text/plain")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Body.String() != "json" {
		t.Errorf("Accept=text/plain /docs/{symbol} → %q, want %q (no .txt twin exists)", rec.Body.String(), "json")
	}
}

// TestNegotiateTextPercentEncodedPath pins IMPORTANT-2: chi routes on
// URL.RawPath when non-empty, so a percent-encoded name (e.g. "pytest%2Dcov")
// must still be rewritten — mutating only Path silently drops the rewrite.
func TestNegotiateTextPercentEncodedPath(t *testing.T) {
	r := newTestRouter()
	req := httptest.NewRequest(http.MethodGet, "/api/packages/pytest%2Dcov", nil)
	req.Header.Set("Accept", "text/plain")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Body.String() != "text" {
		t.Errorf("Accept=text/plain path with percent-encoding → %q, want %q", rec.Body.String(), "text")
	}
}

// TestNegotiateTextPreservesQuery ensures the rewrite only touches the path,
// never dropping the query string.
func TestNegotiateTextPreservesQuery(t *testing.T) {
	r := chi.NewRouter()
	r.Use(NegotiateText)
	r.Get("/api/search", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("json:" + r.URL.RawQuery))
	})
	r.Get("/api/search.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("text:" + r.URL.RawQuery))
	})

	req := httptest.NewRequest(http.MethodGet, "/api/search?"+url.Values{"q": {"flask"}}.Encode(), nil)
	req.Header.Set("Accept", "text/plain")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if want := "text:q=flask"; rec.Body.String() != want {
		t.Errorf("query string not preserved across rewrite: got %q, want %q", rec.Body.String(), want)
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
		// application/* breadth is deliberate: any application/ subtype is a
		// JSON vote, so this documents the current behavior — JSON wins.
		{"application/pdf, text/plain;q=0.5", false},
		// q > 1 is out of range and must clamp to 1, not out-vote a
		// legitimate q=1 JSON preference — becomes a tie → JSON.
		{"text/plain;q=2, application/json", false},
		// q=0 means "not acceptable" (RFC 7231) — must not count as text.
		{"text/plain;q=0", false},
		{"text/plain;q=0, application/json", false},
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
		// No .txt twin: main.go registers only the JSON /docs/{symbol} route.
		{"/api/packages/httpx/docs/Client", false},
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
