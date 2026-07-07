package wheel

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Unit tests for helpers
// ---------------------------------------------------------------------------

func TestNormalizeName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"requests", "requests"},
		{"My-Package", "my_package"},
		{"my.package", "my_package"},
		{"My-Cool.Package", "my_cool_package"},
		{"UPPER", "upper"},
	}
	for _, tt := range tests {
		got := NormalizeName(tt.input)
		if got != tt.want {
			t.Errorf("NormalizeName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestParseTopLevelTxt(t *testing.T) {
	tests := []struct {
		content string
		want    int
	}{
		{"requests\n", 1},
		{"pkg1\npkg2\n", 2},
		{"  spaced  \n", 1},
		{"", 0},
		{"\n\n", 0},
	}
	for _, tt := range tests {
		got := parseTopLevelTxt(tt.content)
		if len(got) != tt.want {
			t.Errorf("parseTopLevelTxt(%q) = %v (len %d), want len %d", tt.content, got, len(got), tt.want)
		}
	}
}

func TestSelectWheel(t *testing.T) {
	wheels := []WheelFile{
		{Filename: "pkg-1.0-cp39-linux.whl", URL: "https://example.com/linux.whl"},
		{Filename: "pkg-1.0-py3-none-any.whl", URL: "https://example.com/any.whl"},
	}
	got := selectWheel(wheels)
	if got != "https://example.com/any.whl" {
		t.Errorf("selectWheel() = %q, want any.whl URL", got)
	}
}

func TestSelectWheelFallback(t *testing.T) {
	wheels := []WheelFile{
		{Filename: "pkg-1.0-cp39-linux.whl", URL: "https://example.com/linux.whl"},
	}
	got := selectWheel(wheels)
	if got != "https://example.com/linux.whl" {
		t.Errorf("selectWheel() = %q, want fallback URL", got)
	}
}

func TestSelectWheel_Deterministic(t *testing.T) {
	tests := []struct {
		name   string
		wheels []WheelFile
		want   string
	}{
		{
			name: "pure none-any beats platform wheels",
			wheels: []WheelFile{
				{Filename: "pkg-1.0-cp39-linux.whl", URL: "linux", Size: 10},
				{Filename: "pkg-1.0-py3-none-any.whl", URL: "any", Size: 999},
				{Filename: "pkg-1.0-cp310-macos.whl", URL: "macos", Size: 5},
			},
			want: "any",
		},
		{
			name: "platform-only, smallest size wins",
			wheels: []WheelFile{
				{Filename: "pkg-1.0-cp39-linux.whl", URL: "linux", Size: 300},
				{Filename: "pkg-1.0-cp310-macos.whl", URL: "macos", Size: 100},
				{Filename: "pkg-1.0-cp311-win.whl", URL: "win", Size: 200},
			},
			want: "macos",
		},
		{
			name: "platform-only equal sizes, lexicographically smallest filename wins",
			wheels: []WheelFile{
				{Filename: "pkg-1.0-cp39-linux.whl", URL: "linux", Size: 100},
				{Filename: "pkg-1.0-cp310-macos.whl", URL: "macos", Size: 100},
				{Filename: "pkg-1.0-cp311-win.whl", URL: "win", Size: 100},
			},
			want: "macos", // "pkg-1.0-cp310-..." < "pkg-1.0-cp311-..." < "pkg-1.0-cp39-..."
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Forward order.
			if got := selectWheel(tt.wheels); got != tt.want {
				t.Errorf("selectWheel(forward) = %q, want %q", got, tt.want)
			}
			// Reversed order must produce the same result.
			rev := make([]WheelFile, len(tt.wheels))
			for i, w := range tt.wheels {
				rev[len(tt.wheels)-1-i] = w
			}
			if got := selectWheel(rev); got != tt.want {
				t.Errorf("selectWheel(reversed) = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestInferTopLevel(t *testing.T) {
	files := map[string][]byte{
		"mypkg/__init__.py":            {},
		"mypkg/mod.py":                 {},
		"mypkg-1.0.dist-info/METADATA": {},
	}
	got := inferTopLevel(files, "mypkg")
	if len(got) != 1 || got[0] != "mypkg" {
		t.Errorf("inferTopLevel() = %v, want [mypkg]", got)
	}
}

// ---------------------------------------------------------------------------
// extractPyFiles tests using crafted zip archives
// ---------------------------------------------------------------------------

func buildTestWheel(files map[string]string) []byte {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, content := range files {
		f, _ := w.Create(name)
		_, _ = f.Write([]byte(content))
	}
	_ = w.Close()
	return buf.Bytes()
}

func TestExtractPyFiles_Basic(t *testing.T) {
	data := buildTestWheel(map[string]string{
		"mypkg/__init__.py":                      `"""My package."""`,
		"mypkg/core.py":                          `def hello(): pass`,
		"mypkg-1.0.dist-info/top_level.txt":      "mypkg\n",
		"mypkg-1.0.dist-info/METADATA":           "Name: mypkg\nVersion: 1.0",
		"mypkg/__pycache__/core.cpython-311.pyc": "binary",
	})

	contents, err := extractPyFiles(data, "mypkg")
	if err != nil {
		t.Fatalf("extractPyFiles: %v", err)
	}

	if len(contents.TopLevelPkgs) != 1 || contents.TopLevelPkgs[0] != "mypkg" {
		t.Errorf("TopLevelPkgs = %v, want [mypkg]", contents.TopLevelPkgs)
	}
	if len(contents.Files) != 2 {
		t.Errorf("Files count = %d, want 2 (should exclude __pycache__)", len(contents.Files))
	}
	if _, ok := contents.Files["mypkg/__init__.py"]; !ok {
		t.Error("missing mypkg/__init__.py")
	}
	if _, ok := contents.Files["mypkg/core.py"]; !ok {
		t.Error("missing mypkg/core.py")
	}
}

func TestExtractPyFiles_NoTopLevel(t *testing.T) {
	data := buildTestWheel(map[string]string{
		"mypkg/__init__.py": `"""Package."""`,
		"mypkg/mod.py":      `x = 1`,
	})

	contents, err := extractPyFiles(data, "mypkg")
	if err != nil {
		t.Fatalf("extractPyFiles: %v", err)
	}

	// Should infer top-level from directory structure.
	if len(contents.TopLevelPkgs) == 0 {
		t.Error("TopLevelPkgs should be inferred")
	}
}

func TestExtractPyFiles_InvalidZip(t *testing.T) {
	_, err := extractPyFiles([]byte("not a zip"), "mypkg")
	if err == nil {
		t.Error("expected error for invalid zip")
	}
}

// ---------------------------------------------------------------------------
// Fetch integration tests using httptest mock servers
// ---------------------------------------------------------------------------

func TestFetch_Success(t *testing.T) {
	wheelData := buildTestWheel(map[string]string{
		"testpkg/__init__.py":                 "def hello(): pass\n",
		"testpkg/core.py":                     "class Foo: pass\n",
		"testpkg-1.0.dist-info/top_level.txt": "testpkg\n",
	})

	// Mock wheel download server.
	wheelSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(wheelData)
	}))
	defer wheelSrv.Close()

	// Mock PyPI JSON API that returns the wheel server URL.
	pypiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/pypi/testpkg/1.0.0/json" {
			resp := fmt.Sprintf(`{"urls":[{"filename":"testpkg-1.0.0-py3-none-any.whl","url":"%s/testpkg-1.0.0-py3-none-any.whl","size":%d,"packagetype":"bdist_wheel"}]}`,
				wheelSrv.URL, len(wheelData))
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(resp))
			return
		}
		http.NotFound(w, r)
	}))
	defer pypiSrv.Close()

	src := &Source{
		HTTPClient: pypiSrv.Client(),
		MaxSize:    DefaultMaxSize,
		BaseURL:    pypiSrv.URL,
	}

	ctx := context.Background()
	contents, err := src.Fetch(ctx, "testpkg", "1.0.0")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(contents.Files) != 2 {
		t.Errorf("Files count = %d, want 2", len(contents.Files))
	}
	if len(contents.TopLevelPkgs) != 1 || contents.TopLevelPkgs[0] != "testpkg" {
		t.Errorf("TopLevelPkgs = %v", contents.TopLevelPkgs)
	}
}

