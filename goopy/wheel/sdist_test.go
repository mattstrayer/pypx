package wheel

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// tarEntry is one archive entry. A zero Type means a regular file.
type tarEntry struct {
	name string
	body string
	typ  byte
	link string
}

func reg(name, body string) tarEntry { return tarEntry{name: name, body: body} }

func buildTarGz(t *testing.T, entries []tarEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		typ := e.typ
		if typ == 0 {
			typ = tar.TypeReg
		}
		hdr := &tar.Header{Name: e.name, Typeflag: typ, Linkname: e.link, Mode: 0o644, Size: int64(len(e.body))}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(e.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// sdistServer serves the PyPI JSON of one sdist-only release and its archive.
func sdistServer(t *testing.T, filename string, archive []byte, declaredSize int64) *Source {
	t.Helper()
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/archive" {
			w.Write(archive) //nolint:errcheck
			return
		}
		fmt.Fprintf(w, `{"urls":[{"filename":%q,"url":%q,"size":%d,"packagetype":"sdist"}]}`,
			filename, srv.URL+"/archive", declaredSize)
	}))
	t.Cleanup(srv.Close)
	return &Source{HTTPClient: srv.Client(), MaxSize: DefaultMaxSize, BaseURL: srv.URL}
}

func fetchTarGz(t *testing.T, entries ...tarEntry) (*WheelContents, error) {
	t.Helper()
	data := buildTarGz(t, entries)
	return sdistServer(t, "pkg-1.0.tar.gz", data, int64(len(data))).Fetch(context.Background(), "pkg", "1.0")
}

func mustFetchTarGz(t *testing.T, entries ...tarEntry) *WheelContents {
	t.Helper()
	c, err := fetchTarGz(t, entries...)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	return c
}

func fileNames(c *WheelContents) []string {
	var names []string
	for name := range c.Files {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

func wantContents(t *testing.T, c *WheelContents, files, top []string) {
	t.Helper()
	if got := fileNames(c); !reflect.DeepEqual(got, files) {
		t.Errorf("files = %v, want %v", got, files)
	}
	if !reflect.DeepEqual(c.TopLevelPkgs, top) {
		t.Errorf("top = %v, want %v", c.TopLevelPkgs, top)
	}
}

func TestFetch_SdistOnly(t *testing.T) {
	c := mustFetchTarGz(t,
		tarEntry{name: "pkg-1.0/", typ: tar.TypeDir},
		reg("pkg-1.0/PKG-INFO", "Name: pkg"),
		reg("pkg-1.0/setup.py", "print('never run')"),
		reg("pkg-1.0/conftest.py", "x = 1"),
		reg("pkg-1.0/pkg/__init__.py", "def f(): pass"),
		reg("pkg-1.0/pkg/core.py", "def g(): pass"),
		reg("pkg-1.0/pkg/core.pyi", "def g() -> int: ..."),
		reg("pkg-1.0/pkg/__pycache__/core.py", "stale"),
		reg("pkg-1.0/tests/test_core.py", "x = 1"),
		reg("pkg-1.0/test/helper.py", "x = 1"),
		reg("pkg-1.0/docs/conf.py", "x = 1"),
		reg("pkg-1.0/doc/conf.py", "x = 1"),
		reg("pkg-1.0/examples/run.py", "x = 1"),
		reg("pkg-1.0/benchmarks/run.py", "x = 1"),
		reg("pkg-1.0/run_me.py", "x = 1"),
		reg("pkg-1.0/tools/build.py", "x = 1"),
	)
	wantContents(t, c, []string{"pkg/__init__.py", "pkg/core.py", "pkg/core.pyi"}, []string{"pkg"})
	if got := string(c.Files["pkg/core.pyi"]); got != "def g() -> int: ..." {
		t.Errorf("core.pyi = %q", got)
	}
}

func TestFetch_SdistDotSlashNames(t *testing.T) {
	c := mustFetchTarGz(t, reg("./pkg-1.0/pkg/__init__.py", "x = 1"), reg("./pkg-1.0/setup.py", ""))
	wantContents(t, c, []string{"pkg/__init__.py"}, []string{"pkg"})
}

func TestFetch_SdistWheelPreferred(t *testing.T) {
	var archiveHits int
	wheel := buildTestWheel(map[string]string{"w/__init__.py": "x = 1"})
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/sdist":
			archiveHits++
		case "/wheel":
			w.Write(wheel) //nolint:errcheck
		default:
			fmt.Fprintf(w, `{"urls":[
				{"filename":"pkg-1.0.tar.gz","url":%q,"size":10,"packagetype":"sdist"},
				{"filename":"pkg-1.0-py3-none-any.whl","url":%q,"size":%d,"packagetype":"bdist_wheel"}]}`,
				srv.URL+"/sdist", srv.URL+"/wheel", len(wheel))
		}
	}))
	defer srv.Close()
	s := &Source{HTTPClient: srv.Client(), MaxSize: DefaultMaxSize, BaseURL: srv.URL}
	c, err := s.Fetch(context.Background(), "pkg", "1.0")
	if err != nil {
		t.Fatal(err)
	}
	if archiveHits != 0 {
		t.Errorf("sdist downloaded %d times", archiveHits)
	}
	wantContents(t, c, []string{"w/__init__.py"}, []string{"w"})
}

