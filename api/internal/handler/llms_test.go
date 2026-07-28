package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type stubCounter struct {
	n   int
	err error
}

func (s stubCounter) Count() (int, error) { return s.n, s.err }

func TestLLMSDynamicCount(t *testing.T) {
	h := NewLLMSHandler(stubCounter{n: 543210})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/llms.txt", nil))
	body := rec.Body.String()
	if !strings.Contains(body, "543,210 packages indexed") {
		t.Errorf("missing dynamic count, body header:\n%s", body[:200])
	}
}

func TestLLMSCountFallback(t *testing.T) {
	h := NewLLMSHandler(stubCounter{err: http.ErrServerClosed})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/llms.txt", nil))
	if !strings.Contains(rec.Body.String(), "500,000+ packages indexed") {
		t.Error("missing static fallback count")
	}
}

func TestLLMSContent(t *testing.T) {
	h := NewLLMSHandler(stubCounter{n: 543210})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/llms.txt", nil))
	body := rec.Body.String()
	for _, want := range []string{
		"## JSON endpoints",
		"/api/packages/{name}/stats",
		"/api/popular",
		"## Rate limits",
		"?limit=",
		"GET /api",
		"stats.txt",
		"popular.txt",
		"Accept: text/plain",
		"Retry-After",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("llms.txt missing %q", want)
		}
	}
	if len(body) > 2600 {
		t.Errorf("llms.txt is %d bytes, budget 2600", len(body))
	}
}
