package wheel

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
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
	got := selectWheel(wheels).URL
	if got != "https://example.com/any.whl" {
		t.Errorf("selectWheel() = %q, want any.whl URL", got)
	}
}

func TestSelectWheelFallback(t *testing.T) {
	wheels := []WheelFile{
		{Filename: "pkg-1.0-cp39-linux.whl", URL: "https://example.com/linux.whl"},
	}
	got := selectWheel(wheels).URL
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
			if got := selectWheel(tt.wheels).URL; got != tt.want {
				t.Errorf("selectWheel(forward) = %q, want %q", got, tt.want)
			}
			// Reversed order must produce the same result.
			rev := make([]WheelFile, len(tt.wheels))
			for i, w := range tt.wheels {
				rev[len(tt.wheels)-1-i] = w
			}
			if got := selectWheel(rev).URL; got != tt.want {
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

func TestInferTopLevel_SingleFileModule(t *testing.T) {
	files := map[string][]byte{
		"six.py": {},
	}
	got := inferTopLevel(files, "six")
	if len(got) != 1 || got[0] != "six" {
		t.Errorf("inferTopLevel() = %v, want [six]", got)
	}
}

// ---------------------------------------------------------------------------
// extractPyFiles tests using crafted zip archives
// ---------------------------------------------------------------------------

func extractBytes(data []byte, pkgName string) (*WheelContents, error) {
	return extractPyFiles(bytes.NewReader(data), int64(len(data)), pkgName)
}

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

	contents, err := extractBytes(data, "mypkg")
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

	contents, err := extractBytes(data, "mypkg")
	if err != nil {
		t.Fatalf("extractPyFiles: %v", err)
	}

	// Should infer top-level from directory structure.
	if len(contents.TopLevelPkgs) == 0 {
		t.Error("TopLevelPkgs should be inferred")
	}
}

func TestExtractPyFiles_VendoredTopLevelIgnored(t *testing.T) {
	data := buildTestWheel(map[string]string{
		"realpkg/__init__.py":                              `x = 1`,
		"realpkg/core.py":                                  `y = 2`,
		"realpkg-1.0.dist-info/top_level.txt":              "realpkg\n",
		"realpkg/_vendor/zipp/__init__.py":                 `z = 3`,
		"realpkg/_vendor/zipp-3.0.dist-info/top_level.txt": "zipp\n",
		"realpkg/_vendor/zzz-1.0.dist-info/top_level.txt":  "zzz\n",
	})
	contents, err := extractBytes(data, "realpkg")
	if err != nil {
		t.Fatalf("extractPyFiles: %v", err)
	}
	if len(contents.TopLevelPkgs) != 1 || contents.TopLevelPkgs[0] != "realpkg" {
		t.Errorf("TopLevelPkgs = %v, want [realpkg]", contents.TopLevelPkgs)
	}
}

func TestExtractPyFiles_StaleTopLevelFallsBack(t *testing.T) {
	data := buildTestWheel(map[string]string{
		"mypkg/__init__.py":                 `x = 1`,
		"mypkg-1.0.dist-info/top_level.txt": "ghost\n",
	})
	contents, err := extractBytes(data, "mypkg")
	if err != nil {
		t.Fatalf("extractPyFiles: %v", err)
	}
	if len(contents.TopLevelPkgs) != 1 || contents.TopLevelPkgs[0] != "mypkg" {
		t.Errorf("TopLevelPkgs = %v, want [mypkg]", contents.TopLevelPkgs)
	}
}

func TestExtractPyFiles_InvalidZip(t *testing.T) {
	_, err := extractBytes([]byte("not a zip"), "mypkg")
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
		if r.Method != http.MethodGet {
			t.Errorf("unexpected %s request; only GET is allowed", r.Method)
		}
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
	// The size comes from PyPI's JSON, so the wheel server must see no request.
	wheelSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected %s request to wheel server", r.Method)
	}))
	defer wheelSrv.Close()

	pypiSrv := newPyPIServer(t, "big", "1.0", wheelSrv.URL+"/big.whl", 999)
	defer pypiSrv.Close()

	src := &Source{HTTPClient: pypiSrv.Client(), MaxSize: 10, BaseURL: pypiSrv.URL}
	_, err := src.Fetch(context.Background(), "big", "1.0")
	if !errors.Is(err, ErrTooLarge) {
		t.Errorf("expected ErrTooLarge, got: %v", err)
	}
}