func TestSelectSdist(t *testing.T) {
	got := selectSdist([]WheelFile{
		{Filename: "p-1.tar.bz2"}, {Filename: "p-1.zip"}, {Filename: "p-1b.tar.gz"}, {Filename: "p-1a.tar.gz"},
	})
	if got.Filename != "p-1a.tar.gz" {
		t.Errorf("selectSdist = %s", got.Filename)
	}
}

func TestFetch_SdistSrcLayout(t *testing.T) {
	tests := []struct {
		name    string
		entries []tarEntry
		files   []string
		top     []string
	}{
		{
			name: "package under src",
			entries: []tarEntry{
				reg("p-1/setup.py", ""),
				reg("p-1/src/pkg/__init__.py", ""),
				reg("p-1/src/pkg/a.py", ""),
				reg("p-1/src/tests/test_a.py", ""),
				reg("p-1/tools/x/__init__.py", ""),
				reg("p-1/loose.py", ""),
			},
			files: []string{"pkg/__init__.py", "pkg/a.py"},
			top:   []string{"pkg"},
		},
		{
			name:    "namespace package under src",
			entries: []tarEntry{reg("p-1/src/ns/pkg/__init__.py", "")},
			files:   []string{"ns/pkg/__init__.py"},
			top:     []string{"ns"},
		},
		{
			name:    "single module under src",
			entries: []tarEntry{reg("p-1/src/mod.py", ""), reg("p-1/setup.py", "")},
			files:   []string{"mod.py"},
			top:     []string{"mod"},
		},
		{
			name:    "stub directory under src",
			entries: []tarEntry{reg("p-1/src/foo-stubs/__init__.pyi", ""), reg("p-1/setup.py", "")},
			files:   []string{"foo-stubs/__init__.pyi"},
			top:     []string{"foo"},
		},
		{
			name:    "src that is itself a package stays under the root",
			entries: []tarEntry{reg("p-1/src/__init__.py", ""), reg("p-1/src/a.py", "")},
			files:   []string{"src/__init__.py", "src/a.py"},
			top:     []string{"src"},
		},
		{
			name:    "loose src files do not beat a root package",
			entries: []tarEntry{reg("p-1/pkg/__init__.py", ""), reg("p-1/src/helper.py", "")},
			files:   []string{"pkg/__init__.py"},
			top:     []string{"pkg"},
		},
		{
			name: "alternative root",
			entries: []tarEntry{
				reg("p-1/python2/pkg/__init__.py", ""),
				reg("p-1/python3/pkg/__init__.py", ""),
				reg("p-1/setup.py", ""),
			},
			files: []string{"pkg/__init__.py"},
			top:   []string{"pkg"},
		},
		{
			name:    "alternative root does not beat a root package",
			entries: []tarEntry{reg("p-1/pkg/__init__.py", ""), reg("p-1/lib/vendored/__init__.py", "")},
			files:   []string{"pkg/__init__.py"},
			top:     []string{"pkg"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wantContents(t, mustFetchTarGz(t, tt.entries...), tt.files, tt.top)
		})
	}
}

