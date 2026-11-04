package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pypx/goopy/model"
	"github.com/pypx/goopy/wheel"
)

func failingExtract(err error) func(context.Context, string, string) (*model.Package, error) {
	return func(context.Context, string, string) (*model.Package, error) { return nil, err }
}

func TestDocsReason_ExtractionErrorMapping(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		reason     string
		wantErr    error
		wantErrKey bool
	}{
		{name: "no artifact", err: fmt.Errorf("sdist: %w", wheel.ErrNoArtifact), reason: "no_artifact"},
		{name: "too large", err: fmt.Errorf("big: %w", wheel.ErrTooLarge), reason: "too_large"},
		{name: "not found", err: fmt.Errorf("gone: %w", wheel.ErrNotFound), wantErr: ErrDocsPackageNotFound},
		{name: "other", err: errors.New("connection reset"), wantErr: ErrDocsExtractionFailed, wantErrKey: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newTTLCache()
			h := docsCacheHandler(c, failingExtract(tt.err))
			resp, _, err := h.docsAtVersion(context.Background(), "p", "1", true)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
				if c.wrote(DocsCacheKey("p", "1")) {
					t.Error("docs key written for a failure")
				}
				if got := c.wrote(DocsErrKey("p", "1")); got != tt.wantErrKey {
					t.Errorf("error key written = %v, want %v", got, tt.wantErrKey)
				}
				return
			}
			if err != nil {
				t.Fatalf("err = %v", err)
			}
			if resp.Available || resp.Reason != tt.reason || resp.Modules == nil {
				t.Errorf("resp = %+v, want Available false, Reason %q, non-nil Modules", resp, tt.reason)
			}
			if !c.wrote(DocsCacheKey("p", "1")) {
				t.Error("reason row not cached")
			}
			if c.wrote(DocsErrKey("p", "1")) {
				t.Error("error key written for a reason row")
			}
		})
	}
}

func TestDocsReason_NotFoundStaysNotFoundOnRetry(t *testing.T) {
	h := docsCacheHandler(newTTLCache(), failingExtract(wheel.ErrNotFound))
	for range 2 {
		if _, _, err := h.docsAtVersion(context.Background(), "p", "1", true); !errors.Is(err, ErrDocsPackageNotFound) {
			t.Fatalf("err = %v, want ErrDocsPackageNotFound", err)
		}
	}
}

func TestDocsReason_RowIsCachedThenRetried(t *testing.T) {
	c := newTTLCache()
	var calls atomic.Int32
	h := docsCacheHandler(c, func(context.Context, string, string) (*model.Package, error) {
		calls.Add(1)
		return nil, wheel.ErrTooLarge
	})
	for _, decode := range []bool{true, false} {
		_, data, err := h.docsAtVersion(context.Background(), "p", "1", decode)
		if err != nil || !strings.Contains(string(data), `"reason":"too_large"`) {
			t.Fatalf("decode=%v: data = %s, err = %v", decode, data, err)
		}
	}
	if calls.Load() != 1 {
		t.Errorf("extract calls = %d, want 1 while the row is fresh", calls.Load())
	}

	key := DocsCacheKey("p", "1")
	data, _, _ := c.Get(key, time.Hour)
	c.put(key, data, docsEmptyTTL+time.Minute)
	if _, _, err := h.docsAtVersion(context.Background(), "p", "1", true); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 {
		t.Errorf("extract calls = %d, want 2 after docsEmptyTTL", calls.Load())
	}
}

// A diff must not read a release without docs as a release with no API.
func TestDocsReason_FetchDocsAtVersionFails(t *testing.T) {
	h := docsCacheHandler(newTTLCache(), failingExtract(wheel.ErrNoArtifact))
	for range 2 { // a miss, then a cache hit
		if _, _, err := h.fetchDocsAtVersion(context.Background(), "p", "1"); !errors.Is(err, ErrDocsExtractionFailed) {
			t.Fatalf("err = %v, want ErrDocsExtractionFailed", err)
		}
	}
}

func docsRouter(h *DocsHandler) http.Handler {
	r := chi.NewRouter()
	r.Get("/{name}/docs", h.Get)
	r.Get("/{name}/docs/status", h.GetStatus)
	r.Get("/{name}/docs.txt", h.GetText)
	r.Get("/{name}/symbols.txt", h.GetSymbols)
	return r
}

func TestDocsReason_Routes(t *testing.T) {
	for _, tt := range []struct {
		err    error
		reason string
		text   string
	}{
		{fmt.Errorf("sdist too big: %w", wheel.ErrTooLarge), "too_large", "too large"},
		{fmt.Errorf("no files: %w", wheel.ErrNoArtifact), "no_artifact", "no wheel"},
	} {
		t.Run(tt.reason, func(t *testing.T) {
			client, _ := fakePyPI(t, "1.0", http.StatusOK)
			h := docsCacheHandler(newTTLCache(), failingExtract(tt.err))
			h.pypi = client
			r := docsRouter(h)

			get := func(path string) *httptest.ResponseRecorder {
				t.Helper()
				rec := httptest.NewRecorder()
				r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
				if rec.Code != http.StatusOK {
					t.Fatalf("%s: status %d", path, rec.Code)
				}
				return rec
			}

			var docs DocsResponse
			if err := json.Unmarshal(get("/p/docs").Body.Bytes(), &docs); err != nil {
				t.Fatal(err)
			}
			if docs.Available || docs.Reason != tt.reason || docs.Version != "1.0" {
				t.Errorf("docs = %+v", docs)
			}
			if body := get("/p/docs/status").Body.String(); !strings.Contains(body, `"available":false`) {
				t.Errorf("status body = %s", body)
			}
			for _, path := range []string{"/p/docs.txt", "/p/symbols.txt"} {
				rec := get(path)
				if !strings.Contains(rec.Body.String(), tt.text) {
					t.Errorf("%s body = %q", path, rec.Body.String())
				}
				if strings.Contains(rec.Header().Get("Cache-Control"), "immutable") {
					t.Errorf("%s Cache-Control = %q", path, rec.Header().Get("Cache-Control"))
				}
			}
		})
	}
}

func TestDocsReason_StatusCodes(t *testing.T) {
	for _, tt := range []struct {
		name string
		err  error
		want int
	}{
		{"not found", wheel.ErrNotFound, http.StatusNotFound},
		{"other", errors.New("connection reset"), http.StatusBadGateway},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client, _ := fakePyPI(t, "1.0", http.StatusOK)
			h := docsCacheHandler(newTTLCache(), failingExtract(tt.err))
			h.pypi = client
			rec := httptest.NewRecorder()
			docsRouter(h).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/p/docs", nil))
			if rec.Code != tt.want {
				t.Errorf("status = %d, want %d", rec.Code, tt.want)
			}
		})
	}
}
