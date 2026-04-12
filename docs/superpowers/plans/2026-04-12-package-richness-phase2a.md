# Package Richness Phase 2a Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add py.typed wheel detection, RST README rendering, and rendered API docs (griffe Python sidecar + new `/packages/{name}/docs` page) to pypx.

**Architecture:** Three independent features sharing the existing Go API + Nuxt frontend stack. py.typed extends the extras handler by performing a zip range request on the package wheel. RST rendering adds a new Go package mirroring the existing markdown package. Rendered docs use a FastAPI Python sidecar (4th docker-compose service) called lazily by a new Go handler, with results cached indefinitely in SQLite.

**Tech Stack:** Go 1.23 (chi router, archive/zip, net/http), Python 3.12 (griffe, FastAPI, httpx), Vue 3 / Nuxt 4, Tailwind CSS, SQLite cache

---

## File Map

| File | Action |
|------|--------|
| `api/internal/pypi/typed.go` | Create — py.typed wheel inspection |
| `api/internal/pypi/typed_test.go` | Create |
| `api/internal/handler/extras.go` | Modify — call CheckPyTyped after CheckTypeSupport |
| `api/internal/handler/extras_test.go` | Modify — add py.typed test case |
| `api/internal/rst/rst.go` | Create — lightweight RST → HTML renderer |
| `api/internal/rst/rst_test.go` | Create |
| `api/internal/handler/packages.go` | Modify — add RST rendering branch |
| `docs-worker/main.py` | Create — FastAPI sidecar |
| `docs-worker/requirements.txt` | Create |
| `docs-worker/Dockerfile` | Create |
| `api/internal/handler/docs.go` | Create — docs handler |
| `api/internal/handler/docs_test.go` | Create |
| `api/cmd/server/main.go` | Modify — wire docs handler + route |
| `docker-compose.yml` | Modify — add docs-worker service |
| `web/app/types/api.ts` | Modify — add DocsData types |
| `web/app/composables/useApi.ts` | Modify — add fetchDocs() |
| `web/app/pages/packages/[name].vue` | Modify — add conditional Docs tab |
| `web/app/pages/packages/[name]/docs.vue` | Create — docs page |

---

## Task 1: py.typed wheel inspection

**Files:**
- Create: `api/internal/pypi/typed.go`
- Create: `api/internal/pypi/typed_test.go`

### Background

A wheel file is a zip archive. Its central directory sits at the end of the file. We perform a single HTTP Range request for the last 64 KB, then use `archive/zip` with a custom `io.ReaderAt` that serves reads from our buffer. If any filename in the central directory matches `*.dist-info/py.typed` or `py.typed`, the package ships inline types.

The `CheckPyTyped(c *Client, wheelURL string) bool` function returns `false` on any error — callers must not cache a `false` result on error (only on definitive miss).

- [ ] **Step 1: Write the failing tests**

```go
// api/internal/pypi/typed_test.go
package pypi_test

import (
	"archive/zip"
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pypx/api/internal/pypi"
)

// buildWheel creates an in-memory zip with the given filenames (empty contents).
func buildWheel(filenames ...string) []byte {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, name := range filenames {
		fw, _ := w.Create(name)
		_ = fw
	}
	w.Close()
	return buf.Bytes()
}

func TestCheckPyTyped(t *testing.T) {
	withTyped := buildWheel(
		"requests-2.33.1.dist-info/METADATA",
		"requests-2.33.1.dist-info/py.typed",
		"requests/__init__.py",
	)
	withoutTyped := buildWheel(
		"numpy-2.0.0.dist-info/METADATA",
		"numpy/__init__.py",
	)

	tests := []struct {
		name     string
		wheel    []byte
		wantSize int64 // 0 means use actual size
		skipHead bool  // if true, HEAD returns no Content-Length
		want     bool
	}{
		{"py.typed present", withTyped, 0, false, true},
		{"py.typed absent", withoutTyped, 0, false, false},
		{"wheel over 50MB skipped", withTyped, 55 * 1024 * 1024, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				size := tt.wantSize
				if size == 0 {
					size = int64(len(tt.wheel))
				}
				if r.Method == http.MethodHead {
					w.Header().Set("Content-Length", fmt.Sprintf("%d", size))
					w.Header().Set("Accept-Ranges", "bytes")
					return
				}
				// Serve Range request.
				rangeHdr := r.Header.Get("Range")
				if rangeHdr != "" && size == int64(len(tt.wheel)) {
					http.ServeContent(w, r, "wheel.whl", time.Time{}, bytes.NewReader(tt.wheel))
					return
				}
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			c := pypi.NewClient(pypi.WithBaseURL(srv.URL))
			got := pypi.CheckPyTyped(c, srv.URL+"/wheel.whl")
			if got != tt.want {
				t.Errorf("CheckPyTyped() = %v, want %v", got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run the test to confirm it fails**

```bash
cd api && go test ./internal/pypi/... -run TestCheckPyTyped -v
```
Expected: compile error — `pypi.CheckPyTyped` undefined.

- [ ] **Step 3: Write the implementation**

```go
// api/internal/pypi/typed.go
package pypi

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

const (
	maxWheelBytes  = 50 * 1024 * 1024 // 50 MB — skip if larger
	tailWindowSize = 64 * 1024         // 64 KB range request
)

// partialReader implements io.ReaderAt for the tail of a remote file.
// It serves reads from a buffered tail; reads outside the buffer fail.
type partialReader struct {
	data     []byte
	fileSize int64
}

func (r *partialReader) ReadAt(p []byte, off int64) (int, error) {
	bufStart := r.fileSize - int64(len(r.data))
	if off < bufStart {
		return 0, fmt.Errorf("read at offset %d is before buffered region starting at %d", off, bufStart)
	}
	localOff := off - bufStart
	if localOff >= int64(len(r.data)) {
		return 0, io.EOF
	}
	n := copy(p, r.data[localOff:])
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}

// extractWheelURL returns the URL of the first bdist_wheel file in urls,
// preferring pure-python wheels (py3-none-any). Returns "" if none found.
func extractWheelURL(files []ReleaseFile) string {
	var first string
	for _, f := range files {
		if f.PackageType != "bdist_wheel" {
			continue
		}
		if first == "" {
			first = f.URL
		}
		if strings.Contains(f.Filename, "none-any") {
			return f.URL
		}
	}
	return first
}

// CheckPyTyped checks whether the given wheel URL contains a py.typed marker.
// It uses an HTTP Range request to fetch only the last 64 KB (zip central directory).
// Returns false on any error or if the wheel exceeds 50 MB.
func CheckPyTyped(c *Client, wheelURL string) bool {
	// HEAD request to get file size.
	head, err := c.httpClient.Head(wheelURL)
	if err != nil {
		return false
	}
	head.Body.Close()

	contentLength, err := strconv.ParseInt(head.Header.Get("Content-Length"), 10, 64)
	if err != nil || contentLength <= 0 {
		return false
	}
	if contentLength > maxWheelBytes {
		return false
	}

	// Range request for the last tailWindowSize bytes (or whole file if smaller).
	start := contentLength - tailWindowSize
	if start < 0 {
		start = 0
	}
	req, err := http.NewRequest(http.MethodGet, wheelURL, nil)
	if err != nil {
		return false
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-", start))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
		return false
	}

	tail, err := io.ReadAll(resp.Body)
	if err != nil {
		return false
	}

	// Use archive/zip with partialReader to parse the central directory.
	zr, err := zip.NewReader(&partialReader{data: tail, fileSize: contentLength}, contentLength)
	if err != nil {
		return false
	}

	for _, f := range zr.File {
		name := f.Name
		if name == "py.typed" ||
			strings.HasSuffix(name, ".dist-info/py.typed") ||
			strings.HasSuffix(name, ".dist-info\\py.typed") {
			return true
		}
	}
	return false
}
```

Note: the test file above uses `fmt` and `time` — add those imports.

- [ ] **Step 4: Add missing imports to the test file**

The test file needs these imports:
```go
import (
	"archive/zip"
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/pypx/api/internal/pypi"
)
```

- [ ] **Step 5: Run the tests to confirm they pass**

```bash
cd api && go test ./internal/pypi/... -run TestCheckPyTyped -v
```
Expected: all 3 subtests PASS.

- [ ] **Step 6: Run full pypi package tests**

```bash
cd api && go test ./internal/pypi/... -v
```
Expected: all tests PASS.

- [ ] **Step 7: Commit**

```bash
git add api/internal/pypi/typed.go api/internal/pypi/typed_test.go
git commit -m "feat(api): add py.typed wheel inspection via HTTP range request"
```

---

## Task 2: Extras handler — py.typed integration

**Files:**
- Modify: `api/internal/handler/extras.go`
- Modify: `api/internal/handler/extras_test.go`

### Background

After `CheckTypeSupport` runs, if the result is not `"typed"`, fetch the package metadata to get the wheel URL, check the py.typed cache, then call `CheckPyTyped` if needed. Cache the typed check result indefinitely (`ttl=0`) since wheel content is immutable per version.

The extras response itself is cached with a 24h TTL, so the wheel inspection only runs once per 24h window per package.

- [ ] **Step 1: Update the extras handler**

Replace `api/internal/handler/extras.go` with:

```go
package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pypx/api/internal/cache"
	"github.com/pypx/api/internal/conda"
	"github.com/pypx/api/internal/pypi"
)

