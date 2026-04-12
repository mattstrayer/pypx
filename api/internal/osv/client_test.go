package osv_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pypx/api/internal/osv"
)

func TestFetchVulns(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/query" {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"vulns": [{
				"id": "GHSA-9wx4-h78v-vm56",
				"summary": "Requests session cookies can leak",
				"severity": [{"type": "CVSS_V3", "score": "MEDIUM"}],
				"affected": [{
					"ranges": [{"type": "ECOSYSTEM", "events": [{"introduced": "0"}, {"fixed": "2.32.0"}]}]
				}],
				"references": [{"url": "https://github.com/advisories/GHSA-9wx4-h78v-vm56"}]
			}]
		}`)
	}))
	defer srv.Close()

	c := osv.NewClient(osv.WithBaseURL(srv.URL))
	vulns, err := c.FetchVulns("requests")
	if err != nil {
		t.Fatalf("FetchVulns() error: %v", err)
	}
	if len(vulns) != 1 {
		t.Fatalf("expected 1 vuln, got %d", len(vulns))
	}
	if vulns[0].ID != "GHSA-9wx4-h78v-vm56" {
		t.Errorf("vuln ID = %q, want GHSA-9wx4-h78v-vm56", vulns[0].ID)
	}
	if vulns[0].Severity != "MEDIUM" {
		t.Errorf("severity = %q, want MEDIUM", vulns[0].Severity)
	}
	if vulns[0].URL == "" {
		t.Error("vuln URL should not be empty")
	}
}

func TestFetchVulnsNoVulns(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{}`)
	}))
	defer srv.Close()

	c := osv.NewClient(osv.WithBaseURL(srv.URL))
	vulns, err := c.FetchVulns("safe-package")
	if err != nil {
		t.Fatalf("FetchVulns() error: %v", err)
	}
	if len(vulns) != 0 {
		t.Errorf("expected 0 vulns, got %d", len(vulns))
	}
}