func TestFetch_NoWheels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"urls":[]}`))
	}))
	defer srv.Close()

	src := &Source{
		HTTPClient: srv.Client(),
		MaxSize:    DefaultMaxSize,
		BaseURL:    srv.URL,
	}

	_, err := src.Fetch(context.Background(), "empty", "1.0")
	if err == nil {
		t.Error("expected error for package with no wheels")
	}
	if !errors.Is(err, ErrNoArtifact) {
		t.Errorf("expected ErrNoArtifact, got: %v", err)
	}
}

func TestFetch_SdistOnly(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"urls":[{"filename":"pkg-1.0.tar.gz","url":"https://example.com/tar.gz","size":100,"packagetype":"sdist"}]}`))
	}))
	defer srv.Close()

	src := &Source{
		HTTPClient: srv.Client(),
		MaxSize:    DefaultMaxSize,
		BaseURL:    srv.URL,
	}

	_, err := src.Fetch(context.Background(), "sdistonly", "1.0")
	if !errors.Is(err, ErrNoArtifact) {
		t.Errorf("expected ErrNoArtifact for sdist-only release, got: %v", err)
	}
}

func TestFetch_PyPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	src := &Source{
		HTTPClient: srv.Client(),
		MaxSize:    DefaultMaxSize,
		BaseURL:    srv.URL,
	}

	_, err := src.Fetch(context.Background(), "nonexistent", "1.0")
	if err == nil {
		t.Error("expected error for 404 response")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

func TestFetch_TooLarge(t *testing.T) {
	// Mock wheel server whose HEAD advertises a size beyond MaxSize.
	wheelSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.Header().Set("Content-Length", "999")
			w.WriteHeader(http.StatusOK)
			return
		}
		w.Write([]byte("ignored"))
	}))
	defer wheelSrv.Close()

	pypiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/pypi/big/1.0/json" {
			resp := fmt.Sprintf(`{"urls":[{"filename":"big-1.0-py3-none-any.whl","url":"%s/big.whl","size":999,"packagetype":"bdist_wheel"}]}`, wheelSrv.URL)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(resp))
			return
		}
		http.NotFound(w, r)
	}))
	defer pypiSrv.Close()

	src := &Source{HTTPClient: pypiSrv.Client(), MaxSize: 10, BaseURL: pypiSrv.URL}
	_, err := src.Fetch(context.Background(), "big", "1.0")
	if !errors.Is(err, ErrTooLarge) {
		t.Errorf("expected ErrTooLarge, got: %v", err)
	}
}