const extrasTTL = 24 * time.Hour

// ExtrasResponse is the response for GET /api/packages/{name}/extras.
type ExtrasResponse struct {
	Package     string                `json:"package"`
	TypeSupport pypi.TypeSupport      `json:"type_support"`
	CondaForge  *conda.CondaForgeInfo `json:"conda_forge"`
}

// ExtrasHandler serves type support and conda-forge data.
type ExtrasHandler struct {
	pypi  *pypi.Client
	conda *conda.Client
	cache cache.Cacher
}

// NewExtrasHandler creates a new ExtrasHandler.
func NewExtrasHandler(pypiClient *pypi.Client, condaClient *conda.Client, c cache.Cacher) *ExtrasHandler {
	return &ExtrasHandler{pypi: pypiClient, conda: condaClient, cache: c}
}

// Get handles GET /api/packages/{name}/extras.
func (h *ExtrasHandler) Get(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if !validateName(w, name) {
		return
	}

	cacheKey := "extras:" + strings.ToLower(name)

	if data, _, err := h.cache.Get(cacheKey, extrasTTL); err == nil && data != nil {
		w.Header().Set("Cache-Control", "public, max-age=86400")
		w.Header().Set("Content-Type", "application/json")
		w.Write(data) //nolint:errcheck
		return
	}

	// Fetch type support and conda info in parallel.
	var (
		typeSupport pypi.TypeSupport
		condaInfo   conda.CondaForgeInfo
		condaErr    error
		wg          sync.WaitGroup
	)

	wg.Add(2)
	go func() {
		defer wg.Done()
		typeSupport = pypi.CheckTypeSupport(h.pypi, name)
	}()
	go func() {
		defer wg.Done()
		condaInfo, condaErr = h.conda.FetchCondaInfo(name)
	}()
	wg.Wait()

	// If not already typed via stubs, check for py.typed marker in the wheel.
	if typeSupport.Status != "typed" {
		if pkg, err := h.pypi.FetchPackage(name); err == nil {
			typedKey := "typed:" + strings.ToLower(name) + ":" + pkg.Info.Version
			if data, _, err := h.cache.Get(typedKey, 0); err == nil && data != nil {
				if string(data) == "1" {
					typeSupport.Status = "typed"
				}
			} else {
				wheelURL := pypi.ExtractWheelURL(pkg.URLs)
				if wheelURL != "" && pypi.CheckPyTyped(h.pypi, wheelURL) {
					typeSupport.Status = "typed"
					h.cache.Set(typedKey, []byte("1"), 0) //nolint:errcheck
				} else if wheelURL != "" {
					// Cache negative result too — wheel content is immutable.
					h.cache.Set(typedKey, []byte("0"), 0) //nolint:errcheck
				}
			}
		}
	}

	resp := ExtrasResponse{
		Package:     name,
		TypeSupport: typeSupport,
	}
	if condaErr == nil {
		resp.CondaForge = &condaInfo
	}

	encoded, err := json.Marshal(resp)
	if err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}

	h.cache.Set(cacheKey, encoded, extrasTTL) //nolint:errcheck

	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Header().Set("Content-Type", "application/json")
	w.Write(encoded) //nolint:errcheck
}
```

- [ ] **Step 2: Export `extractWheelURL` from typed.go**

In `api/internal/pypi/typed.go`, rename `extractWheelURL` → `ExtractWheelURL` (capital E):

```go
// ExtractWheelURL returns the URL of the first bdist_wheel file in urls,
// preferring pure-python wheels (py3-none-any). Returns "" if none found.
func ExtractWheelURL(files []ReleaseFile) string {
```

- [ ] **Step 3: Add a py.typed test case to extras_test.go**

Add a second test function in `api/internal/handler/extras_test.go` after `TestExtrasHandlerGet`:

```go
func TestExtrasHandlerGetPyTyped(t *testing.T) {
	// Build a minimal wheel zip with py.typed.
	var wheelBuf bytes.Buffer
	zw := zip.NewWriter(&wheelBuf)
	zw.Create("typed_pkg-1.0.0.dist-info/py.typed")
	zw.Create("typed_pkg/__init__.py")
	zw.Close()
	wheelBytes := wheelBuf.Bytes()

	pypiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/pypi/typed-pkg/json" || r.URL.Path == "/pypi/typed_pkg/json":
			// No stubs packages.
			w.WriteHeader(http.StatusNotFound)
		case r.URL.Path == "/pypi/types-typed-pkg/json":
			w.WriteHeader(http.StatusNotFound)
		case r.URL.Path == "/pypi/typed-pkg-stubs/json":
			w.WriteHeader(http.StatusNotFound)
		// Package metadata endpoint.
		case r.URL.Path == "/pypi/typed-pkg/json" || r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/json"):
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{
				"info": {"name":"typed-pkg","version":"1.0.0"},
				"urls": [{"packagetype":"bdist_wheel","url":"%s/wheel.whl","filename":"typed_pkg-1.0.0-py3-none-any.whl","size":%d}],
				"releases": {}
			}`, r.Host, len(wheelBytes))
		case r.URL.Path == "/wheel.whl":
			if r.Method == http.MethodHead {
				w.Header().Set("Content-Length", fmt.Sprintf("%d", len(wheelBytes)))
				w.Header().Set("Accept-Ranges", "bytes")
				return
			}
			http.ServeContent(w, r, "wheel.whl", time.Time{}, bytes.NewReader(wheelBytes))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer pypiSrv.Close()

	condaSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer condaSrv.Close()

	sqliteCache, err := cache.New(":memory:")
	if err != nil {
		t.Fatalf("cache.New: %v", err)
	}
	defer sqliteCache.Close()
	memCache := cache.NewMemoryCache(sqliteCache, 100)

	pypiClient := pypi.NewClient(pypi.WithBaseURL(pypiSrv.URL))
	condaClient := conda.NewClient(conda.WithBaseURL(condaSrv.URL))
	h := handler.NewExtrasHandler(pypiClient, condaClient, memCache)

	router := chi.NewRouter()
	router.Get("/api/packages/{name}/extras", h.Get)

	req := httptest.NewRequest(http.MethodGet, "/api/packages/typed-pkg/extras", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	var resp handler.ExtrasResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.TypeSupport.Status != "typed" {
		t.Errorf("TypeSupport.Status = %q, want typed", resp.TypeSupport.Status)
	}
}
```

Add the required imports to `extras_test.go`:
```go
import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pypx/api/internal/cache"
	"github.com/pypx/api/internal/conda"
	"github.com/pypx/api/internal/handler"
	"github.com/pypx/api/internal/pypi"
)
```

- [ ] **Step 4: Run the handler tests**

```bash
cd api && go test ./internal/handler/... -run TestExtras -v
```
Expected: both `TestExtrasHandlerGet` and `TestExtrasHandlerGetPyTyped` PASS.

Note: the py.typed test may require adjusting the mock server URL format — if you see URL issues in the wheel metadata JSON, update the `r.Host` reference to use `pypiSrv.URL` directly in the JSON template.

- [ ] **Step 5: Run all tests**

```bash
cd api && go test ./...
```
Expected: all PASS, no compile errors.

- [ ] **Step 6: Commit**

```bash
git add api/internal/pypi/typed.go api/internal/handler/extras.go api/internal/handler/extras_test.go
git commit -m "feat(api): integrate py.typed detection into extras endpoint"
```

---

## Task 3: RST renderer

**Files:**
- Create: `api/internal/rst/rst.go`
- Create: `api/internal/rst/rst_test.go`

### Background

Lightweight RST → HTML converter that handles the most common directives found in PyPI READMEs. Not full Sphinx compatibility — "renders well for 90% of packages." Same `Render(src string) (string, error)` interface as the markdown package.

Key constructs handled:
- Headings (underline style: `=` h1, `-` h2, `~` h3, `^` h4)
- Paragraphs
- Bullet lists (`-` or `*`)
- Literal blocks (paragraph ending `::`)
- Directives: `code-block`, `note`, `warning`, `image`, `toctree` (omitted)
- Inline: `` `code` ``, `**bold**`, `*italic*`, `` :role:`text` ``

- [ ] **Step 1: Write the failing tests**

```go
// api/internal/rst/rst_test.go
package rst_test

import (
	"strings"
	"testing"

	"github.com/pypx/api/internal/rst"
)

func TestRender(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantContain string // empty means output should be empty
		wantEmpty   bool
	}{
		{
			name:      "empty input",
			input:     "",
			wantEmpty: true,
		},
		{
			name:        "plain paragraph",
			input:       "Hello world.\n",
			wantContain: "<p>Hello world.</p>",
		},
		{
			name:        "bold inline",
			input:       "This is **bold** text.",
			wantContain: "<strong>bold</strong>",
		},
		{
			name:        "italic inline",
			input:       "This is *italic* text.",
			wantContain: "<em>italic</em>",
		},
		{
			name:        "code inline",
			input:       "Use `my_func()` here.",
			wantContain: "<code>my_func()</code>",
		},
		{
			name:        "role inline",
			input:       "See :func:`requests.get` for details.",
			wantContain: "<code>requests.get</code>",
		},
		{
			name:        "h1 heading",
			input:       "My Title\n========\n",
			wantContain: "<h1>My Title</h1>",
		},
		{
			name:        "h2 heading",
			input:       "Section\n-------\n",
			wantContain: "<h2>Section</h2>",
		},
		{
			name:        "code-block directive",
			input:       ".. code-block:: python\n\n   x = 1\n   print(x)\n",
			wantContain: "<pre><code",
		},
		{
			name:        "code-block content",
			input:       ".. code-block:: python\n\n   x = 1\n",
			wantContain: "x = 1",
		},
		{
			name:        "note directive",
			input:       ".. note::\n\n   This is a note.\n",
			wantContain: "rst-note",
		},
		{
			name:        "warning directive",
			input:       ".. warning::\n\n   Be careful.\n",
			wantContain: "rst-warning",
		},
		{
			name:        "image directive",
			input:       ".. image:: /path/to/img.png\n",
			wantContain: "<img",
		},
		{
			name:      "toctree omitted",
			input:     ".. toctree::\n\n   page1\n   page2\n",
			wantEmpty: true,
		},
		{
			name:        "bullet list",
			input:       "- item one\n- item two\n",
			wantContain: "<li>",
		},
		{
			name:        "html escaping",
			input:       "Use <tag> & 'quotes'.",
			wantContain: "&lt;tag&gt;",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := rst.Render(tt.input)
			if err != nil {
				t.Fatalf("Render() error: %v", err)
			}
			if tt.wantEmpty {
				if strings.TrimSpace(got) != "" {
					t.Errorf("Render() = %q, want empty", got)
				}
				return
			}
			if !strings.Contains(got, tt.wantContain) {
				t.Errorf("Render() = %q\nwant substring: %q", got, tt.wantContain)
			}
		})
	}
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
cd api && go test ./internal/rst/... -v
```
Expected: compile error — package `rst` does not exist.

- [ ] **Step 3: Create the renderer**

```go
// api/internal/rst/rst.go
package rst

import (
	"fmt"
	"html"
	"strings"
)

// Render converts RST source to HTML.
// Returns an error only for nil-safe guard; callers may ignore the error and
// fall back to raw text if the output looks wrong.
func Render(src string) (string, error) {
	if src == "" {
		return "", nil
	}
	return renderDoc(src), nil
}

// headingLevel maps underline characters to HTML heading levels.
func headingLevel(ch byte) int {
	switch ch {
	case '=':
		return 1
	case '-':
		return 2
	case '~':
		return 3
	case '^':
		return 4
	default:
		return 5
	}
}

// isUnderline returns true when underline is all the same non-space character
// and at least as long as the text line above it.
func isUnderline(underline, above string) bool {
	u := strings.TrimRight(underline, "\n")
	if len(u) < len(above) || len(u) == 0 {
		return false
	}
	ch := u[0]
	if ch == ' ' || ch == '\t' {
		return false
	}
	for _, c := range []byte(u) {
		if c != ch {
			return false
		}
	}
	return true
}

// applyInline processes inline RST markup and returns escaped HTML.
func applyInline(s string) string {
	var b strings.Builder
	i := 0
	for i < len(s) {
		// Role: :role:`text`
		if s[i] == ':' {
			end := strings.IndexByte(s[i+1:], ':')
			if end >= 0 {
				afterColon := i + 1 + end + 1
				if afterColon < len(s) && s[afterColon] == '`' {
					tickEnd := strings.IndexByte(s[afterColon+1:], '`')
					if tickEnd >= 0 {
						text := s[afterColon+1 : afterColon+1+tickEnd]
						b.WriteString("<code>")
						b.WriteString(html.EscapeString(text))
						b.WriteString("</code>")
						i = afterColon + 1 + tickEnd + 1
						continue
					}
				}
			}
		}
		// Bold: **text**
		if i+1 < len(s) && s[i] == '*' && s[i+1] == '*' {
			end := strings.Index(s[i+2:], "**")
			if end >= 0 {
				b.WriteString("<strong>")
				b.WriteString(html.EscapeString(s[i+2 : i+2+end]))
				b.WriteString("</strong>")
				i = i + 4 + end
				continue
			}
		}
		// Italic: *text* (not **)
		if s[i] == '*' && (i+1 >= len(s) || s[i+1] != '*') {
			end := strings.IndexByte(s[i+1:], '*')
			if end >= 0 && end > 0 {
				b.WriteString("<em>")
				b.WriteString(html.EscapeString(s[i+1 : i+1+end]))
				b.WriteString("</em>")
				i = i + 2 + end
				continue
			}
		}
		// Inline code: `text`
		if s[i] == '`' {
			end := strings.IndexByte(s[i+1:], '`')
			if end >= 0 {
				b.WriteString("<code>")
				b.WriteString(html.EscapeString(s[i+1 : i+1+end]))
				b.WriteString("</code>")
				i = i + 2 + end
				continue
			}
		}
		// HTML-escape raw characters.
		switch s[i] {
		case '&':
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		case '"':
			b.WriteString("&#34;")
		default:
			b.WriteByte(s[i])
		}
		i++
	}
	return b.String()
}

