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
	gh "github.com/pypx/api/internal/github"
	"github.com/pypx/api/internal/gitlab"
	"github.com/pypx/api/internal/handler"
	"github.com/pypx/api/internal/pypi"
)

func newDiffTestServer(t *testing.T) (string, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/pypi/httpx/json":
			fmt.Fprint(w, `{
				"info": {"name":"httpx","version":"0.28.1","project_urls":{"Source":"https://github.com/encode/httpx"}},
				"urls": [],
				"releases": {"0.26.0": [{}], "0.27.0": [{}], "0.28.0": [{}], "0.28.1": [{}]}
			}`)
		case "/pypi/httpx/0.26.0/json":
			fmt.Fprint(w, `{"info":{"name":"httpx","version":"0.26.0","requires_dist":["httpcore==0.17.*","sniffio"]},"urls":[],"releases":{}}`)
		case "/pypi/httpx/0.28.1/json":
			fmt.Fprint(w, `{"info":{"name":"httpx","version":"0.28.1","requires_dist":["httpcore==1.*","sniffio","anyio"]},"urls":[],"releases":{}}`)
		default:
			http.Error(w, "not found: "+r.URL.Path, http.StatusNotFound)
		}
	}))
	return srv.URL, srv
}

func setupDiffHandler(t *testing.T, pypiURL string) (*chi.Mux, *handler.DiffHandler, func()) {
	t.Helper()
	c, err := cache.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	memCache := cache.NewMemoryCache(c, 100)

	pypiClient := pypi.NewClient(pypi.WithBaseURL(pypiURL))
	pkgHandler := handler.NewPackageHandler(pypiClient, memCache)

	docsHandler := handler.NewDocsHandler(pypiClient, memCache)
	ghClient := gh.NewClient()
	glClient := gitlab.NewClient()
	clHandler := handler.NewChangelogHandler(ghClient, glClient, memCache, pkgHandler)

	// Pre-seed docs cache so api-changes section is populated without hitting goopy.
	docsFrom := []byte(`{"package":"httpx","version":"0.26.0","available":true,"modules":[{"name":"httpx","functions":[{"name":"send_singlerequest","kind":"function","signature":"def send_singlerequest(req)"}],"classes":[{"name":"Client","kind":"class","signature":"class Client","methods":[{"name":"send","kind":"method","signature":"def send(request)"}]}],"exceptions":[]}]}`)
	docsTo := []byte(`{"package":"httpx","version":"0.28.1","available":true,"modules":[{"name":"httpx","functions":[],"classes":[{"name":"Client","kind":"class","signature":"class Client","methods":[{"name":"send","kind":"method","signature":"def send(request, *, stream=False)"}]},{"name":"AsyncClient","kind":"class","signature":"class AsyncClient","methods":[{"name":"aclose","kind":"method","signature":"def aclose()"}]}],"exceptions":[]}]}`)
	if err := memCache.Set("docs:httpx:0.26.0", docsFrom, 0); err != nil {
		t.Fatal(err)
	}
	if err := memCache.Set("docs:httpx:0.28.1", docsTo, 0); err != nil {
		t.Fatal(err)
	}

	h := handler.NewDiffHandler(pypiClient, memCache, docsHandler, clHandler, pkgHandler)
	r := chi.NewRouter()
	r.Get("/api/packages/{name}/diff.txt", h.Get)
	r.Get("/api/packages/{name}/diff", h.GetJSON)
	return r, h, func() { c.Close() }
}

func TestDiffHandler_Get_HappyPath(t *testing.T) {
	pypiURL, pypiSrv := newDiffTestServer(t)
	defer pypiSrv.Close()
	r, _, cleanup := setupDiffHandler(t, pypiURL)
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/packages/httpx/diff.txt?from=0.26.0&to=0.28.1", nil)
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
		"package: httpx",
		"from: 0.26.0",
		"to: 0.28.1",
		"## changelog",
		"## dependency changes",
		"## api changes",
		"+ added: anyio",
		"~ bumped: httpcore",
		"+ added: httpx.AsyncClient.aclose",
		"- removed: httpx.send_singlerequest",
		"~ changed: httpx.Client.send",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q:\n%s", want, body)
		}
	}
}

func TestDiffHandler_Get_BadInputs(t *testing.T) {
	pypiURL, pypiSrv := newDiffTestServer(t)
	defer pypiSrv.Close()
	r, _, cleanup := setupDiffHandler(t, pypiURL)
	defer cleanup()

	cases := []struct {
		name     string
		query    string
		wantCode int
	}{
		{"missing from", "?to=0.28.1", 400},
		{"missing to", "?from=0.26.0", 400},
		{"both missing", "", 400},
		{"downgrade", "?from=0.28.1&to=0.26.0", 400},
		{"equal versions", "?from=0.28.1&to=0.28.1", 400},
		{"unknown from", "?from=99.0.0&to=0.28.1", 400},
		{"unknown to", "?from=0.26.0&to=99.0.0", 400},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/packages/httpx/diff.txt"+tc.query, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != tc.wantCode {
				t.Errorf("status = %d, want %d", w.Code, tc.wantCode)
			}
		})
	}
}

func TestDiffHandler_Get_PackageNotFound(t *testing.T) {
	pypiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer pypiSrv.Close()
	r, _, cleanup := setupDiffHandler(t, pypiSrv.URL)
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/packages/httpx/diff.txt?from=0.26.0&to=0.28.1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestDiffHandler_GetJSON_HappyPath(t *testing.T) {
	pypiURL, pypiSrv := newDiffTestServer(t)
	defer pypiSrv.Close()
	r, _, cleanup := setupDiffHandler(t, pypiURL)
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/packages/httpx/diff?from=0.26.0&to=0.28.1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var body struct {
		Package string `json:"Package"`
		From    string `json:"From"`
		To      string `json:"To"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if body.Package != "httpx" {
		t.Errorf("Package = %q, want httpx", body.Package)
	}
	if body.From != "0.26.0" {
		t.Errorf("From = %q, want 0.26.0", body.From)
	}
	if body.To != "0.28.1" {
		t.Errorf("To = %q, want 0.28.1", body.To)
	}
}

func TestDiffHandler_GetJSON_BadInputs(t *testing.T) {
	pypiURL, pypiSrv := newDiffTestServer(t)
	defer pypiSrv.Close()
	r, _, cleanup := setupDiffHandler(t, pypiURL)
	defer cleanup()

	cases := []struct {
		name     string
		query    string
		wantCode int
	}{
		{"missing from", "?to=0.28.1", 400},
		{"missing to", "?from=0.26.0", 400},
		{"downgrade", "?from=0.28.1&to=0.26.0", 400},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/packages/httpx/diff"+tc.query, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != tc.wantCode {
				t.Errorf("status = %d, want %d", w.Code, tc.wantCode)
			}
		})
	}
}
