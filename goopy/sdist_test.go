package goopy

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/pypx/goopy/model"
	"github.com/pypx/goopy/wheel"
)

// sdistSource serves one sdist-only release whose archive holds files.
func sdistSource(t *testing.T, files map[string]string) *wheel.Source {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, body := range files {
		hdr := &tar.Header{Name: name, Typeflag: tar.TypeReg, Mode: 0o644, Size: int64(len(body))}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}

	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/archive" {
			w.Write(buf.Bytes()) //nolint:errcheck
			return
		}
		fmt.Fprintf(w, `{"urls":[{"filename":"p-1.tar.gz","url":%q,"size":%d,"packagetype":"sdist"}]}`, srv.URL+"/archive", buf.Len())
	}))
	t.Cleanup(srv.Close)
	return &wheel.Source{HTTPClient: srv.Client(), MaxSize: wheel.DefaultMaxSize, BaseURL: srv.URL}
}

func moduleNames(pkg *model.Package) []string {
	var names []string
	for _, m := range pkg.Modules {
		names = append(names, m.Name)
	}
	return names
}

func TestExtractFromWheel_SdistMergesStubs(t *testing.T) {
	pkg, err := ExtractFromWheel(context.Background(), sdistSource(t, map[string]string{
		"p-1/setup.py":           "def setup_helper(): pass\n",
		"p-1/src/p/__init__.py":  "",
		"p-1/src/p/core.py":      "def add(a, b):\n    \"\"\"Add two numbers.\"\"\"\n    return a + b\n",
		"p-1/src/p/core.pyi":     "def add(a: int, b: int) -> int: ...\n",
		"p-1/src/p/native.pyi":   "def fast(x: str) -> bytes: ...\n",
		"p-1/tests/test_core.py": "def test_add(): pass\n",
	}), "p", "1")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := moduleNames(pkg), []string{"p.core", "p.native"}; !slices.Equal(got, want) {
		t.Fatalf("modules = %v, want %v", got, want)
	}
	add := pkg.Modules[0].Functions[0]
	if add.Name != "add" || add.Docstring == nil || add.Returns == nil || add.Returns.Raw != "int" || add.Parameters[0].Type == nil {
		t.Errorf("add = %+v, want the docstring of the .py and the types of the .pyi", add)
	}
	if fast := pkg.Modules[1].Functions[0]; fast.Name != "fast" || fast.Returns == nil || fast.Returns.Raw != "bytes" {
		t.Errorf("fast = %+v", fast)
	}
}

func TestExtractFromWheel_SdistStubDistribution(t *testing.T) {
	pkg, err := ExtractFromWheel(context.Background(), sdistSource(t, map[string]string{
		"foo-stubs-1/setup.py":               "",
		"foo-stubs-1/foo-stubs/__init__.pyi": "def top() -> int: ...\n",
		"foo-stubs-1/foo-stubs/sub.pyi":      "class C:\n    x: int\n",
	}), "foo-stubs", "1")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := moduleNames(pkg), []string{"foo", "foo.sub"}; !slices.Equal(got, want) {
		t.Errorf("modules = %v, want %v", got, want)
	}
}