// collectIndentedBody reads lines that are indented (or blank within the block)
// starting at lines[start]. Returns the dedented lines and the next line index.
func collectIndentedBody(lines []string, start int) ([]string, int) {
	var body []string
	i := start
	// Skip leading blank lines.
	for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}
	if i >= len(lines) {
		return nil, i
	}
	// Determine indentation from first non-blank line.
	indent := len(lines[i]) - len(strings.TrimLeft(lines[i], " \t"))
	if indent == 0 {
		return nil, i
	}
	for i < len(lines) {
		line := lines[i]
		if strings.TrimSpace(line) == "" {
			// Allow blank lines within body only if followed by indented line.
			if i+1 < len(lines) {
				nextIndent := len(lines[i+1]) - len(strings.TrimLeft(lines[i+1], " \t"))
				if nextIndent >= indent {
					body = append(body, "")
					i++
					continue
				}
			}
			break
		}
		lineIndent := len(line) - len(strings.TrimLeft(line, " \t"))
		if lineIndent < indent {
			break
		}
		dedented := line
		if len(line) >= indent {
			dedented = line[indent:]
		}
		body = append(body, dedented)
		i++
	}
	return body, i
}

func renderDoc(src string) string {
	src = strings.ReplaceAll(src, "\r\n", "\n")
	lines := strings.Split(src, "\n")
	var out strings.Builder
	i := 0

	for i < len(lines) {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		// Blank line.
		if trimmed == "" {
			i++
			continue
		}

		// Directive: .. directive:: args
		if strings.HasPrefix(trimmed, ".. ") {
			rest := trimmed[3:] // after ".. "
			// Extract directive name and args.
			colonIdx := strings.Index(rest, "::")
			if colonIdx >= 0 {
				directive := strings.TrimSpace(rest[:colonIdx])
				args := strings.TrimSpace(rest[colonIdx+2:])
				body, next := collectIndentedBody(lines, i+1)
				i = next

				switch directive {
				case "code-block", "code", "sourcecode":
					lang := args
					code := strings.Join(body, "\n")
					if lang != "" {
						fmt.Fprintf(&out, "<pre><code class=\"language-%s\">%s</code></pre>\n",
							html.EscapeString(lang), html.EscapeString(code))
					} else {
						fmt.Fprintf(&out, "<pre><code>%s</code></pre>\n", html.EscapeString(code))
					}

				case "note":
					fmt.Fprintf(&out, "<div class=\"rst-note\">%s</div>\n",
						applyInline(strings.Join(body, " ")))

				case "warning", "caution", "danger", "attention":
					fmt.Fprintf(&out, "<div class=\"rst-warning\">%s</div>\n",
						applyInline(strings.Join(body, " ")))

				case "image":
					src := args
					fmt.Fprintf(&out, "<img src=\"%s\" alt=\"\">\n", html.EscapeString(src))

				case "toctree", "contents", "include", "literalinclude":
					// Omit — internal Sphinx directives not meaningful in this context.

				default:
					// Unknown directive — render body as paragraph if non-empty.
					if len(body) > 0 {
						fmt.Fprintf(&out, "<p>%s</p>\n", applyInline(strings.Join(body, " ")))
					}
				}
				continue
			}
		}

		// Heading: current line followed by underline.
		if i+1 < len(lines) && isUnderline(lines[i+1], trimmed) {
			level := headingLevel(strings.TrimSpace(lines[i+1])[0])
			fmt.Fprintf(&out, "<h%d>%s</h%d>\n", level, applyInline(trimmed), level)
			i += 2
			continue
		}

		// Bullet list.
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
			out.WriteString("<ul>\n")
			for i < len(lines) {
				t := strings.TrimSpace(lines[i])
				if t == "" {
					i++
					break
				}
				if strings.HasPrefix(t, "- ") || strings.HasPrefix(t, "* ") {
					fmt.Fprintf(&out, "<li>%s</li>\n", applyInline(t[2:]))
					i++
				} else {
					break
				}
			}
			out.WriteString("</ul>\n")
			continue
		}

		// Literal block: paragraph ending with ::.
		if strings.HasSuffix(trimmed, "::") {
			text := strings.TrimSuffix(trimmed, "::")
			text = strings.TrimRight(text, " ")
			if text != "" {
				fmt.Fprintf(&out, "<p>%s:</p>\n", applyInline(text))
			}
			body, next := collectIndentedBody(lines, i+1)
			i = next
			if len(body) > 0 {
				fmt.Fprintf(&out, "<pre><code>%s</code></pre>\n",
					html.EscapeString(strings.Join(body, "\n")))
			}
			continue
		}

		// Regular paragraph: collect until blank line or heading underline.
		var paraLines []string
		for i < len(lines) {
			t := strings.TrimSpace(lines[i])
			if t == "" {
				i++
				break
			}
			// Stop before a directive.
			if strings.HasPrefix(t, ".. ") {
				break
			}
			// Stop before a heading (next line is underline).
			if i+1 < len(lines) && isUnderline(lines[i+1], t) {
				break
			}
			// Stop before a bullet.
			if strings.HasPrefix(t, "- ") || strings.HasPrefix(t, "* ") {
				break
			}
			paraLines = append(paraLines, t)
			i++
		}
		if len(paraLines) > 0 {
			fmt.Fprintf(&out, "<p>%s</p>\n", applyInline(strings.Join(paraLines, " ")))
		}
	}

	return out.String()
}
```

- [ ] **Step 4: Run the tests**

```bash
cd api && go test ./internal/rst/... -v
```
Expected: all tests PASS. If the `toctree omitted` test fails (output not empty), verify the `collectIndentedBody` loop doesn't emit the toctree body.

- [ ] **Step 5: Commit**

```bash
git add api/internal/rst/
git commit -m "feat(api): add lightweight RST to HTML renderer"
```

---

## Task 4: RST rendering in packages handler

**Files:**
- Modify: `api/internal/handler/packages.go`

The `buildPackageResponse` function already has the markdown branch. Extend it to also handle RST.

- [ ] **Step 1: Add the RST import and extend the branch**

In `api/internal/handler/packages.go`, find the import block and add:
```go
"github.com/pypx/api/internal/rst"
```

Find this block (around line 346):
```go
var descHTML string
if strings.Contains(info.DescriptionType, "text/markdown") {
    descHTML, _ = markdown.Render(info.Description)
}
```

Replace with:
```go
var descHTML string
switch {
case strings.Contains(info.DescriptionType, "text/markdown"):
    descHTML, _ = markdown.Render(info.Description)
case strings.Contains(info.DescriptionType, "text/x-rst"),
    strings.Contains(info.DescriptionType, "text/x-restructuredtext"):
    descHTML, _ = rst.Render(info.Description)
}
```

- [ ] **Step 2: Build to verify no compile errors**

```bash
cd api && go build ./...
```
Expected: no errors.

- [ ] **Step 3: Run all handler tests**

```bash
cd api && go test ./internal/handler/... -v
```
Expected: all PASS.

- [ ] **Step 4: Commit**

```bash
git add api/internal/handler/packages.go
git commit -m "feat(api): render RST README descriptions as HTML"
```

---

## Task 5: docs-worker Python sidecar

**Files:**
- Create: `docs-worker/main.py`
- Create: `docs-worker/requirements.txt`
- Create: `docs-worker/Dockerfile`

### Background

The sidecar receives `{"name": "requests", "version": "2.33.1"}`, downloads the wheel from PyPI, extracts .py files to a temp directory, runs griffe to parse the public API, transforms the result to our JSON format, and returns it. For binary-only packages (no .py files), it returns `{"empty": true, "reason": "no_python_source"}`.

- [ ] **Step 1: Create requirements.txt**

```
# docs-worker/requirements.txt
griffe==1.7.3
fastapi==0.115.12
uvicorn[standard]==0.34.2
httpx==0.28.1
```

- [ ] **Step 2: Create the FastAPI sidecar**

```python
# docs-worker/main.py
import io
import os
import re
import tempfile
import zipfile
from typing import Any

