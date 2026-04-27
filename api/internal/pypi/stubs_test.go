package pypi_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pypx/api/internal/pypi"
)

func TestCheckTypeSupport(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/pypi/types-requests/json":
			w.WriteHeader(http.StatusOK)
		case "/pypi/requests-stubs/json":
			w.WriteHeader(http.StatusNotFound)
		case "/pypi/types-numpy/json":
			w.WriteHeader(http.StatusNotFound)
		case "/pypi/numpy-stubs/json":
			w.WriteHeader(http.StatusOK)
		case "/pypi/types-notstubs/json", "/pypi/notstubs-stubs/json":
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	tests := []struct {
		name        string
		pkg         string
		wantStatus  string
		wantStubPkg string
	}{
		{"types- prefix found", "requests", "stubs", "types-requests"},
		{"-stubs suffix found", "numpy", "stubs", "numpy-stubs"},
		{"no stubs found", "notstubs", "untyped", ""},
	}

	c := pypi.NewClient(pypi.WithBaseURL(srv.URL))
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pypi.CheckTypeSupport(context.Background(), c, tt.pkg)
			if got.Status != tt.wantStatus {
				t.Errorf("Status = %q, want %q", got.Status, tt.wantStatus)
			}
			if got.StubsPackage != tt.wantStubPkg {
				t.Errorf("StubsPackage = %q, want %q", got.StubsPackage, tt.wantStubPkg)
			}
		})
	}
}
