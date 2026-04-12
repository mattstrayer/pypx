package pypi_test

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
		want     bool
	}{
		{"py.typed present", withTyped, 0, true},
		{"py.typed absent", withoutTyped, 0, false},
		{"wheel over 50MB skipped", withTyped, 55 * 1024 * 1024, false},
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
				http.ServeContent(w, r, "wheel.whl", time.Time{}, bytes.NewReader(tt.wheel))
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