import griffe
import httpx
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel

app = FastAPI()


class GenerateRequest(BaseModel):
    name: str
    version: str


def normalize_name(name: str) -> str:
    return re.sub(r"[-_.]+", "_", name).lower()


def get_top_level_packages(zf: zipfile.ZipFile, pkg_name: str) -> list[str]:
    """Return importable top-level package names from the wheel."""
    for entry in zf.namelist():
        if entry.endswith(".dist-info/top_level.txt"):
            content = zf.read(entry).decode("utf-8", errors="replace").strip()
            pkgs = [p.strip() for p in content.splitlines() if p.strip()]
            if pkgs:
                return pkgs
    # Infer from wheel structure.
    tops: set[str] = set()
    for entry in zf.namelist():
        parts = entry.split("/")
        if (
            len(parts) >= 2
            and not parts[0].endswith(".dist-info")
            and not parts[0].endswith(".data")
            and parts[0]
        ):
            tops.add(parts[0])
    return list(tops) if tops else [normalize_name(pkg_name)]


def get_signature(member: Any) -> str:
    try:
        params = []
        if hasattr(member, "parameters"):
            for p in member.parameters:
                s = p.name
                if p.annotation is not None:
                    s += f": {p.annotation}"
                if p.default is not None and str(p.default) not in ("", "PosOnlyParam", "KwOnlyParam"):
                    s += f" = {p.default}"
                params.append(s)
        ret = ""
        if hasattr(member, "returns") and member.returns is not None:
            ret = f" -> {member.returns}"
        prefix = "class " if member.kind.value == "class" else "def "
        return f"{prefix}{member.name}({', '.join(params)}){ret}"
    except Exception:
        return member.name


def get_docstring(member: Any) -> str:
    if not member.docstring:
        return ""
    return member.docstring.value or ""


def is_exception_class(member: Any) -> bool:
    if member.kind.value != "class":
        return False
    for base in getattr(member, "bases", []):
        base_str = str(base)
        if any(word in base_str for word in ("Exception", "Error", "Warning", "BaseException")):
            return True
    return False


def transform_parameters(member: Any) -> list[dict]:
    if not hasattr(member, "parameters"):
        return []
    result = []
    for p in member.parameters:
        annotation = None
        if p.annotation is not None:
            try:
                annotation = str(p.annotation)
            except Exception:
                pass
        result.append({"name": p.name, "type": annotation, "description": ""})
    return result


def transform_returns(member: Any) -> dict | None:
    if not hasattr(member, "returns") or member.returns is None:
        return None
    try:
        return {"type": str(member.returns), "description": ""}
    except Exception:
        return None


def transform_module(module: Any) -> dict:
    functions = []
    classes = []
    exceptions = []

    for name, member in module.members.items():
        if name.startswith("_"):
            continue
        kind = member.kind.value
        sym: dict[str, Any] = {
            "name": member.name,
            "kind": kind,
            "signature": get_signature(member),
            "docstring": get_docstring(member),
            "parameters": transform_parameters(member),
            "returns": transform_returns(member),
        }
        if kind == "function":
            functions.append(sym)
        elif kind == "class":
            if is_exception_class(member):
                exceptions.append(sym)
            else:
                classes.append(sym)

    return {
        "name": module.name,
        "functions": functions,
        "classes": classes,
        "exceptions": exceptions,
    }


