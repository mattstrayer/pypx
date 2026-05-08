package handler_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pypx/api/internal/handler"
)

func TestLLMSHandler(t *testing.T) {
	h := handler.NewLLMSHandler()
	req := httptest.NewRequest("GET", "/llms.txt", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("Content-Type = %q, want text/plain prefix", ct)
	}
	body := w.Body.String()
	for _, expect := range []string{
		"# pypx",
		"/api/packages/{name}.txt",
		"/api/packages/{name}/changelog.txt",
		"/api/packages/{name}/security.txt",
		"/api/packages/{name}/extras.txt",
		"/api/packages/{name}/summary.txt",
		"/api/search.txt",
		"/api/packages/{name}/docs.txt",
		"/api/packages/{name}/docs/{symbol}.txt",
		"/api/packages/{name}/symbols.txt",
	} {
		if !strings.Contains(body, expect) {
			t.Errorf("body missing %q", expect)
		}
	}
}