func TestFetch_SdistTopLevelModules(t *testing.T) {
	c := mustFetchTarGz(t,
		reg("p-1/euclid.py", ""),
		reg("p-1/euclid.pyi", ""),
		reg("p-1/setup.py", ""),
		reg("p-1/ez_setup.py", ""),
		reg("p-1/test_euclid.py", ""),
		reg("p-1/euclid_test.py", ""),
		reg("p-1/scripts/run.py", ""),
	)
	wantContents(t, c, []string{"euclid.py", "euclid.pyi"}, []string{"euclid"})
}

func TestFetch_SdistStubDistribution(t *testing.T) {
	c := mustFetchTarGz(t,
		reg("p-1/foo-stubs/__init__.pyi", ""),
		reg("p-1/foo-stubs/sub.pyi", ""),
		reg("p-1/setup.py", ""),
	)
	wantContents(t, c, []string{"foo-stubs/__init__.pyi", "foo-stubs/sub.pyi"}, []string{"foo"})
}

func TestFetch_SdistUnimportablePathsDropped(t *testing.T) {
	c := mustFetchTarGz(t,
		reg("p-1/pkg/__init__.py", ""),
		reg("p-1/pkg/ok.py", ""),
		reg("p-1/pkg/{{cookiecutter.name}}/gen.py", ""),
		reg("p-1/pkg/not-a-module.py", ""),
		reg("p-1/pkg/1bad.py", ""),
		reg("p-1/{{template}}/__init__.py", ""),
		reg("p-1/.github/scripts/__init__.py", ""),
	)
	wantContents(t, c, []string{"pkg/__init__.py", "pkg/ok.py"}, []string{"pkg"})
}

func TestFetch_SdistDeclaredTopLevel(t *testing.T) {
	c := mustFetchTarGz(t,
		reg("p-1/pkg.egg-info/top_level.txt", "alpha\nsolo\nstale\ntests\n"),
		reg("p-1/alpha/__init__.py", ""),
		reg("p-1/solo.py", ""),
		reg("p-1/tools/__init__.py", ""),
		reg("p-1/tests/__init__.py", ""),
		reg("p-1/alpha/vendor/x.egg-info/top_level.txt", "tools\n"),
	)
	if want := []string{"alpha", "solo"}; !reflect.DeepEqual(c.TopLevelPkgs, want) {
		t.Errorf("top = %v, want %v", c.TopLevelPkgs, want)
	}
}

func TestFetch_SdistDeclaredTopLevelUnderSrc(t *testing.T) {
	c := mustFetchTarGz(t,
		reg("p-1/src/pkg.egg-info/top_level.txt", "alpha\n"),
		reg("p-1/src/alpha/__init__.py", ""),
		reg("p-1/src/extra/__init__.py", ""),
	)
	if want := []string{"alpha"}; !reflect.DeepEqual(c.TopLevelPkgs, want) {
		t.Errorf("top = %v, want %v", c.TopLevelPkgs, want)
	}
}

// configparser2 has this shape. Its setup.py passes py_modules only on Python 2, so
// its top_level.txt is right: on Python 3 "import configparser2" fails, and the
// unlisted module is not part of the API.
func TestFetch_SdistDeclaredTopLevelLeavesUnlistedModuleOut(t *testing.T) {
	c := mustFetchTarGz(t,
		reg("p-1/src/pkg.egg-info/top_level.txt", "backports\n"),
		reg("p-1/src/backports/__init__.py", ""),
		reg("p-1/src/backports/mod/__init__.py", ""),
		reg("p-1/src/solo.py", ""),
	)
	if want := []string{"backports"}; !reflect.DeepEqual(c.TopLevelPkgs, want) {
		t.Errorf("top = %v, want %v", c.TopLevelPkgs, want)
	}
}

func TestFetch_SdistWithoutWrapperDirectory(t *testing.T) {
	c := mustFetchTarGz(t, reg("setup.py", ""), reg("pkg/__init__.py", ""), reg("pkg/a.py", ""))
	wantContents(t, c, []string{"pkg/__init__.py", "pkg/a.py"}, []string{"pkg"})
}

func TestFetch_SdistNoPythonIsNoArtifact(t *testing.T) {
	for name, entries := range map[string][]tarEntry{
		"setup.py only": {reg("p-1/setup.py", ""), reg("p-1/PKG-INFO", "")},
		"C only":        {reg("p-1/ext.c", "")},
		"tests only":    {reg("p-1/tests/test_a.py", "")},
		"empty archive": {},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := fetchTarGz(t, entries...); !errors.Is(err, ErrNoArtifact) {
				t.Errorf("err = %v, want ErrNoArtifact", err)
			}
		})
	}
}