@app.post("/generate")
async def generate(req: GenerateRequest) -> dict:
    # Fetch PyPI metadata to get wheel URL.
    pypi_url = f"https://pypi.org/pypi/{req.name}/{req.version}/json"
    try:
        resp = httpx.get(pypi_url, timeout=15)
        resp.raise_for_status()
        data = resp.json()
    except Exception as e:
        raise HTTPException(status_code=502, detail=f"PyPI fetch failed: {e}")

    # Find best wheel URL: prefer pure-python, fall back to first wheel.
    wheel_url: str | None = None
    for f in data.get("urls", []):
        if f["packagetype"] != "bdist_wheel":
            continue
        if wheel_url is None:
            wheel_url = f["url"]
        if "none-any" in f["filename"]:
            wheel_url = f["url"]
            break

    if not wheel_url:
        return {"empty": True, "reason": "no_wheel", "modules": []}

    # Download wheel.
    try:
        wheel_resp = httpx.get(wheel_url, timeout=60, follow_redirects=True)
        wheel_resp.raise_for_status()
        wheel_bytes = wheel_resp.content
    except Exception as e:
        raise HTTPException(status_code=502, detail=f"Wheel download failed: {e}")

    # Open as zip.
    try:
        zf = zipfile.ZipFile(io.BytesIO(wheel_bytes))
    except Exception as e:
        raise HTTPException(status_code=502, detail=f"Wheel open failed: {e}")

    py_files = {
        name: zf.read(name)
        for name in zf.namelist()
        if name.endswith(".py") and "__pycache__" not in name
    }

    if not py_files:
        return {"empty": True, "reason": "no_python_source", "modules": []}

    top_pkgs = get_top_level_packages(zf, req.name)

    modules = []
    with tempfile.TemporaryDirectory() as tmpdir:
        # Write .py files to temp dir.
        for filename, content in py_files.items():
            filepath = os.path.join(tmpdir, filename)
            os.makedirs(os.path.dirname(filepath), exist_ok=True)
            try:
                with open(filepath, "wb") as fh:
                    fh.write(content)
            except Exception:
                continue

        for pkg_name in top_pkgs:
            try:
                module = griffe.load(pkg_name, search_paths=[tmpdir])
                transformed = transform_module(module)
                if any(transformed[k] for k in ("functions", "classes", "exceptions")):
                    modules.append(transformed)
            except Exception:
                continue

    if not modules:
        return {"empty": True, "reason": "no_python_source", "modules": []}

    return {"empty": False, "reason": "", "modules": modules}


if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8000)
```

- [ ] **Step 3: Create the Dockerfile**

```dockerfile
# docs-worker/Dockerfile
FROM python:3.12-slim

WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY main.py .

CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8000"]
```

- [ ] **Step 4: Build the sidecar locally to confirm it works**

```bash
cd docs-worker && docker build -t pypx-docs-worker .
```
Expected: build succeeds, no errors.

- [ ] **Step 5: Smoke test the sidecar**

```bash
docker run --rm -p 8000:8000 pypx-docs-worker &
sleep 3
curl -s -X POST http://localhost:8000/generate \
  -H "Content-Type: application/json" \
  -d '{"name":"requests","version":"2.33.1"}' | python3 -m json.tool | head -40
```
Expected: JSON response with `"empty": false` and `"modules"` array containing functions like `get`, `post`, etc.

Kill the test container when done:
```bash
docker stop $(docker ps -q --filter ancestor=pypx-docs-worker)
```

- [ ] **Step 6: Commit**

```bash
git add docs-worker/
git commit -m "feat(docs-worker): add Python griffe sidecar for API doc generation"
```

---

## Task 6: Docs API handler

**Files:**
- Create: `api/internal/handler/docs.go`
- Create: `api/internal/handler/docs_test.go`

### Background

The handler resolves the latest version via `FetchPackage`, checks the `docs:{name}:{version}` cache (indefinite TTL = 0), calls the sidecar on miss, stores the result, and returns it. On sidecar unavailability, return 502 without caching.

- [ ] **Step 1: Write the failing test**

```go
// api/internal/handler/docs_test.go
package handler_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/pypx/api/internal/cache"
	"github.com/pypx/api/internal/handler"
	"github.com/pypx/api/internal/pypi"
)

func TestDocsHandlerGet(t *testing.T) {
	// Mock PyPI server returning package metadata.
	pypiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/pypi/requests/json" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"info":{"name":"requests","version":"2.33.1"},"urls":[],"releases":{}}`)
			return
		}
		http.NotFound(w, r)
	}))
	defer pypiSrv.Close()

	// Mock sidecar returning docs JSON.
	sidecarSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/generate" && r.Method == http.MethodPost {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{
				"empty": false,
				"reason": "",
				"modules": [{
					"name": "requests",
					"functions": [{"name":"get","kind":"function","signature":"def get(url: str)","docstring":"Sends a GET request.","parameters":[{"name":"url","type":"str","description":"The URL."}],"returns":{"type":"Response","description":""}}],
					"classes": [],
					"exceptions": []
				}]
			}`)
			return
		}
		http.NotFound(w, r)
	}))
	defer sidecarSrv.Close()

	sqliteCache, err := cache.New(":memory:")
	if err != nil {
		t.Fatalf("cache.New: %v", err)
	}
	defer sqliteCache.Close()
	memCache := cache.NewMemoryCache(sqliteCache, 100)

	pypiClient := pypi.NewClient(pypi.WithBaseURL(pypiSrv.URL))
	h := handler.NewDocsHandler(pypiClient, memCache, sidecarSrv.URL)

	r := chi.NewRouter()
	r.Get("/api/packages/{name}/docs", h.Get)

	t.Run("returns docs for available package", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/packages/requests/docs", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
		}
		var resp handler.DocsResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if !resp.Available {
			t.Error("Available should be true")
		}
		if resp.Package != "requests" {
			t.Errorf("Package = %q, want requests", resp.Package)
		}
		if resp.Version != "2.33.1" {
			t.Errorf("Version = %q, want 2.33.1", resp.Version)
		}
		if len(resp.Modules) == 0 {
			t.Error("Modules should not be empty")
		}
	})

	t.Run("second request served from cache", func(t *testing.T) {
		// Shut down the sidecar — cache should serve the second request.
		sidecarSrv.Close()

		req := httptest.NewRequest(http.MethodGet, "/api/packages/requests/docs", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (expected cache hit)", w.Code)
		}
	})
}

func TestDocsHandlerSidecarUnavailable(t *testing.T) {
	pypiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"info":{"name":"mypkg","version":"1.0.0"},"urls":[],"releases":{}}`)
	}))
	defer pypiSrv.Close()

	sqliteCache, err := cache.New(":memory:")
	if err != nil {
		t.Fatalf("cache.New: %v", err)
	}
	defer sqliteCache.Close()
	memCache := cache.NewMemoryCache(sqliteCache, 100)

	pypiClient := pypi.NewClient(pypi.WithBaseURL(pypiSrv.URL))
	// Point at a port nothing is listening on.
	h := handler.NewDocsHandler(pypiClient, memCache, "http://127.0.0.1:19999")

	r := chi.NewRouter()
	r.Get("/api/packages/{name}/docs", h.Get)

	req := httptest.NewRequest(http.MethodGet, "/api/packages/mypkg/docs", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", w.Code)
	}
}
```

- [ ] **Step 2: Run the tests to confirm they fail**

```bash
cd api && go test ./internal/handler/... -run TestDocs -v
```
Expected: compile error — `handler.NewDocsHandler` and `handler.DocsResponse` undefined.

- [ ] **Step 3: Write the docs handler**

```go
// api/internal/handler/docs.go
package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pypx/api/internal/cache"
	"github.com/pypx/api/internal/pypi"
)

// DocsResponse is the response for GET /api/packages/{name}/docs.
type DocsResponse struct {
	Package   string      `json:"package"`
	Version   string      `json:"version"`
	Available bool        `json:"available"`
	Modules   []DocModule `json:"modules"`
}

// DocModule is a Python module in the documentation.
type DocModule struct {
	Name       string      `json:"name"`
	Functions  []DocSymbol `json:"functions"`
	Classes    []DocSymbol `json:"classes"`
	Exceptions []DocSymbol `json:"exceptions"`
}

// DocSymbol is a single documented symbol (function, class, or exception).
type DocSymbol struct {
	Name       string     `json:"name"`
	Kind       string     `json:"kind"`
	Signature  string     `json:"signature"`
	Docstring  string     `json:"docstring"`
	Parameters []DocParam `json:"parameters,omitempty"`
	Returns    *DocReturn `json:"returns,omitempty"`
}

// DocParam is a function parameter.
type DocParam struct {
	Name        string `json:"name"`
	Type        string `json:"type,omitempty"`
	Description string `json:"description"`
}

// DocReturn is the return type annotation and description.
type DocReturn struct {
	Type        string `json:"type,omitempty"`
	Description string `json:"description"`
}

// sidecarRequest is the body sent to the docs-worker sidecar.
type sidecarRequest struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// sidecarResponse is the JSON returned by the docs-worker sidecar.
type sidecarResponse struct {
	Empty   bool        `json:"empty"`
	Reason  string      `json:"reason"`
	Modules []DocModule `json:"modules"`
}