func TestFetch_ExactMaxSizeAccepted(t *testing.T) {
	data := buildTestWheel(map[string]string{
		"exact/__init__.py": "x = 1\n",
	})
	wheelSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("unexpected %s request", r.Method)
		}
		_, _ = w.Write(data)
	}))
	defer wheelSrv.Close()
	// Size 0 in the JSON: the limit is enforced on the download itself.
	pypiSrv := newPyPIServer(t, "exact", "1.0", wheelSrv.URL+"/exact.whl", 0)
	defer pypiSrv.Close()

	src := &Source{HTTPClient: pypiSrv.Client(), MaxSize: int64(len(data)), BaseURL: pypiSrv.URL}
	if _, err := src.Fetch(context.Background(), "exact", "1.0"); err != nil {
		t.Fatalf("wheel of exactly MaxSize bytes rejected: %v", err)
	}

	src.MaxSize = int64(len(data)) - 1
	_, err := src.Fetch(context.Background(), "exact", "1.0")
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("expected ErrTooLarge for MaxSize-1, got: %v", err)
	}
}

// newPyPIServer serves a PyPI JSON response with one wheel.
func newPyPIServer(t *testing.T, name, version, wheelURL string, size int64) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == fmt.Sprintf("/pypi/%s/%s/json", name, version) {
			resp := fmt.Sprintf(`{"urls":[{"filename":"%s-%s-py3-none-any.whl","url":"%s","size":%d,"packagetype":"bdist_wheel"}]}`,
				name, version, wheelURL, size)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(resp))
			return
		}
		http.NotFound(w, r)
	}))
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

	_, err := extractBytes(buf.Bytes(), "bomb")
	if err == nil {
		t.Fatal("expected an error when a .py entry exceeds the decompression budget")
	}
}

func TestFetch_TruncatedDownloadRejected(t *testing.T) {
	const maxSize = 1024
	body := bytes.Repeat([]byte("x"), maxSize*2)

	wheelSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(body) // longer than maxSize, size unknown in the JSON
	}))
	defer wheelSrv.Close()

	pypiSrv := newPyPIServer(t, "big", "1.0", wheelSrv.URL+"/big.whl", 0)
	defer pypiSrv.Close()

	src := &Source{HTTPClient: pypiSrv.Client(), MaxSize: maxSize, BaseURL: pypiSrv.URL}
	_, err := src.Fetch(context.Background(), "big", "1.0")
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("expected ErrTooLarge for an oversized download, got: %v", err)
	}
}

func TestInferTopLevel_Deterministic(t *testing.T) {
	files := map[string][]byte{
		"zeta/__init__.py":           nil,
		"alpha/__init__.py":          nil,
		"mid.py":                     nil,
		"beta/x.py":                  nil,
		"pkg-1.0.dist-info/METADATA": nil,
	}
	want := []string{"alpha", "beta", "mid", "zeta"}
	for i := 0; i < 30; i++ {
		got := inferTopLevel(files, "pkg")
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("run %d: got %v, want %v", i, got, want)
		}
	}
}

// ---------------------------------------------------------------------------
// Range reader and ranged Fetch tests
// ---------------------------------------------------------------------------

// buildBigWheel builds a wheel with the given text files plus one stored,
// incompressible binary entry of blobSize bytes placed between the files.
func buildBigWheel(t *testing.T, files map[string]string, blobSize int) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for i, name := range names {
		f, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = f.Write([]byte(files[name]))
		if i == 0 {
			blob := make([]byte, blobSize)
			_, _ = rand.New(rand.NewSource(1)).Read(blob)
			bf, err := w.CreateHeader(&zip.FileHeader{Name: "bigpkg/_native.so", Method: zip.Store})
			if err != nil {
				t.Fatal(err)
			}
			_, _ = bf.Write(blob)
		}
	}
	_ = w.Close()
	return buf.Bytes()
}

// rangeServer serves data and counts requests and body bytes.
type rangeServer struct {
	*httptest.Server
	mu       sync.Mutex
	requests int
	ranged   int
	bytes    int64
	other    []string // methods of requests without a Range header
}