func TestFetch_SdistUnsupportedFormat(t *testing.T) {
	s := sdistServer(t, "pkg-1.0.tar.bz2", []byte("x"), 1)
	if _, err := s.Fetch(context.Background(), "pkg", "1.0"); !errors.Is(err, ErrNoArtifact) {
		t.Errorf("err = %v, want ErrNoArtifact", err)
	}
}

func TestFetch_SdistPathTraversal(t *testing.T) {
	good := reg("p-1/pkg/__init__.py", "")
	for _, name := range []string{
		"../evil.py",
		"p-1/../../evil.py",
		"p-1/pkg/../../../evil.py",
		"/etc/evil.py",
		"/p-1/pkg/evil.py",
		`..\evil.py`,
		`p-1\pkg\evil.py`,
		`C:\evil.py`,
		"C:/evil.py",
		"p-1/../README",
	} {
		t.Run(name, func(t *testing.T) {
			c, err := fetchTarGz(t, good, reg(name, "x = 1"))
			if !errors.Is(err, ErrNoArtifact) {
				t.Fatalf("contents = %+v, err = %v, want ErrNoArtifact", c, err)
			}
			if !strings.Contains(err.Error(), "unsafe path") {
				t.Errorf("err = %v", err)
			}
		})
	}
	t.Run("unsafe link name", func(t *testing.T) {
		_, err := fetchTarGz(t, good, tarEntry{name: "../link.py", typ: tar.TypeSymlink, link: "x"})
		if !errors.Is(err, ErrNoArtifact) {
			t.Errorf("err = %v, want ErrNoArtifact", err)
		}
	})
}

func TestFetch_SdistLinksAreNotRead(t *testing.T) {
	c := mustFetchTarGz(t,
		reg("p-1/pkg/__init__.py", "real"),
		tarEntry{name: "p-1/pkg/sym.py", typ: tar.TypeSymlink, link: "/etc/passwd"},
		tarEntry{name: "p-1/pkg/up.py", typ: tar.TypeSymlink, link: "../../../etc/passwd"},
		tarEntry{name: "p-1/pkg/hard.py", typ: tar.TypeLink, link: "p-1/pkg/__init__.py"},
		tarEntry{name: "p-1/pkg/dev.py", typ: tar.TypeChar},
		tarEntry{name: "p-1/pkg/fifo.py", typ: tar.TypeFifo},
		tarEntry{name: "p-1/evil", typ: tar.TypeSymlink, link: "/"},
		reg("p-1/evil/inner.py", "not under a real directory, but stays in memory only"),
	)
	if got := fileNames(c); slices.Contains(got, "pkg/sym.py") || slices.Contains(got, "pkg/up.py") ||
		slices.Contains(got, "pkg/hard.py") || slices.Contains(got, "pkg/dev.py") || slices.Contains(got, "pkg/fifo.py") {
		t.Errorf("files = %v", got)
	}
	if want := []string{"pkg"}; !reflect.DeepEqual(c.TopLevelPkgs, want) {
		t.Errorf("top = %v, want %v", c.TopLevelPkgs, want)
	}
}