// DocsHandler serves rendered API documentation for a package.
type DocsHandler struct {
	pypi       *pypi.Client
	cache      cache.Cacher
	sidecarURL string
	httpClient *http.Client
}

// NewDocsHandler creates a new DocsHandler.
func NewDocsHandler(pypiClient *pypi.Client, c cache.Cacher, sidecarURL string) *DocsHandler {
	return &DocsHandler{
		pypi:       pypiClient,
		cache:      c,
		sidecarURL: sidecarURL,
		httpClient: &http.Client{Timeout: 120 * time.Second},
	}
}

// Get handles GET /api/packages/{name}/docs.
func (h *DocsHandler) Get(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if !validateName(w, name) {
		return
	}

	// Resolve latest version.
	pkg, err := h.pypi.FetchPackage(name)
	if err != nil {
		http.Error(w, "package not found", http.StatusNotFound)
		return
	}
	version := pkg.Info.Version

	cacheKey := "docs:" + strings.ToLower(name) + ":" + version

	// TTL=0 means indefinite (source is immutable per version).
	if data, _, err := h.cache.Get(cacheKey, 0); err == nil && data != nil {
		w.Header().Set("Content-Type", "application/json")
		w.Write(data) //nolint:errcheck
		return
	}

	// Call the docs-worker sidecar.
	reqBody, _ := json.Marshal(sidecarRequest{Name: name, Version: version})
	resp, err := h.httpClient.Post(h.sidecarURL+"/generate", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		http.Error(w, "documentation service unavailable", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		http.Error(w, "documentation service error", http.StatusBadGateway)
		return
	}

	var sidecar sidecarResponse
	if err := json.NewDecoder(resp.Body).Decode(&sidecar); err != nil {
		http.Error(w, "failed to decode docs response", http.StatusInternalServerError)
		return
	}

	modules := sidecar.Modules
	if modules == nil {
		modules = []DocModule{}
	}

	docsResp := DocsResponse{
		Package:   name,
		Version:   version,
		Available: !sidecar.Empty,
		Modules:   modules,
	}

	encoded, err := json.Marshal(docsResp)
	if err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}

	h.cache.Set(cacheKey, encoded, 0) //nolint:errcheck

	w.Header().Set("Content-Type", "application/json")
	w.Write(encoded) //nolint:errcheck
}
```

- [ ] **Step 4: Run the docs handler tests**

```bash
cd api && go test ./internal/handler/... -run TestDocs -v
```
Expected: both test functions PASS.

- [ ] **Step 5: Run all tests**

```bash
cd api && go test ./...
```
Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add api/internal/handler/docs.go api/internal/handler/docs_test.go
git commit -m "feat(api): add docs handler — proxies griffe sidecar with indefinite cache"
```

---

## Task 7: Wire docs route and docker-compose

**Files:**
- Modify: `api/cmd/server/main.go`
- Modify: `docker-compose.yml`

- [ ] **Step 1: Wire the docs handler in main.go**

In `api/cmd/server/main.go`, add the `DOCS_WORKER_URL` env var and register the docs route.

After the `condaClient` line (around line 63), add:
```go
docsWorkerURL := os.Getenv("DOCS_WORKER_URL")
if docsWorkerURL == "" {
    docsWorkerURL = "http://localhost:8001"
}
docsHandler := handler.NewDocsHandler(pypiClient, c, docsWorkerURL)
```

After the extras route registration (around line 98), add:
```go
r.Get("/api/packages/{name}/docs", docsHandler.Get)
```

The full updated routes block should look like:
```go
r.Get("/api/health", handler.Health)
r.Get("/api/packages/{name}", pkgHandler.Get)
r.Get("/api/packages/{name}/versions", pkgHandler.GetVersions)
r.Get("/api/packages/{name}/dependencies", pkgHandler.GetDependencies)
r.Get("/api/packages/{name}/changelog", changelogHandler.Get)
r.Get("/api/packages/{name}/stats", statsHandler.Get)
r.Get("/api/packages/{name}/security", securityHandler.Get)
r.Get("/api/packages/{name}/extras", extrasHandler.Get)
r.Get("/api/packages/{name}/docs", docsHandler.Get)
r.Get("/api/search", searchHandler.Search)
r.Get("/api/popular", popularHandler.Get)
```

- [ ] **Step 2: Build to verify**

```bash
cd api && go build ./...
```
Expected: no errors.

- [ ] **Step 3: Add docs-worker to docker-compose.yml**

In `docker-compose.yml`, add the `docs-worker` service and the `DOCS_WORKER_URL` env var to `api`. The final file:

```yaml
services:
  caddy:
    image: caddy:2-alpine
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./Caddyfile:/etc/caddy/Caddyfile
      - caddy_data:/data
      - caddy_config:/config
    depends_on:
      api:
        condition: service_healthy
      web:
        condition: service_healthy
    restart: unless-stopped
    deploy:
      resources:
        limits:
          memory: 128M

  api:
    build:
      context: ./api
    environment:
      - API_PORT=8080
      - SQLITE_PATH=/data/pypx.db
      - GITHUB_TOKEN=${GITHUB_TOKEN:-}
      - DOCS_WORKER_URL=http://docs-worker:8000
    volumes:
      - api_data:/data
    expose:
      - "8080"
    depends_on:
      docs-worker:
        condition: service_started
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "-q", "--spider", "http://localhost:8080/api/health"]
      interval: 30s
      timeout: 5s
      retries: 3
      start_period: 10s
    deploy:
      resources:
        limits:
          memory: 512M

  docs-worker:
    build:
      context: ./docs-worker
    expose:
      - "8000"
    restart: unless-stopped
    deploy:
      resources:
        limits:
          memory: 512M

  web:
    build:
      context: ./web
    environment:
      - NUXT_API_BASE=http://api:8080
      - NUXT_PUBLIC_API_BASE=/api
    expose:
      - "3000"
    depends_on:
      api:
        condition: service_healthy
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "-q", "--spider", "http://localhost:3000"]
      interval: 30s
      timeout: 5s
      retries: 3
      start_period: 15s
    deploy:
      resources:
        limits:
          memory: 256M

volumes:
  caddy_data:
  caddy_config:
  api_data:
```

- [ ] **Step 4: Update docker-compose.override.yml to expose docs-worker for local dev**

In `docker-compose.override.yml`, add the docs-worker service and ensure the api's DOCS_WORKER_URL points to it. If the file doesn't exist yet, create it. Add:

```yaml
services:
  docs-worker:
    ports:
      - "8001:8000"
```

(Keep any existing override entries intact.)

- [ ] **Step 5: Commit**

```bash
git add api/cmd/server/main.go docker-compose.yml docker-compose.override.yml
git commit -m "feat(infra): wire docs handler route and docs-worker docker service"
```

---

## Task 8: Frontend types and composable

**Files:**
- Modify: `web/app/types/api.ts`
- Modify: `web/app/composables/useApi.ts`

- [ ] **Step 1: Add DocsData types to api.ts**

Append to `web/app/types/api.ts`:

```typescript
export interface DocParam {
  name: string;
  type?: string;
  description: string;
}

export interface DocReturn {
  type?: string;
  description: string;
}

export interface DocSymbol {
  name: string;
  kind: "function" | "class" | "exception";
  signature: string;
  docstring: string;
  parameters?: DocParam[];
  returns?: DocReturn | null;
}

export interface DocModule {
  name: string;
  functions: DocSymbol[];
  classes: DocSymbol[];
  exceptions: DocSymbol[];
}

export interface DocsData {
  package: string;
  version: string;
  available: boolean;
  modules: DocModule[];
}
```

- [ ] **Step 2: Add fetchDocs to useApi.ts**

In `web/app/composables/useApi.ts`, add the import for `DocsData`:

```typescript
import type {
  PackageData,
  VersionInfo,
  DependencyTree,
  StatsData,
  SearchResult,
  ChangelogData,
  SecurityData,
  ExtrasData,
  DocsData,
} from "~/types/api";
```

Add the `fetchDocs` function after `fetchExtras`:

```typescript
async function fetchDocs(name: string): Promise<DocsData> {
  return $fetch<DocsData>(`${baseURL}/packages/${name}/docs`);
}
```

Add `fetchDocs` to the return object:
```typescript
return {
  fetchPackage,
  fetchVersions,
  fetchDependencies,
  fetchStats,
  searchPackages,
  fetchChangelog,
  fetchSecurity,
  fetchExtras,
  fetchDocs,
};
```

- [ ] **Step 3: Run the Nuxt type check**

```bash
cd web && npx nuxi typecheck
```
Expected: no type errors related to the new types.

- [ ] **Step 4: Commit**

```bash
git add web/app/types/api.ts web/app/composables/useApi.ts
git commit -m "feat(web): add DocsData types and fetchDocs composable"
```

---

## Task 9: Docs tab on the package page

**Files:**
- Modify: `web/app/pages/packages/[name].vue`

