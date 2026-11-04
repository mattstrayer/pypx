package wheel

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

// compressedSources returns the inflated and compressed sizes of a wheel's sources.
func compressedSources(t *testing.T, data []byte) (source, compressed int64) {
	t.Helper()
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range r.File {
		if strings.HasSuffix(f.Name, ".py") && !strings.Contains(f.Name, "__pycache__") {
			source += int64(f.UncompressedSize64)
			compressed += int64(f.CompressedSize64)
		}
	}
	return source, compressed
}

func weighSource(t *testing.T, data []byte, honourRange bool) (*Source, *rangeServer) {
	t.Helper()
	rs := newRangeServer(t, data, honourRange)
	pypiSrv := newPyPIServer(t, "bigpkg", "1.0", rs.URL+"/bigpkg.whl", int64(len(data)))
	t.Cleanup(pypiSrv.Close)
	return &Source{HTTPClient: pypiSrv.Client(), MaxSize: DefaultMaxSize, MaxRangeSize: DefaultMaxRangeSize, BaseURL: pypiSrv.URL}, rs
}

func TestWeigh_LargeWheelCountsSourcesNotTheWheel(t *testing.T) {
	data := buildBigWheel(t, rangeTestFiles(), 9*1024*1024)
	src, rs := weighSource(t, data, true)

	got, err := src.Weigh(context.Background(), "bigpkg", "1.0")
	if err != nil {
		t.Fatal(err)
	}
	source, compressed := compressedSources(t, data)
	if want := source + compressed; got != want {
		t.Errorf("Weigh = %d, want %d", got, want)
	}
	if _, _, served, _ := rs.stats(); served > int64(len(data))/4 {
		t.Errorf("transferred %d of %d bytes, want only the central directory", served, len(data))
	}
}

func TestWeigh_SmallWheelCountsTheWholeDownload(t *testing.T) {
	data := buildBigWheel(t, rangeTestFiles(), 1024*1024)
	src, _ := weighSource(t, data, true)

	got, err := src.Weigh(context.Background(), "bigpkg", "1.0")
	if err != nil {
		t.Fatal(err)
	}
	source, _ := compressedSources(t, data)
	if want := source + int64(len(data)); got != want {
		t.Errorf("Weigh = %d, want %d", got, want)
	}
}

func TestWeigh_RangeRefusedEstimatesFromWheelSize(t *testing.T) {
	data := buildBigWheel(t, rangeTestFiles(), 1024*1024)
	src, _ := weighSource(t, data, false)

	got, err := src.Weigh(context.Background(), "bigpkg", "1.0")
	if err != nil {
		t.Fatal(err)
	}
	if want := (unreadInflateRatio + 1) * int64(len(data)); got != want {
		t.Errorf("Weigh = %d, want %d", got, want)
	}
}

func TestWeigh_CorruptWheelEstimatesFromWheelSize(t *testing.T) {
	data := bytes.Repeat([]byte("not a zip "), 1000)
	src, _ := weighSource(t, data, true)

	got, err := src.Weigh(context.Background(), "bigpkg", "1.0")
	if err != nil {
		t.Fatal(err)
	}
	if want := (unreadInflateRatio + 1) * int64(len(data)); got != want {
		t.Errorf("Weigh = %d, want %d", got, want)
	}
}

func TestWeigh_WheelFetchWouldRejectWeighsNothing(t *testing.T) {
	data := buildBigWheel(t, rangeTestFiles(), 1024*1024)
	src, _ := weighSource(t, data, true)
	src.MaxSize = 1024

	got, err := src.Weigh(context.Background(), "bigpkg", "1.0")
	if err != nil || got != 0 {
		t.Errorf("Weigh = (%d, %v), want (0, nil)", got, err)
	}
}

func TestWeigh_MissingPackageReturnsError(t *testing.T) {
	data := buildBigWheel(t, rangeTestFiles(), 1024)
	src, _ := weighSource(t, data, true)

	_, err := src.Weigh(context.Background(), "other", "1.0")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestWeigh_Sdist(t *testing.T) {
	const size = 1024 * 1024
	for _, tc := range []struct {
		name     string
		filename string
		size     int64
		want     int64
	}{
		{"tar.gz", "pkg-1.0.tar.gz", size, (unreadInflateRatio + 1) * size},
		{"zip", "pkg-1.0.zip", size, (unreadInflateRatio + 1) * size},
		{"too large for Fetch", "pkg-1.0.tar.gz", DefaultMaxSize + 1, 0},
		{"unreadable format", "pkg-1.0.tar.bz2", size, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := sdistServer(t, tc.filename, nil, tc.size)
			got, err := s.Weigh(context.Background(), "pkg", "1.0")
			if err != nil || got != tc.want {
				t.Errorf("Weigh = (%d, %v), want (%d, nil)", got, err, tc.want)
			}
		})
	}
}