func newRangeServer(t *testing.T, data []byte, honourRange bool) *rangeServer {
	t.Helper()
	rs := &rangeServer{}
	rs.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rs.mu.Lock()
		rs.requests++
		if r.Header.Get("Range") != "" {
			rs.ranged++
		} else {
			rs.other = append(rs.other, r.Method)
		}
		rs.mu.Unlock()
		cw := &countWriter{ResponseWriter: w}
		if honourRange {
			http.ServeContent(cw, r, "x.whl", time.Time{}, bytes.NewReader(data))
		} else {
			_, _ = cw.Write(data)
		}
		rs.mu.Lock()
		rs.bytes += cw.n
		rs.mu.Unlock()
	}))
	t.Cleanup(rs.Close)
	return rs
}

type countWriter struct {
	http.ResponseWriter
	n int64
}

func (c *countWriter) Write(p []byte) (int, error) {
	n, err := c.ResponseWriter.Write(p)
	c.n += int64(n)
	return n, err
}

func TestHTTPReaderAt_ReadsMatchBytes(t *testing.T) {
	data := make([]byte, 5*1024*1024+12345)
	_, _ = rand.New(rand.NewSource(2)).Read(data)
	rs := newRangeServer(t, data, true)

	ra, err := newHTTPReaderAt(context.Background(), rs.Client(), rs.URL, int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	rng := rand.New(rand.NewSource(3))
	for i := 0; i < 200; i++ {
		off := rng.Int63n(int64(len(data)))
		n := rng.Intn(100_000) + 1
		p := make([]byte, n)
		got, err := ra.ReadAt(p, off)
		want := data[off:min(off+int64(n), int64(len(data)))]
		if got != len(want) || !bytes.Equal(p[:got], want) {
			t.Fatalf("read %d@%d: got %d bytes, mismatch (want %d)", n, off, got, len(want))
		}
		if got < n && err != io.EOF {
			t.Fatalf("short read without io.EOF: %v", err)
		}
	}
	// Read across the very end.
	p := make([]byte, 100)
	n, err := ra.ReadAt(p, int64(len(data))-10)
	if n != 10 || err != io.EOF || !bytes.Equal(p[:10], data[len(data)-10:]) {
		t.Fatalf("end read: n=%d err=%v", n, err)
	}
	if _, err := ra.ReadAt(p, int64(len(data))); err != io.EOF {
		t.Fatalf("read at size: %v", err)
	}
	maxBlocks := len(data)/rangeBlockSize + 2
	if rs.requests > maxBlocks {
		t.Errorf("requests = %d, want at most one per block (%d)", rs.requests, maxBlocks)
	}
}

func TestHTTPReaderAt_SequentialReadsCoalesce(t *testing.T) {
	data := make([]byte, 6*1024*1024)
	_, _ = rand.New(rand.NewSource(4)).Read(data)
	rs := newRangeServer(t, data, true)

	ra, err := newHTTPReaderAt(context.Background(), rs.Client(), rs.URL, int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	before := rs.requests
	buf := make([]byte, 4096)
	for off := int64(0); off < 4*1024*1024; off += int64(len(buf)) {
		if _, err := ra.ReadAt(buf, off); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(buf, data[off:off+int64(len(buf))]) {
			t.Fatalf("mismatch at %d", off)
		}
	}
	// 4 MiB in 256 KiB blocks is 16 blocks; doubling readahead needs far fewer requests.
	if got := rs.requests - before; got > 6 {
		t.Errorf("sequential 4 MiB read took %d requests, want at most 6", got)
	}
}

func TestHTTPReaderAt_RejectsNonRangeResponses(t *testing.T) {
	data := bytes.Repeat([]byte("z"), 3*1024*1024)

	t.Run("200 instead of 206", func(t *testing.T) {
		rs := newRangeServer(t, data, false)
		_, err := newHTTPReaderAt(context.Background(), rs.Client(), rs.URL, int64(len(data)))
		if !errors.Is(err, errRangeUnsupported) {
			t.Fatalf("want errRangeUnsupported, got %v", err)
		}
	})

	t.Run("wrong Content-Range", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Range", "bytes 0-9/10")
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write(data[:10])
		}))
		defer srv.Close()
		_, err := newHTTPReaderAt(context.Background(), srv.Client(), srv.URL, int64(len(data)))
		if !errors.Is(err, errRangeUnsupported) {
			t.Fatalf("want errRangeUnsupported, got %v", err)
		}
	})

	t.Run("missing Content-Range", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write(data)
		}))
		defer srv.Close()
		_, err := newHTTPReaderAt(context.Background(), srv.Client(), srv.URL, int64(len(data)))
		if !errors.Is(err, errRangeUnsupported) {
			t.Fatalf("want errRangeUnsupported, got %v", err)
		}
	})
}

