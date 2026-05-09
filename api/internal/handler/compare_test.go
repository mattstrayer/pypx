package handler_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/pypx/api/internal/cache"
	"github.com/pypx/api/internal/handler"
	"github.com/pypx/api/internal/osv"
	"github.com/pypx/api/internal/pypi"
	"github.com/pypx/api/internal/search"
)

func TestCompareHandler_Get_TwoPackages(t *testing.T) {
	pypiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		path := r.URL.Path
		switch {
		case strings.Contains(path, "/httpx/"):
			fmt.Fprint(w, `{"info":{"name":"httpx","version":"0.28.1","summary":"Next gen HTTP","license_expression":"BSD-3-Clause","requires_python":">=3.8","requires_dist":["anyio","certifi"]},"urls":[],"releases":{}}`)
		case strings.Contains(path, "/requests/"):
			fmt.Fprint(w, `{"info":{"name":"requests","version":"2.32.3","summary":"HTTP for Humans","license_expression":"Apache-2.0","requires_python":">=3.8","requires_dist":["urllib3","charset-normalizer","idna","certifi"]},"urls":[],"releases":{}}`)
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
	defer pypiSrv.Close()

	osvSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"vulns":[]}`)
	}))
	defer osvSrv.Close()

	c, err := cache.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	memCache := cache.NewMemoryCache(c, 100)

	idx, err := search.NewIndex(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer idx.Close()
	idx.UpsertBatch([]search.PackageEntry{ //nolint:errcheck
		{Name: "httpx", Summary: "Next gen HTTP", Downloads: 50_000_000},
		{Name: "requests", Summary: "HTTP for Humans", Downloads: 70_000_000},
	})

	pypiClient := pypi.NewClient(pypi.WithBaseURL(pypiSrv.URL))
	osvClient := osv.NewClient(osv.WithBaseURL(osvSrv.URL))
	pkgHandler := handler.NewPackageHandler(pypiClient, memCache)

	h := handler.NewCompareHandler(pkgHandler, pypiClient, osvClient, idx)

	r := chi.NewRouter()
	r.Get("/api/compare.txt", h.Get)

	req := httptest.NewRequest("GET", "/api/compare.txt?pkgs=httpx,requests", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("Content-Type = %q", ct)
	}
	body := w.Body.String()
	for _, want := range []string{
		"| metric",
		"httpx",
		"requests",
		"| version",
		"0.28.1",
		"2.32.3",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q:\n%s", want, body)
		}
	}
}

func TestCompareHandler_Get_SkipsMissing(t *testing.T) {
	pypiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if strings.Contains(path, "/httpx/") {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"info":{"name":"httpx","version":"0.28.1","summary":"x"},"urls":[],"releases":{}}`)
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer pypiSrv.Close()

	osvSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"vulns":[]}`)
	}))
	defer osvSrv.Close()

	c, _ := cache.New(":memory:")
	defer c.Close()
	memCache := cache.NewMemoryCache(c, 100)
	idx, _ := search.NewIndex(":memory:")
	defer idx.Close()

	pypiClient := pypi.NewClient(pypi.WithBaseURL(pypiSrv.URL))
	osvClient := osv.NewClient(osv.WithBaseURL(osvSrv.URL))
	pkgHandler := handler.NewPackageHandler(pypiClient, memCache)
	h := handler.NewCompareHandler(pkgHandler, pypiClient, osvClient, idx)

	r := chi.NewRouter()
	r.Get("/api/compare.txt", h.Get)

	req := httptest.NewRequest("GET", "/api/compare.txt?pkgs=httpx,fakepkg", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	body := w.Body.String()
	if !strings.HasPrefix(body, "# skipped: fakepkg (not found)\n") {
		t.Errorf("expected leading skipped comment, got:\n%s", body)
	}
	if !strings.Contains(body, "httpx") {
		t.Errorf("expected httpx column, got:\n%s", body)
	}
}

func TestCompareHandler_Get_BadInputs(t *testing.T) {
	c, _ := cache.New(":memory:")
	defer c.Close()
	memCache := cache.NewMemoryCache(c, 100)
	idx, _ := search.NewIndex(":memory:")
	defer idx.Close()

	pypiClient := pypi.NewClient()
	osvClient := osv.NewClient()
	pkgHandler := handler.NewPackageHandler(pypiClient, memCache)
	h := handler.NewCompareHandler(pkgHandler, pypiClient, osvClient, idx)

	r := chi.NewRouter()
	r.Get("/api/compare.txt", h.Get)

	cases := []struct {
		name     string
		query    string
		wantCode int
	}{
		{"empty pkgs", "?pkgs=", 400},
		{"missing pkgs", "", 400},
		{"too many", "?pkgs=a,b,c,d,e,f", 400},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/compare.txt"+tc.query, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != tc.wantCode {
				t.Errorf("status = %d, want %d", w.Code, tc.wantCode)
			}
		})
	}
}

func TestCompareHandler_Get_DedupesAndPreservesOrder(t *testing.T) {
	pypiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		path := r.URL.Path
		if strings.Contains(path, "/httpx/") {
			fmt.Fprint(w, `{"info":{"name":"httpx","version":"0.28.1","summary":"x"},"urls":[],"releases":{}}`)
			return
		}
		if strings.Contains(path, "/requests/") {
			fmt.Fprint(w, `{"info":{"name":"requests","version":"2.32.3","summary":"y"},"urls":[],"releases":{}}`)
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer pypiSrv.Close()

	osvSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"vulns":[]}`)
	}))
	defer osvSrv.Close()

	c, _ := cache.New(":memory:")
	defer c.Close()
	memCache := cache.NewMemoryCache(c, 100)
	idx, _ := search.NewIndex(":memory:")
	defer idx.Close()

	pypiClient := pypi.NewClient(pypi.WithBaseURL(pypiSrv.URL))
	osvClient := osv.NewClient(osv.WithBaseURL(osvSrv.URL))
	pkgHandler := handler.NewPackageHandler(pypiClient, memCache)
	h := handler.NewCompareHandler(pkgHandler, pypiClient, osvClient, idx)

	r := chi.NewRouter()
	r.Get("/api/compare.txt", h.Get)

	req := httptest.NewRequest("GET", "/api/compare.txt?pkgs=httpx,httpx,requests", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	body := w.Body.String()
	// First line is the header. Confirm httpx appears once and before requests.
	headerLine := strings.SplitN(body, "\n", 2)[0]
	hi := strings.Index(headerLine, "httpx")
	ri := strings.Index(headerLine, "requests")
	if hi < 0 || ri < 0 || hi >= ri {
		t.Errorf("order wrong, header: %q", headerLine)
	}
	if strings.Count(headerLine, "httpx") != 1 {
		t.Errorf("expected 1 httpx in header, got: %q", headerLine)
	}
}