The Docs tab is a `<NuxtLink>` (not an in-page tab button) that navigates to `/packages/{name}/docs`. It only appears when `docsData.value?.available` is `true`. The docs fetch runs client-side, non-blocking — the tab appears after the fetch resolves.

- [ ] **Step 1: Update [name].vue**

Replace the full content of `web/app/pages/packages/[name].vue`:

```vue
<script setup lang="ts">
const route = useRoute();
const name = computed(() => route.params.name as string);
const activeTab = ref("overview");

const api = useApi();
const { data: pkg, status } = await useAsyncData(`package-${name.value}`, () =>
  api.fetchPackage(name.value),
);

// Non-blocking parallel fetches (client-side, don't block SSR)
const { data: security } = useAsyncData(
  `security-${name.value}`,
  () => api.fetchSecurity(name.value, pkg.value?.version),
  { server: false, default: () => null },
);

const { data: extras } = useAsyncData(`extras-${name.value}`, () => api.fetchExtras(name.value), {
  server: false,
  default: () => null,
});

const { data: changelog } = useAsyncData(
  `changelog-${name.value}`,
  () => api.fetchChangelog(name.value).catch(() => null),
  { server: false, default: () => null },
);

// Docs: non-blocking fetch to determine if the Docs tab should be shown.
const { data: docsData } = useAsyncData(
  `docs-${name.value}`,
  () => api.fetchDocs(name.value).catch(() => null),
  { server: false, default: () => null },
);

const repoInfo = computed(() => changelog.value?.repo_info ?? null);

const inPageTabs = [
  { key: "overview", label: "Overview" },
  { key: "dependencies", label: "Dependencies" },
  { key: "versions", label: "Versions" },
  { key: "stats", label: "Stats" },
];

useSeoMeta({
  title: () => (pkg.value ? `${pkg.value.name} — pypx` : "Loading — pypx"),
  description: () => pkg.value?.summary || "",
  ogTitle: () => (pkg.value ? `${pkg.value.name} ${pkg.value.version}` : "pypx"),
  ogDescription: () => pkg.value?.summary || "",
});
</script>

<template>
  <div>
    <!-- Loading state -->
    <div v-if="status === 'pending'" class="flex items-center justify-center py-24">
      <div class="h-8 w-8 animate-spin rounded-full border-2 border-zinc-700 border-t-zinc-300" />
    </div>

    <!-- Error state -->
    <div v-else-if="status === 'error'" class="py-24 text-center">
      <p class="text-lg font-medium text-zinc-300">Package not found</p>
      <p class="mt-1 text-sm text-zinc-500">No package named "{{ name }}" could be found.</p>
    </div>

    <!-- Loaded state -->
    <div v-else-if="pkg">
      <!-- Header -->
      <div class="mb-6">
        <div class="flex flex-wrap items-baseline gap-3">
          <h1 class="text-3xl font-bold text-zinc-50">{{ pkg.name }}</h1>
          <span class="rounded bg-zinc-800 px-2 py-0.5 font-mono text-sm text-zinc-400">
            v{{ pkg.version }}
          </span>
        </div>
        <p v-if="pkg.summary" class="mt-2 text-zinc-400">{{ pkg.summary }}</p>
        <div class="mt-3">
          <PackageBadges :pkg="pkg" :extras="extras" :security="security" />
        </div>
      </div>

      <!-- Tabs -->
      <div class="mb-6 flex gap-1 overflow-x-auto border-b border-zinc-800 pb-0">
        <!-- In-page tabs -->
        <button
          v-for="tab in inPageTabs"
          :key="tab.key"
          class="cursor-pointer whitespace-nowrap rounded-t px-4 py-2 text-sm font-medium transition-colors"
          :class="
            activeTab === tab.key ? 'bg-zinc-800 text-zinc-50' : 'text-zinc-500 hover:text-zinc-300'
          "
          @click="activeTab = tab.key"
        >
          {{ tab.label }}
        </button>

        <!-- Docs tab — link to separate route, shown only when available -->
        <NuxtLink
          v-if="docsData?.available"
          :to="`/packages/${pkg.name}/docs`"
          class="cursor-pointer whitespace-nowrap rounded-t px-4 py-2 text-sm font-medium text-zinc-500 transition-colors hover:text-zinc-300"
        >
          Docs
        </NuxtLink>
      </div>

      <!-- Tab content -->
      <div>
        <div v-if="activeTab === 'overview'">
          <PackageOverview :pkg="pkg" :repo-info="repoInfo" />
        </div>
        <div v-else-if="activeTab === 'dependencies'">
          <PackageDependencies :name="pkg.name" :dependencies="pkg.dependencies" />
        </div>
        <div v-else-if="activeTab === 'versions'"><PackageVersions :name="pkg.name" /></div>
        <div v-else-if="activeTab === 'stats'">
          <PackageStats :name="pkg.name" />
        </div>
      </div>
    </div>
  </div>
</template>
```

- [ ] **Step 2: Run the type check**

```bash
cd web && npx nuxi typecheck
```
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add web/app/pages/packages/\[name\].vue
git commit -m "feat(web): add conditional Docs tab linking to package docs page"
```

---

## Task 10: Docs page

**Files:**
- Create: `web/app/pages/packages/[name]/docs.vue`

This is a separate Nuxt route at `/packages/:name/docs`. It shows the same package header (with Docs tab active) and the fixed sidebar + scrolling content layout.

The sidebar lists symbols grouped by kind (Functions, Classes, Exceptions). Clicking a sidebar entry smooth-scrolls to the symbol's section in the main content.

- [ ] **Step 1: Ensure the directory exists**

```bash
mkdir -p web/app/pages/packages/\[name\]
```

- [ ] **Step 2: Create the docs page**

```vue
<!-- web/app/pages/packages/[name]/docs.vue -->
<script setup lang="ts">
import type { DocSymbol } from "~/types/api";

const route = useRoute();
const name = computed(() => route.params.name as string);

const api = useApi();

const { data: pkg, status: pkgStatus } = await useAsyncData(
  `package-${name.value}`,
  () => api.fetchPackage(name.value),
);

const { data: docs, status: docsStatus } = await useAsyncData(
  `docs-${name.value}`,
  () => api.fetchDocs(name.value),
);

const allFunctions = computed<DocSymbol[]>(() =>
  docs.value?.modules?.flatMap((m) => m.functions) ?? [],
);
const allClasses = computed<DocSymbol[]>(() =>
  docs.value?.modules?.flatMap((m) => m.classes) ?? [],
);
const allExceptions = computed<DocSymbol[]>(() =>
  docs.value?.modules?.flatMap((m) => m.exceptions) ?? [],
);

const activeSymbol = ref<string | null>(null);

function scrollTo(symbolName: string) {
  activeSymbol.value = symbolName;
  const el = document.getElementById(`sym-${symbolName}`);
  if (el) {
    el.scrollIntoView({ behavior: "smooth", block: "start" });
  }
}

useSeoMeta({
  title: () => (pkg.value ? `${pkg.value.name} — API docs — pypx` : "Loading — pypx"),
  description: () => `API documentation for ${pkg.value?.name ?? name.value}`,
});
</script>