func TestFetchWheelURLs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"urls": [
				{"filename": "pkg-1.0.tar.gz", "url": "https://example.com/tar.gz", "size": 100, "packagetype": "sdist"},
				{"filename": "pkg-1.0-py3-none-any.whl", "url": "https://example.com/any.whl", "size": 200, "packagetype": "bdist_wheel"},
				{"filename": "pkg-1.0-cp39-linux.whl", "url": "https://example.com/linux.whl", "size": 300, "packagetype": "bdist_wheel"}
			]
		}`))
	}))
	defer srv.Close()

	src := &Source{
		HTTPClient: srv.Client(),
		MaxSize:    DefaultMaxSize,
		BaseURL:    srv.URL,
	}

	wheels, err := src.fetchWheelURLs(context.Background(), "pkg", "1.0")
	if err != nil {
		t.Fatalf("fetchWheelURLs: %v", err)
	}
	if len(wheels) != 2 {
		t.Fatalf("wheels count = %d, want 2 (sdist should be filtered)", len(wheels))
	}
	if wheels[0].Filename != "pkg-1.0-py3-none-any.whl" {
		t.Errorf("wheels[0].Filename = %q", wheels[0].Filename)
	}
}

func TestFetchWheelURLs_EscapesPath(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		http.NotFound(w, r)
	}))
	defer srv.Close()

	src := &Source{
		HTTPClient: srv.Client(),
		MaxSize:    DefaultMaxSize,
		BaseURL:    srv.URL,
	}

	// The error is expected (mock 404s any path); only the observed escaped
	// path matters here.
	_, _ = src.fetchWheelURLs(context.Background(), "testpkg", "1.0/x")

	want := "/pypi/testpkg/1.0%2Fx/json"
	if gotPath != want {
		t.Errorf("escaped path = %q, want %q", gotPath, want)
	}
}

// ---------------------------------------------------------------------------
// Decompression-budget and truncation-detection tests
// ---------------------------------------------------------------------------

func TestExtractPyFiles_DecompressionBudget(t *testing.T) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, _ := w.Create("bomb/huge.py")
	big := bytes.Repeat([]byte("A"), maxDecompressedFile+1024)
	_, _ = f.Write(big)
	_ = w.Close()

	_, err := extractPyFiles(buf.Bytes(), "bomb")
	if err == nil {
		t.Fatal("expected an error when a .py entry exceeds the decompression budget")
	}
}

func TestFetch_TruncatedDownloadRejected(t *testing.T) {
	const maxSize = 1024
	body := bytes.Repeat([]byte("x"), maxSize*2)

	wheelSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			// No Content-Length -> resp.ContentLength == -1, headSize pre-check skipped.
			w.WriteHeader(http.StatusOK)
			return
		}
		w.Write(body) // body is >= maxSize, so io.LimitReader truncates to exactly maxSize
	}))
	defer wheelSrv.Close()

	pypiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/pypi/big/1.0/json" {
			resp := fmt.Sprintf(`{"urls":[{"filename":"big-1.0-py3-none-any.whl","url":"%s/big.whl","size":0,"packagetype":"bdist_wheel"}]}`, wheelSrv.URL)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(resp))
			return
		}
		http.NotFound(w, r)
	}))
	defer pypiSrv.Close()

	src := &Source{HTTPClient: pypiSrv.Client(), MaxSize: maxSize, BaseURL: pypiSrv.URL}
	_, err := src.Fetch(context.Background(), "big", "1.0")
	if err == nil {
		t.Fatal("expected an error for a download truncated at MaxSize")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("expected the post-download truncation error, got: %v", err)
	}
}