func TestFetch_SdistCaps(t *testing.T) {
	t.Run("declared compressed size", func(t *testing.T) {
		data := buildTarGz(t, []tarEntry{reg("p-1/pkg/__init__.py", "")})
		s := sdistServer(t, "pkg-1.0.tar.gz", data, DefaultMaxSize+1)
		if _, err := s.Fetch(context.Background(), "pkg", "1.0"); !errors.Is(err, ErrTooLarge) {
			t.Errorf("err = %v, want ErrTooLarge", err)
		}
	})

	t.Run("body larger than declared", func(t *testing.T) {
		data := buildTarGz(t, []tarEntry{reg("p-1/pkg/__init__.py", strings.Repeat("x = 1\n", 1000))})
		s := sdistServer(t, "pkg-1.0.tar.gz", data, 1)
		s.MaxSize = int64(len(data)) / 2
		if _, err := s.Fetch(context.Background(), "pkg", "1.0"); !errors.Is(err, ErrTooLarge) {
			t.Errorf("err = %v, want ErrTooLarge", err)
		}
	})

	t.Run("exact compressed size is accepted", func(t *testing.T) {
		data := buildTarGz(t, []tarEntry{reg("p-1/pkg/__init__.py", "")})
		s := sdistServer(t, "pkg-1.0.tar.gz", data, int64(len(data)))
		s.MaxSize = int64(len(data))
		if _, err := s.Fetch(context.Background(), "pkg", "1.0"); err != nil {
			t.Errorf("err = %v", err)
		}
	})

	t.Run("entry count", func(t *testing.T) {
		entries := make([]tarEntry, 0, maxFileCount+1)
		for i := range maxFileCount + 1 {
			entries = append(entries, reg(fmt.Sprintf("p-1/data/%d.txt", i), ""))
		}
		_, err := fetchTarGz(t, entries...)
		if !errors.Is(err, ErrTooLarge) || !strings.Contains(err.Error(), "too many") {
			t.Errorf("err = %v, want ErrTooLarge for too many entries", err)
		}
	})

	t.Run("entry count at the cap is accepted", func(t *testing.T) {
		entries := make([]tarEntry, 0, maxFileCount)
		entries = append(entries, reg("p-1/pkg/__init__.py", ""))
		for i := range maxFileCount - 1 {
			entries = append(entries, reg(fmt.Sprintf("p-1/data/%d.txt", i), ""))
		}
		if _, err := fetchTarGz(t, entries...); err != nil {
			t.Errorf("err = %v", err)
		}
	})

	t.Run("per-file", func(t *testing.T) {
		_, err := fetchTarGz(t, reg("p-1/pkg/huge.py", strings.Repeat("A", MaxDecompressedFile+1)))
		if !errors.Is(err, ErrTooLarge) || !strings.Contains(err.Error(), "per-file") {
			t.Errorf("err = %v, want ErrTooLarge for one file", err)
		}
	})

	t.Run("per-file at the cap is accepted", func(t *testing.T) {
		if _, err := fetchTarGz(t, reg("p-1/pkg/huge.py", strings.Repeat("A", MaxDecompressedFile))); err != nil {
			t.Errorf("err = %v", err)
		}
	})

	t.Run("total", func(t *testing.T) {
		body := strings.Repeat("A", MaxDecompressedFile)
		var entries []tarEntry
		for i := range MaxDecompressedTotal/MaxDecompressedFile + 1 {
			entries = append(entries, reg(fmt.Sprintf("p-1/pkg/m%d.py", i), body))
		}
		_, err := fetchTarGz(t, entries...)
		if !errors.Is(err, ErrTooLarge) || !strings.Contains(err.Error(), "total") {
			t.Errorf("err = %v, want ErrTooLarge for the total", err)
		}
	})

	t.Run("skipped trees do not use up the caps", func(t *testing.T) {
		huge := strings.Repeat("A", MaxDecompressedFile+1)
		c := mustFetchTarGz(t, reg("p-1/tests/huge.py", huge), reg("p-1/pkg/__init__.py", ""))
		wantContents(t, c, []string{"pkg/__init__.py"}, []string{"pkg"})
	})

	t.Run("data files inflate past the stream cap", func(t *testing.T) {
		// A real 1 GiB of zeros in a dropped file: a gzip bomb that holds no source.
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		tw := tar.NewWriter(gz)
		size := int64(maxSdistInflated) + 1
		if err := tw.WriteHeader(&tar.Header{Name: "p-1/bomb.bin", Typeflag: tar.TypeReg, Mode: 0o644, Size: size}); err != nil {
			t.Fatal(err)
		}
		if _, err := io.CopyN(tw, zeroReader{}, size); err != nil {
			t.Fatal(err)
		}
		_ = tw.Close()
		_ = gz.Close()
		data := buf.Bytes()
		if int64(len(data)) > DefaultMaxSize {
			t.Fatalf("test archive is %d bytes compressed", len(data))
		}
		s := sdistServer(t, "pkg-1.0.tar.gz", data, int64(len(data)))
		_, err := s.Fetch(context.Background(), "pkg", "1.0")
		if !errors.Is(err, ErrTooLarge) || !strings.Contains(err.Error(), "inflates past") {
			t.Errorf("err = %v, want ErrTooLarge for the stream", err)
		}
	})
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}