<template>
  <div>
    <!-- Loading state -->
    <div v-if="pkgStatus === 'pending'" class="flex items-center justify-center py-24">
      <div class="h-8 w-8 animate-spin rounded-full border-2 border-zinc-700 border-t-zinc-300" />
    </div>

    <!-- Error state -->
    <div v-else-if="pkgStatus === 'error'" class="py-24 text-center">
      <p class="text-lg font-medium text-zinc-300">Package not found</p>
    </div>

    <!-- Loaded -->
    <div v-else-if="pkg">
      <!-- Header -->
      <div class="mb-6">
        <div class="flex flex-wrap items-baseline gap-3">
          <NuxtLink
            :to="`/packages/${pkg.name}`"
            class="text-3xl font-bold text-zinc-50 hover:text-zinc-300 transition-colors"
          >{{ pkg.name }}</NuxtLink>
          <span class="rounded bg-zinc-800 px-2 py-0.5 font-mono text-sm text-zinc-400">
            v{{ pkg.version }}
          </span>
        </div>
        <p v-if="pkg.summary" class="mt-2 text-zinc-400">{{ pkg.summary }}</p>
      </div>

      <!-- Tab strip — Docs tab active, others link back to package page -->
      <div class="mb-6 flex gap-1 overflow-x-auto border-b border-zinc-800 pb-0">
        <NuxtLink
          v-for="tab in ['Overview', 'Dependencies', 'Versions', 'Stats']"
          :key="tab"
          :to="`/packages/${pkg.name}`"
          class="cursor-pointer whitespace-nowrap rounded-t px-4 py-2 text-sm font-medium text-zinc-500 transition-colors hover:text-zinc-300"
        >{{ tab }}</NuxtLink>
        <span
          class="cursor-default whitespace-nowrap rounded-t bg-zinc-800 px-4 py-2 text-sm font-medium text-zinc-50"
        >Docs</span>
      </div>

      <!-- Docs loading -->
      <div v-if="docsStatus === 'pending'" class="flex items-center justify-center py-16">
        <div class="h-6 w-6 animate-spin rounded-full border-2 border-zinc-700 border-t-zinc-300" />
      </div>

      <!-- Docs unavailable -->
      <div v-else-if="!docs?.available" class="py-16 text-center">
        <p class="text-zinc-400">API documentation is not available for this package.</p>
        <p class="mt-1 text-sm text-zinc-500">
          This package may be binary-only or could not be parsed.
        </p>
        <a
          v-if="pkg.doc_url"
          :href="pkg.doc_url"
          target="_blank"
          rel="noopener noreferrer"
          class="mt-4 inline-block text-sm text-[var(--color-brand)] hover:text-[var(--color-brand-light)] transition-colors"
        >View external documentation →</a>
      </div>

      <!-- Docs content: sidebar + main -->
      <div v-else class="flex gap-0 -mx-4 sm:-mx-6 lg:-mx-8">
        <!-- Fixed sidebar -->
        <div
          class="w-48 flex-shrink-0 sticky top-0 h-screen overflow-y-auto border-r border-zinc-800 bg-zinc-950 py-3 hidden md:block"
        >
          <p class="px-3 pb-2 text-[9px] font-bold uppercase tracking-widest text-zinc-600">
            Contents
          </p>

          <!-- Functions -->
          <template v-if="allFunctions.length">
            <p class="px-3 pt-2 pb-1 text-[10px] font-semibold uppercase tracking-wide text-zinc-500">
              Functions <span class="text-zinc-700">({{ allFunctions.length }})</span>
            </p>
            <button
              v-for="sym in allFunctions"
              :key="sym.name"
              class="block w-full px-4 py-1 text-left text-[11px] font-mono transition-colors"
              :class="
                activeSymbol === sym.name
                  ? 'border-r-2 border-[var(--color-brand)] bg-[var(--color-brand)]/5 text-[var(--color-brand)]'
                  : 'text-zinc-500 hover:text-zinc-300'
              "
              @click="scrollTo(sym.name)"
            >{{ sym.name }}</button>
          </template>

          <!-- Classes -->
          <template v-if="allClasses.length">
            <p class="px-3 pt-3 pb-1 text-[10px] font-semibold uppercase tracking-wide text-zinc-500">
              Classes <span class="text-zinc-700">({{ allClasses.length }})</span>
            </p>
            <button
              v-for="sym in allClasses"
              :key="sym.name"
              class="block w-full px-4 py-1 text-left text-[11px] font-mono transition-colors"
              :class="
                activeSymbol === sym.name
                  ? 'border-r-2 border-[var(--color-brand)] bg-[var(--color-brand)]/5 text-[var(--color-brand)]'
                  : 'text-zinc-500 hover:text-zinc-300'
              "
              @click="scrollTo(sym.name)"
            >{{ sym.name }}</button>
          </template>

          <!-- Exceptions -->
          <template v-if="allExceptions.length">
            <p class="px-3 pt-3 pb-1 text-[10px] font-semibold uppercase tracking-wide text-zinc-500">
              Exceptions <span class="text-zinc-700">({{ allExceptions.length }})</span>
            </p>
            <button
              v-for="sym in allExceptions"
              :key="sym.name"
              class="block w-full px-4 py-1 text-left text-[11px] font-mono transition-colors"
              :class="
                activeSymbol === sym.name
                  ? 'border-r-2 border-[var(--color-brand)] bg-[var(--color-brand)]/5 text-[var(--color-brand)]'
                  : 'text-zinc-500 hover:text-zinc-300'
              "
              @click="scrollTo(sym.name)"
            >{{ sym.name }}</button>
          </template>
        </div>

        <!-- Main content -->
        <div class="flex-1 min-w-0 px-6 py-5">
          <template v-for="mod in docs.modules" :key="mod.name">
            <template
              v-for="sym in [...mod.functions, ...mod.classes, ...mod.exceptions]"
              :key="sym.name"
            >
              <div :id="`sym-${sym.name}`" class="mb-10 scroll-mt-4">
                <!-- Symbol name + kind badge -->
                <div class="mb-3 flex items-center gap-2">
                  <span class="font-mono text-base font-bold text-zinc-50">{{ sym.name }}</span>
                  <span
                    class="rounded px-1.5 py-0.5 text-[9px] font-semibold uppercase tracking-wide"
                    :class="{
                      'bg-blue-950 text-blue-300': sym.kind === 'function',
                      'bg-purple-950 text-purple-300': sym.kind === 'class',
                      'bg-red-950 text-red-300': sym.kind === 'exception',
                    }"
                  >{{ sym.kind }}</span>
                </div>

                <!-- Signature -->
                <div
                  class="mb-3 rounded-md border border-zinc-800 bg-zinc-900 px-4 py-2.5 font-mono text-[11px] leading-relaxed text-violet-300"
                >{{ sym.signature }}</div>

                <!-- Docstring -->
                <p
                  v-if="sym.docstring"
                  class="mb-3 text-sm leading-relaxed text-zinc-400"
                >{{ sym.docstring }}</p>

                <!-- Parameters -->
                <div v-if="sym.parameters && sym.parameters.length" class="mb-3">
                  <p class="mb-2 text-[9px] font-bold uppercase tracking-widest text-zinc-600">
                    Parameters
                  </p>
                  <div class="border-l-2 border-zinc-800 pl-3 space-y-2">
                    <div v-for="param in sym.parameters" :key="param.name">
                      <span class="font-mono text-[11px] text-sky-400">{{ param.name }}</span>
                      <span v-if="param.type" class="ml-1.5 text-[10px] text-zinc-600">{{ param.type }}</span>
                      <p v-if="param.description" class="mt-0.5 text-[11px] text-zinc-500">
                        {{ param.description }}
                      </p>
                    </div>
                  </div>
                </div>

                <!-- Returns -->
                <div v-if="sym.returns" class="mb-3">
                  <p class="mb-1 text-[9px] font-bold uppercase tracking-widest text-zinc-600">
                    Returns
                  </p>
                  <span v-if="sym.returns.type" class="font-mono text-[11px] text-sky-400">{{ sym.returns.type }}</span>
                  <span v-if="sym.returns.description" class="ml-2 text-[11px] text-zinc-500">{{ sym.returns.description }}</span>
                </div>

                <div class="mt-8 border-t border-zinc-900" />
              </div>
            </template>
          </template>
        </div>
      </div>
    </div>
  </div>
</template>
```

- [ ] **Step 3: Run the type check**

```bash
cd web && npx nuxi typecheck
```
Expected: no errors.

- [ ] **Step 4: Start the dev server and smoke test**

```bash
# Start API (with docs-worker running or DOCS_WORKER_URL pointing to a running sidecar)
cd api && go run ./cmd/server &
# Start Nuxt dev
cd web && npm run dev
```

Navigate to `http://localhost:3000/packages/requests` — the Docs tab should appear after the non-blocking fetch (about 10–30 seconds for first load as the sidecar processes the wheel). Click the Docs tab to navigate to `/packages/requests/docs`.

On the docs page:
- Sidebar should show Functions, Classes, Exceptions grouped correctly
- Clicking a sidebar entry should smooth-scroll to the corresponding section
- The Docs tab in the tab strip should appear active (non-link style)
- Other tabs should link back to `/packages/requests`

If the sidecar is not running, the docs page should show the "not available" message instead of erroring.

- [ ] **Step 5: Commit**

```bash
git add web/app/pages/packages/\[name\]/docs.vue
git commit -m "feat(web): add API docs page with fixed sidebar and symbol sections"
```

---

## Final verification

- [ ] **Run all Go tests**

```bash
cd api && go test ./... -v
```
Expected: all PASS.

- [ ] **Run Nuxt type check**

```bash
cd web && npx nuxi typecheck
```
Expected: no errors.

- [ ] **Build both containers**

```bash
docker compose build api docs-worker web
```
Expected: all three build successfully.

- [ ] **Smoke test with docker compose**

```bash
docker compose up -d
sleep 15
curl -s http://localhost:8080/api/packages/requests/extras | python3 -m json.tool | grep status
# Expected: "status": "typed" (or "stubs" depending on the package)

curl -s http://localhost:8080/api/packages/requests/docs | python3 -m json.tool | head -5
# Expected: {"available": true, ...} or 502 if sidecar is still warming up

curl -s http://localhost:8080/api/packages/pillow/json | python3 -m json.tool | grep description_content_type
# If RST, verify description_html is populated in GET /api/packages/pillow
```

- [ ] **Final commit if any cleanup needed**

```bash
git add -p  # review any uncommitted changes
git commit -m "chore: phase 2a final cleanup"
```