func rangeTestFiles() map[string]string {
	return map[string]string{
		"bigpkg/__init__.py":                      "\"\"\"Big package.\"\"\"\n",
		"bigpkg/core.py":                          "def run(): pass\n",
		"bigpkg/util.py":                          "def helper(): pass\n",
		"bigpkg-1.0.dist-info/top_level.txt":      "bigpkg\n",
		"bigpkg-1.0.dist-info/METADATA":           "Name: bigpkg\n",
		"bigpkg/__pycache__/core.cpython-311.pyc": "binary",
	}
}

func TestFetch_RangeLargeWheel(t *testing.T) {
	data := buildBigWheel(t, rangeTestFiles(), 9*1024*1024)
	if len(data) <= rangeThreshold {
		t.Fatalf("test wheel is %d bytes, must exceed the %d byte threshold", len(data), rangeThreshold)
	}
	rs := newRangeServer(t, data, true)
	pypiSrv := newPyPIServer(t, "bigpkg", "1.0", rs.URL+"/bigpkg.whl", int64(len(data)))
	defer pypiSrv.Close()

	// MaxSize below the wheel size proves the ranged path has its own limit.
	src := &Source{HTTPClient: pypiSrv.Client(), MaxSize: 1024, MaxRangeSize: DefaultMaxRangeSize, BaseURL: pypiSrv.URL}
	contents, err := src.Fetch(context.Background(), "bigpkg", "1.0")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(contents.Files) != 3 {
		t.Errorf("Files = %d, want 3", len(contents.Files))
	}
	if string(contents.Files["bigpkg/core.py"]) != "def run(): pass\n" {
		t.Errorf("core.py content = %q", contents.Files["bigpkg/core.py"])
	}
	if len(contents.TopLevelPkgs) != 1 || contents.TopLevelPkgs[0] != "bigpkg" {
		t.Errorf("TopLevelPkgs = %v", contents.TopLevelPkgs)
	}
	if rs.ranged != rs.requests || len(rs.other) != 0 {
		t.Errorf("wheel server saw non-range requests: %v", rs.other)
	}
	if rs.bytes > int64(len(data))/2 {
		t.Errorf("transferred %d of %d bytes, want well under half", rs.bytes, len(data))
	}
	if rs.requests > 6 {
		t.Errorf("requests = %d, want a handful", rs.requests)
	}
}

func TestFetch_RangeUnsupportedFallsBack(t *testing.T) {
	data := buildBigWheel(t, rangeTestFiles(), 9*1024*1024)
	rs := newRangeServer(t, data, false)
	pypiSrv := newPyPIServer(t, "bigpkg", "1.0", rs.URL+"/bigpkg.whl", int64(len(data)))
	defer pypiSrv.Close()

	t.Run("within MaxSize downloads whole wheel", func(t *testing.T) {
		src := &Source{HTTPClient: pypiSrv.Client(), MaxSize: DefaultMaxSize, MaxRangeSize: DefaultMaxRangeSize, BaseURL: pypiSrv.URL}
		contents, err := src.Fetch(context.Background(), "bigpkg", "1.0")
		if err != nil {
			t.Fatalf("Fetch: %v", err)
		}
		if len(contents.Files) != 3 {
			t.Errorf("Files = %d, want 3", len(contents.Files))
		}
	})

	t.Run("beyond MaxSize returns ErrTooLarge", func(t *testing.T) {
		src := &Source{HTTPClient: pypiSrv.Client(), MaxSize: 1024 * 1024, MaxRangeSize: DefaultMaxRangeSize, BaseURL: pypiSrv.URL}
		_, err := src.Fetch(context.Background(), "bigpkg", "1.0")
		if !errors.Is(err, ErrTooLarge) {
			t.Fatalf("want ErrTooLarge, got %v", err)
		}
	})
}