func TestFetch_SdistReadFailuresFailTheFetch(t *testing.T) {
	entries := []tarEntry{reg("p-1/pkg/__init__.py", strings.Repeat("def f(): pass\n", 100))}
	for i := range 200 {
		// Each body differs, so the gzip stream stays long enough to cut.
		entries = append(entries, reg(fmt.Sprintf("p-1/pkg/m%d.py", i), strings.Repeat(fmt.Sprintf("x%d = %d\n", i, i*7919), 50)))
	}
	data := buildTarGz(t, entries)

	assertFails := func(t *testing.T, err error) {
		t.Helper()
		if err == nil {
			t.Fatal("got contents, want an error")
		}
		if errors.Is(err, ErrNoArtifact) || errors.Is(err, ErrTooLarge) {
			t.Errorf("err = %v, want a failure the API does not cache as a reason", err)
		}
	}

	t.Run("truncated body", func(t *testing.T) {
		s := sdistServer(t, "pkg-1.0.tar.gz", data[:len(data)/2], int64(len(data)))
		_, err := s.Fetch(context.Background(), "pkg", "1.0")
		assertFails(t, err)
	})
	t.Run("connection dropped mid-body", func(t *testing.T) {
		var srv *httptest.Server
		srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/archive" {
				w.Header().Set("Content-Length", fmt.Sprint(len(data)))
				w.Write(data[:len(data)/2]) //nolint:errcheck
				return                      // The server closes with bytes owed.
			}
			fmt.Fprintf(w, `{"urls":[{"filename":"pkg-1.0.tar.gz","url":%q,"size":%d,"packagetype":"sdist"}]}`, srv.URL+"/archive", len(data))
		}))
		defer srv.Close()
		s := &Source{HTTPClient: srv.Client(), MaxSize: DefaultMaxSize, BaseURL: srv.URL}
		_, err := s.Fetch(context.Background(), "pkg", "1.0")
		assertFails(t, err)
	})
	t.Run("cut at a tar block inside the stream", func(t *testing.T) {
		// The gzip stream ends cleanly, but the tar has no end marker and a file
		// is short. tar reports this as an unexpected EOF.
		var raw bytes.Buffer
		tw := tar.NewWriter(&raw)
		_ = tw.WriteHeader(&tar.Header{Name: "p-1/pkg/a.py", Typeflag: tar.TypeReg, Mode: 0o644, Size: 5000})
		_, _ = tw.Write(make([]byte, 5000))
		var gzBuf bytes.Buffer
		gz := gzip.NewWriter(&gzBuf)
		_, _ = gz.Write(raw.Bytes()[:1024])
		_ = gz.Close()
		s := sdistServer(t, "pkg-1.0.tar.gz", gzBuf.Bytes(), int64(gzBuf.Len()))
		_, err := s.Fetch(context.Background(), "pkg", "1.0")
		assertFails(t, err)
	})
	t.Run("corrupt gzip body", func(t *testing.T) {
		bad := slices.Clone(data)
		for i := len(bad) / 2; i < len(bad)/2+64; i++ {
			bad[i] ^= 0xff
		}
		s := sdistServer(t, "pkg-1.0.tar.gz", bad, int64(len(bad)))
		_, err := s.Fetch(context.Background(), "pkg", "1.0")
		assertFails(t, err)
	})
	t.Run("not gzip", func(t *testing.T) {
		s := sdistServer(t, "pkg-1.0.tar.gz", []byte("<html>not an archive</html>"), 27)
		_, err := s.Fetch(context.Background(), "pkg", "1.0")
		assertFails(t, err)
	})
	t.Run("download status", func(t *testing.T) {
		var srv *httptest.Server
		srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/archive" {
				http.Error(w, "nope", http.StatusServiceUnavailable)
				return
			}
			fmt.Fprintf(w, `{"urls":[{"filename":"pkg-1.0.tar.gz","url":%q,"size":10,"packagetype":"sdist"}]}`, srv.URL+"/archive")
		}))
		defer srv.Close()
		s := &Source{HTTPClient: srv.Client(), MaxSize: DefaultMaxSize, BaseURL: srv.URL}
		_, err := s.Fetch(context.Background(), "pkg", "1.0")
		assertFails(t, err)
		if !strings.Contains(err.Error(), "503") {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("cancelled context", func(t *testing.T) {
		s := sdistServer(t, "pkg-1.0.tar.gz", data, int64(len(data)))
		wheels, sdists, _, err := s.fetchRelease(context.Background(), "pkg", "1.0")
		if err != nil || len(wheels) != 0 {
			t.Fatal(wheels, err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = s.fetchSdist(ctx, sdists[0], "pkg")
		assertFails(t, err)
		if !errors.Is(err, context.Canceled) {
			t.Errorf("err = %v, want context.Canceled", err)
		}
	})
}

func buildZip(t *testing.T, add func(w *zip.Writer)) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	add(w)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func zipFile(t *testing.T, w *zip.Writer, name, body string) {
	t.Helper()
	f, err := w.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write([]byte(body)); err != nil {
		t.Fatal(err)
	}
}

func fetchZip(t *testing.T, data []byte) (*WheelContents, error) {
	t.Helper()
	return sdistServer(t, "pkg-1.0.zip", data, int64(len(data))).Fetch(context.Background(), "pkg", "1.0")
}

func TestFetch_SdistZip(t *testing.T) {
	c, err := fetchZip(t, buildZip(t, func(w *zip.Writer) {
		zipFile(t, w, "pkg-1.0/setup.py", "")
		zipFile(t, w, "pkg-1.0/src/pkg/__init__.py", "x = 1")
		zipFile(t, w, "pkg-1.0/src/pkg/a.pyi", "y: int")
		zipFile(t, w, "pkg-1.0/tests/test_a.py", "")
	}))
	if err != nil {
		t.Fatal(err)
	}
	wantContents(t, c, []string{"pkg/__init__.py", "pkg/a.pyi"}, []string{"pkg"})
}

func TestFetch_SdistZipRules(t *testing.T) {
	t.Run("traversal", func(t *testing.T) {
		_, err := fetchZip(t, buildZip(t, func(w *zip.Writer) {
			zipFile(t, w, "pkg-1.0/pkg/__init__.py", "")
			zipFile(t, w, "../evil.py", "")
		}))
		if !errors.Is(err, ErrNoArtifact) || !strings.Contains(err.Error(), "unsafe path") {
			t.Errorf("err = %v, want ErrNoArtifact", err)
		}
	})
	t.Run("absolute", func(t *testing.T) {
		_, err := fetchZip(t, buildZip(t, func(w *zip.Writer) { zipFile(t, w, "/etc/evil.py", "") }))
		if !errors.Is(err, ErrNoArtifact) {
			t.Errorf("err = %v, want ErrNoArtifact", err)
		}
	})
	t.Run("symlink", func(t *testing.T) {
		c, err := fetchZip(t, buildZip(t, func(w *zip.Writer) {
			zipFile(t, w, "pkg-1.0/pkg/__init__.py", "")
			h := &zip.FileHeader{Name: "pkg-1.0/pkg/link.py"}
			h.SetMode(os.ModeSymlink | 0o777)
			f, err := w.CreateHeader(h)
			if err != nil {
				t.Fatal(err)
			}
			_, _ = f.Write([]byte("/etc/passwd"))
		}))
		if err != nil {
			t.Fatal(err)
		}
		wantContents(t, c, []string{"pkg/__init__.py"}, []string{"pkg"})
	})
	t.Run("per-file cap", func(t *testing.T) {
		_, err := fetchZip(t, buildZip(t, func(w *zip.Writer) {
			zipFile(t, w, "pkg-1.0/pkg/huge.py", strings.Repeat("A", MaxDecompressedFile+1))
		}))
		if !errors.Is(err, ErrTooLarge) {
			t.Errorf("err = %v, want ErrTooLarge", err)
		}
	})
	t.Run("total cap", func(t *testing.T) {
		body := strings.Repeat("A", MaxDecompressedFile)
		_, err := fetchZip(t, buildZip(t, func(w *zip.Writer) {
			for i := range MaxDecompressedTotal/MaxDecompressedFile + 1 {
				zipFile(t, w, fmt.Sprintf("pkg-1.0/pkg/m%d.py", i), body)
			}
		}))
		if !errors.Is(err, ErrTooLarge) {
			t.Errorf("err = %v, want ErrTooLarge", err)
		}
	})
	t.Run("entry count", func(t *testing.T) {
		_, err := fetchZip(t, buildZip(t, func(w *zip.Writer) {
			for i := range maxFileCount + 1 {
				zipFile(t, w, fmt.Sprintf("pkg-1.0/data/%d.txt", i), "")
			}
		}))
		if !errors.Is(err, ErrTooLarge) {
			t.Errorf("err = %v, want ErrTooLarge", err)
		}
	})
	t.Run("declared size over the cap", func(t *testing.T) {
		s := sdistServer(t, "pkg-1.0.zip", []byte("x"), DefaultMaxSize+1)
		if _, err := s.Fetch(context.Background(), "pkg", "1.0"); !errors.Is(err, ErrTooLarge) {
			t.Errorf("err = %v, want ErrTooLarge", err)
		}
	})
	t.Run("corrupt entry is skipped", func(t *testing.T) {
		var buf bytes.Buffer
		w := zip.NewWriter(&buf)
		zipFile(t, w, "pkg-1.0/pkg/__init__.py", "x = 1")
		f, _ := w.CreateHeader(&zip.FileHeader{Name: "pkg-1.0/pkg/bad.py", Method: zip.Deflate})
		_, _ = f.Write(bytes.Repeat([]byte("corrupt me\n"), 200))
		_ = w.Close()
		data := buf.Bytes()
		// Damage the deflate data of the second entry, then read through the checksum.
		r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			t.Fatal(err)
		}
		off, err := r.File[1].DataOffset()
		if err != nil {
			t.Fatal(err)
		}
		for i := off; i < off+8; i++ {
			data[i] ^= 0xff
		}
		c, err := fetchZip(t, data)
		if err != nil {
			t.Fatal(err)
		}
		wantContents(t, c, []string{"pkg/__init__.py"}, []string{"pkg"})
	})
	t.Run("not a zip", func(t *testing.T) {
		_, err := fetchZip(t, []byte("not a zip"))
		if err == nil || errors.Is(err, ErrNoArtifact) {
			t.Errorf("err = %v, want a plain failure", err)
		}
	})
}

// TestSdistReal reads real sdists from PyPI. pyspark is over MaxSize, so it must
// fail by its declared size, without a download.
func TestSdistReal(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test in -short mode")
	}
	tests := []struct {
		name, version string
		want          string // A file the sdist must give.
	}{
		{"euclid3", "0.01", "euclid3.py"},
		{"systemd-python", "235", "systemd/journal.py"},
		{"pika-stubs", "0.1.3", "pika-stubs/channel.pyi"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := NewSource().Fetch(context.Background(), tt.name, tt.version)
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := c.Files[tt.want]; !ok {
				t.Errorf("files = %v, want %s", fileNames(c), tt.want)
			}
			t.Logf("%s==%s: %d files, top %v", tt.name, tt.version, len(c.Files), c.TopLevelPkgs)
		})
	}
	t.Run("pyspark", func(t *testing.T) {
		_, err := NewSource().Fetch(context.Background(), "pyspark", "4.3.0")
		if !errors.Is(err, ErrTooLarge) {
			t.Fatalf("err = %v, want ErrTooLarge", err)
		}
		t.Log(err)
	})
}

func TestFetch_SdistEmptyVersionUsesLatest(t *testing.T) {
	archive := buildTarGz(t, []tarEntry{reg("pkg-2.5.0/pkg/__init__.py", "x = 1")})
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/archive" {
			w.Write(archive) //nolint:errcheck
			return
		}
		fmt.Fprintf(w, `{"info":{"version":"2.5.0"},"urls":[{"filename":"pkg-2.5.0.tar.gz","url":%q,"size":%d,"packagetype":"sdist"}]}`,
			srv.URL+"/archive", len(archive))
	}))
	defer srv.Close()
	s := &Source{HTTPClient: srv.Client(), MaxSize: DefaultMaxSize, BaseURL: srv.URL}
	c, err := s.Fetch(context.Background(), "pkg", "")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if c.Version != "2.5.0" {
		t.Errorf("Version = %q, want 2.5.0", c.Version)
	}
}
