package handler

import (
	"net/http"
	"net/http/httptest"
	"regexp"
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
		"openapi.json",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("llms.txt missing %q", want)
		}
	}
	// The byte budget is a proxy for "cheap for an agent to read", not a hard
	// spec — it exists to catch unbounded growth, not to force trimming
	// accurate content. Raised from 2800 to 4400 when the endpoint lists became
	// real Markdown links with absolute example URLs (llmstxt.org format, and
	// required by Lighthouse's llms-txt audit — see TestLLMSFollowsLighthouse).
	// The added bytes are working example URLs, which are content, not padding.
	if len(body) > 4400 {
		t.Errorf("llms.txt is %d bytes, budget 4400", len(body))
	}
}

// TestLLMSFollowsLighthouse encodes the three checks Lighthouse's `llms-txt`
// audit (Agentic Browsing category) runs against the served file. The regexes
// are copied verbatim from core/audits/agentic/llms-txt.js so a wording change
// here fails locally instead of silently dropping the PageSpeed score.
func TestLLMSFollowsLighthouse(t *testing.T) {
	h := NewLLMSHandler(stubCounter{n: 543210})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/llms.txt", nil))
	body := rec.Body.String()

	if !regexp.MustCompile(`(?m)^\s*#\s+.+`).MatchString(body) {
		t.Error("llms.txt is missing a required H1 header")
	}
	if !regexp.MustCompile(`\[.+\]\(.+\)`).MatchString(body) {
		t.Error("llms.txt does not contain any Markdown links")
	}
	if len(body) < 50 {
		t.Errorf("llms.txt is suspiciously short (%d bytes)", len(body))
	}
}

// TestLLMSLinksAreAbsolute guards the reason the links exist: agents fetch
// llms.txt on its own, so a relative target is not reliably resolvable.
func TestLLMSLinksAreAbsolute(t *testing.T) {
	h := NewLLMSHandler(stubCounter{n: 543210})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/llms.txt", nil))

	targets := regexp.MustCompile(`\[[^\]]+\]\(([^)]+)\)`).FindAllStringSubmatch(rec.Body.String(), -1)
	if len(targets) < 10 {
		t.Fatalf("expected the endpoint lists to be links, found only %d", len(targets))
	}
	for _, m := range targets {
		if !strings.HasPrefix(m[1], "https://") {
			t.Errorf("llms.txt link target %q is not absolute", m[1])
		}
	}
}
