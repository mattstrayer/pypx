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
	// Pre-seed docs cache so api-changes section is populated without hitting goopy.
	docsFrom := `{"package":"httpx","version":"0.26.0","available":true,"modules":[{"name":"httpx","functions":[{"name":"send_singlerequest","kind":"function","signature":"def send_singlerequest(req)"}],"classes":[{"name":"Client","kind":"class","signature":"class Client","methods":[{"name":"send","kind":"method","signature":"def send(request)"}]}],"exceptions":[]}]}`
	docsTo := `{"package":"httpx","version":"0.28.1","available":true,"modules":[{"name":"httpx","functions":[],"classes":[{"name":"Client","kind":"class","signature":"class Client","methods":[{"name":"send","kind":"method","signature":"def send(request, *, stream=False)"}]},{"name":"AsyncClient","kind":"class","signature":"class AsyncClient","methods":[{"name":"aclose","kind":"method","signature":"def aclose()"}]}],"exceptions":[]}]}`
	return setupDiffHandlerWithDocs(t, pypiURL, docsFrom, docsTo)
}

// setupDiffHandlerWithDocs seeds the docs of httpx 0.26.0 and 0.28.1.
func setupDiffHandlerWithDocs(t *testing.T, pypiURL, docsFrom, docsTo string) (*chi.Mux, *handler.DiffHandler, func()) {
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

	if err := memCache.Set(handler.DocsCacheKey("httpx", "0.26.0"), []byte(docsFrom), 0); err != nil {
		t.Fatal(err)
	}
	if err := memCache.Set(handler.DocsCacheKey("httpx", "0.28.1"), []byte(docsTo), 0); err != nil {
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

// apiDiffBody returns the api changes section of diff.txt for the two seeded docs.
func apiDiffBody(t *testing.T, docsFrom, docsTo string) string {
	t.Helper()
	pypiURL, pypiSrv := newDiffTestServer(t)
	t.Cleanup(pypiSrv.Close)
	r, _, cleanup := setupDiffHandlerWithDocs(t, pypiURL, docsFrom, docsTo)
	t.Cleanup(cleanup)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/packages/httpx/diff.txt?from=0.26.0&to=0.28.1", nil))
	if w.Code != 200 {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	_, section, _ := strings.Cut(w.Body.String(), "## api changes")
	return section
}

func docsJSON(version, modules string) string {
	return `{"package":"httpx","version":"` + version + `","available":true,"modules":[` + modules + `]}`
}

func TestDiffHandler_FollowsAliases(t *testing.T) {
	foo := func(sig string) string { return `{"name":"foo","kind":"function","signature":"` + sig + `"}` }
	cli := `{"name":"Cli","kind":"class","signature":"class Cli","methods":[{"name":"get","kind":"method","signature":"def get()"}]}`

	// Version one defines the API in a public module, and the root re-exports it.
	public := docsJSON("0.26.0", `{"name":"pkg","functions":[],"classes":[],"exceptions":[],"aliases":[{"name":"foo","target":"pkg.core.foo"},{"name":"Cli","target":"pkg.core.Cli"}]},
		{"name":"pkg.core","functions":[`+foo("def foo()")+`],"classes":[`+cli+`],"exceptions":[]}`)
	// Version two moves the code to a private module, so the root holds the copies.
	private := func(fooSig string) string {
		return docsJSON("0.28.1", `{"name":"pkg","functions":[`+strings.Replace(foo(fooSig), `{"name"`, `{"origin":"pkg._core","name"`, 1)+`],"classes":[`+strings.Replace(cli, `{"name":"Cli"`, `{"origin":"pkg._core","name":"Cli"`, 1)+`],"exceptions":[]}`)
	}

	t.Run("moved to a private module", func(t *testing.T) {
		body := apiDiffBody(t, public, private("def foo()"))
		for _, reject := range []string{"added: pkg.foo", "added: pkg.Cli", "changed: pkg.foo", "changed: pkg.Cli"} {
			if strings.Contains(body, reject) {
				t.Errorf("unexpected %q:\n%s", reject, body)
			}
		}
		// The old public path can no longer be imported, so that part is a removal.
		for _, want := range []string{"removed: pkg.core.foo", "removed: pkg.core.Cli"} {
			if !strings.Contains(body, want) {
				t.Errorf("missing %q:\n%s", want, body)
			}
		}
	})

	t.Run("signature change through an alias", func(t *testing.T) {
		body := apiDiffBody(t, public, private("def foo(a)"))
		if !strings.Contains(body, "changed: pkg.foo") {
			t.Errorf("missing change:\n%s", body)
		}
	})

	t.Run("alias removed", func(t *testing.T) {
		kept := docsJSON("0.28.1", `{"name":"pkg.core","functions":[`+foo("def foo()")+`],"classes":[`+cli+`],"exceptions":[]}`)
		body := apiDiffBody(t, public, kept)
		for _, want := range []string{"removed: pkg.foo", "removed: pkg.Cli", "removed: pkg.Cli.get"} {
			if !strings.Contains(body, want) {
				t.Errorf("missing %q:\n%s", want, body)
			}
		}
		if strings.Contains(body, "removed: pkg.core") {
			t.Errorf("target stayed, but shows as removed:\n%s", body)
		}
	})

	t.Run("moved between private modules", func(t *testing.T) {
		from := private("def foo()")
		to := strings.ReplaceAll(from, "pkg._core", "pkg._impl")
		body := apiDiffBody(t, from, to)
		for _, reject := range []string{"pkg.foo", "pkg.Cli"} {
			if strings.Contains(body, reject) {
				t.Errorf("unexpected %q:\n%s", reject, body)
			}
		}
	})
}

func TestDiffHandler_AliasRows(t *testing.T) {
	fn := func(name string) string {
		return `{"name":"` + name + `","kind":"function","signature":"def ` + name + `()"}`
	}
	root := func(aliases string) string {
		return `{"name":"pkg","functions":[],"classes":[],"exceptions":[],"aliases":[` + aliases + `]}`
	}
	alias := func(name, target string) string {
		return `{"name":"` + name + `","target":"` + target + `"}`
	}
	// Both versions define foo in two modules with one signature, and neither has
	// a symbol for the attribute targets.
	mods := `{"name":"pkg.a","functions":[` + fn("foo") + `],"classes":[],"exceptions":[]},
		{"name":"pkg.b","functions":[` + fn("foo") + `],"classes":[],"exceptions":[]}`
	from := docsJSON("0.26.0", root(alias("foo", "pkg.a.foo")+","+alias("add", "pkg.attrs.add")+","+alias("sub", "pkg.attrs.sub"))+","+mods)

	tests := []struct {
		name   string
		to     string
		want   []string
		reject []string
	}{
		{
			name: "alias of an attribute removed",
			to:   docsJSON("0.28.1", root(alias("foo", "pkg.a.foo")+","+alias("add", "pkg.attrs.add"))+","+mods),
			want: []string{"- removed: pkg.sub"},
		},
		{
			name: "alias of an attribute added",
			to:   docsJSON("0.28.1", root(alias("foo", "pkg.a.foo")+","+alias("add", "pkg.attrs.add")+","+alias("sub", "pkg.attrs.sub")+","+alias("mul", "pkg.attrs.mul"))+","+mods),
			want: []string{"+ added: pkg.mul"},
		},
		{
			name: "alias of an attribute retargeted",
			to:   docsJSON("0.28.1", root(alias("foo", "pkg.a.foo")+","+alias("add", "pkg.other.add")+","+alias("sub", "pkg.attrs.sub"))+","+mods),
			want: []string{"~ changed: pkg.add", "-> pkg.attrs.add", "-> pkg.other.add"},
		},
		{
			name: "alias retargeted to a definition with the same signature",
			to:   docsJSON("0.28.1", root(alias("foo", "pkg.b.foo")+","+alias("add", "pkg.attrs.add")+","+alias("sub", "pkg.attrs.sub"))+","+mods),
			want: []string{"~ changed: pkg.foo", "-> pkg.a.foo", "-> pkg.b.foo"},
		},
		{
			name:   "nothing changed",
			to:     docsJSON("0.28.1", root(alias("foo", "pkg.a.foo")+","+alias("add", "pkg.attrs.add")+","+alias("sub", "pkg.attrs.sub"))+","+mods),
			reject: []string{"pkg.foo", "pkg.add", "pkg.sub"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := apiDiffBody(t, from, tt.to)
			for _, want := range tt.want {
				if !strings.Contains(body, want) {
					t.Errorf("missing %q:\n%s", want, body)
				}
			}
			for _, reject := range tt.reject {
				if strings.Contains(body, reject) {
					t.Errorf("unexpected %q:\n%s", reject, body)
				}
			}
		})
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
		{"slash in from", "?from=a/b&to=0.28.1", 400},
		{"traversal in from", "?from=..%2Fx&to=0.28.1", 400},
		{"space in to", "?from=0.26.0&to=0.28%201", 400},
		{"hash in to", "?from=0.26.0&to=0.28%231", 400},
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
		{"slash in from", "?from=a/b&to=0.28.1", 400},
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