func TestFetch_RangeTooLarge(t *testing.T) {
	wheelSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected %s request to wheel server", r.Method)
	}))
	defer wheelSrv.Close()
	pypiSrv := newPyPIServer(t, "huge", "1.0", wheelSrv.URL+"/huge.whl", 3*1024*1024*1024)
	defer pypiSrv.Close()

	src := &Source{HTTPClient: pypiSrv.Client(), MaxSize: DefaultMaxSize, MaxRangeSize: DefaultMaxRangeSize, BaseURL: pypiSrv.URL}
	_, err := src.Fetch(context.Background(), "huge", "1.0")
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("want ErrTooLarge, got %v", err)
	}
}

// countingTransport counts response body bytes and requests per host.
type countingTransport struct {
	base http.RoundTripper
	mu   sync.Mutex
	n    map[string]int64
	reqs map[string]int
}

func (c *countingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := c.base.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	host := req.URL.Host
	c.mu.Lock()
	c.reqs[host]++
	c.mu.Unlock()
	resp.Body = &countingBody{ReadCloser: resp.Body, add: func(n int) {
		c.mu.Lock()
		c.n[host] += int64(n)
		c.mu.Unlock()
	}}
	return resp, nil
}

type countingBody struct {
	io.ReadCloser
	add func(int)
}

func (b *countingBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	b.add(n)
	return n, err
}

// TestRangeRealWheel reads torch's latest wheel from PyPI. It uses the
// network, so it is skipped under -short.
func TestRangeRealWheel(t *testing.T) {
	if testing.Short() {
		t.Skip("uses the network; skipped under -short")
	}
	ct := &countingTransport{base: http.DefaultTransport, n: map[string]int64{}, reqs: map[string]int{}}
	client := &http.Client{Timeout: 120 * time.Second, Transport: ct}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second) // docs route limit
	defer cancel()

	// Look up the latest torch version.
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, pypiBaseURL+"/pypi/torch/json", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var meta struct {
		Info struct {
			Version string `json:"version"`
		} `json:"info"`
	}
	err = json.NewDecoder(resp.Body).Decode(&meta)
	_ = resp.Body.Close()
	if err != nil || meta.Info.Version == "" {
		t.Fatalf("latest torch version: %v", err)
	}

	src := &Source{HTTPClient: client, MaxSize: DefaultMaxSize, MaxRangeSize: DefaultMaxRangeSize, BaseURL: pypiBaseURL}
	wheels, err := src.fetchWheelURLs(ctx, "torch", meta.Info.Version)
	if err != nil || len(wheels) == 0 {
		t.Fatalf("wheel URLs: %v", err)
	}
	chosen := selectWheel(wheels)
	if chosen.Size <= rangeThreshold {
		t.Fatalf("torch wheel %s is only %d bytes; test needs a ranged read", chosen.Filename, chosen.Size)
	}

	start := time.Now()
	contents, err := src.Fetch(ctx, "torch", meta.Info.Version)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("Fetch: %v (after %v)", err, elapsed)
	}

	var pyBytes int64
	for _, b := range contents.Files {
		pyBytes += int64(len(b))
	}
	fileHost := "files.pythonhosted.org"
	got := ct.n[fileHost]
	t.Logf("torch %s wheel %s", meta.Info.Version, chosen.Filename)
	t.Logf("wheel size: %d bytes; transferred from %s: %d bytes (%.1f%%) in %d requests",
		chosen.Size, fileHost, got, 100*float64(got)/float64(chosen.Size), ct.reqs[fileHost])
	t.Logf("elapsed %v (Fetch, including the PyPI JSON request); modules extracted: %d files, %d inflated .py bytes, top-level %v",
		elapsed, len(contents.Files), pyBytes, contents.TopLevelPkgs)

	if len(contents.Files) == 0 {
		t.Fatal("no .py files extracted")
	}
	if ct.reqs[fileHost] == 0 {
		t.Fatalf("no requests reached %s; hosts seen: %v", fileHost, ct.reqs)
	}
	if got*4 >= chosen.Size {
		t.Errorf("transferred %d bytes, want under 25%% of %d", got, chosen.Size)
	}
}
