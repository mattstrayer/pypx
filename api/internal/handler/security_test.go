package handler_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/pypx/api/internal/cache"
	"github.com/pypx/api/internal/handler"
	"github.com/pypx/api/internal/osv"
)

func TestSecurityHandlerGet(t *testing.T) {
	osvSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"vulns":[{"id":"GHSA-test","summary":"Test vuln","severity":[{"type":"CVSS_V3","score":"HIGH"}],"references":[{"url":"https://example.com"}]}]}`)
	}))
	defer osvSrv.Close()

	c, err := cache.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer c.Close()
	memCache := cache.NewMemoryCache(c, 100)

	osvClient := osv.NewClient(osv.WithBaseURL(osvSrv.URL))
	h := handler.NewSecurityHandler(osvClient, memCache)

	r := chi.NewRouter()
	r.Get("/api/packages/{name}/security", h.Get)

	req := httptest.NewRequest(http.MethodGet, "/api/packages/requests/security", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp handler.SecurityResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Package != "requests" {
		t.Errorf("package = %q, want requests", resp.Package)
	}
	if len(resp.Vulns) != 1 {
		t.Errorf("vulns count = %d, want 1", len(resp.Vulns))
	}
}

func TestSecurityHandlerInvalidName(t *testing.T) {
	c, err := cache.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer c.Close()
	memCache := cache.NewMemoryCache(c, 100)

	osvClient := osv.NewClient()
	h := handler.NewSecurityHandler(osvClient, memCache)

	r := chi.NewRouter()
	r.Get("/api/packages/{name}/security", h.Get)

	req := httptest.NewRequest(http.MethodGet, "/api/packages/../etc/security", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code == http.StatusOK {
		t.Error("expected non-200 for invalid package name")
	}
}

func TestSecurityHandlerGetText(t *testing.T) {
	osvSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"vulns":[{"id":"GHSA-test","summary":"Test vuln","severity":[{"type":"CVSS_V3","score":"HIGH"}],"references":[{"url":"https://example.com"}]}]}`)
	}))
	defer osvSrv.Close()

	c, err := cache.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer c.Close()
	memCache := cache.NewMemoryCache(c, 100)

	osvClient := osv.NewClient(osv.WithBaseURL(osvSrv.URL))
	h := handler.NewSecurityHandler(osvClient, memCache)

	r := chi.NewRouter()
	r.Get("/api/packages/{name}/security.txt", h.GetText)

	req := httptest.NewRequest(http.MethodGet, "/api/packages/httpx/security.txt", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if !strings.HasPrefix(contentType, "text/plain") {
		t.Errorf("Content-Type = %q, want text/plain*", contentType)
	}

	body := w.Body.String()
	if !strings.Contains(body, "package: httpx") {
		t.Errorf("body does not contain 'package: httpx'")
	}
	if !strings.Contains(body, "vuln_count:") {
		t.Errorf("body does not contain 'vuln_count:'")
	}
}
