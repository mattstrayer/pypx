package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/pypx/api/internal/cache"
	"github.com/pypx/api/internal/handler"
	"github.com/pypx/api/internal/stats"
)

// mockPypiStats returns a test server that serves realistic pypistats.org
// responses with date fields, and records the request paths it receives.
func mockPypiStats(t *testing.T) (*httptest.Server, *[]string) {
	t.Helper()
	var requestPaths []string

	overallResp := `{
		"package": "django",
		"type": "overall_downloads",
		"data": [
			{"category": "with_mirrors", "date": "2026-03-03", "downloads": 500000},
			{"category": "without_mirrors", "date": "2026-03-03", "downloads": 400000},
			{"category": "with_mirrors", "date": "2026-03-04", "downloads": 520000},
			{"category": "without_mirrors", "date": "2026-03-04", "downloads": 410000},
			{"category": "with_mirrors", "date": "2026-03-10", "downloads": 530000},
			{"category": "without_mirrors", "date": "2026-03-10", "downloads": 420000},
			{"category": "with_mirrors", "date": "2026-03-11", "downloads": 540000},
			{"category": "without_mirrors", "date": "2026-03-11", "downloads": 430000}
		]
	}`

	pythonResp := `{
		"package": "django",
		"type": "python_minor_downloads",
		"data": [
			{"category": "3.12", "date": "2026-03-03", "downloads": 200000},
			{"category": "3.11", "date": "2026-03-03", "downloads": 150000},
			{"category": "3.10", "date": "2026-03-03", "downloads": 50000},
			{"category": "3.12", "date": "2026-03-04", "downloads": 210000},
			{"category": "3.11", "date": "2026-03-04", "downloads": 140000},
			{"category": "3.10", "date": "2026-03-04", "downloads": 55000},
			{"category": "null", "date": "2026-03-03", "downloads": 1000},
			{"category": "null", "date": "2026-03-04", "downloads": 1100}
		]
	}`

	systemResp := `{
		"package": "django",
		"type": "system_downloads",
		"data": [
			{"category": "Linux", "date": "2026-03-03", "downloads": 300000},
			{"category": "Darwin", "date": "2026-03-03", "downloads": 50000},
			{"category": "Windows", "date": "2026-03-03", "downloads": 40000},
			{"category": "null", "date": "2026-03-03", "downloads": 5000},
			{"category": "Linux", "date": "2026-03-04", "downloads": 310000},
			{"category": "Darwin", "date": "2026-03-04", "downloads": 52000},
			{"category": "Windows", "date": "2026-03-04", "downloads": 42000},
			{"category": "null", "date": "2026-03-04", "downloads": 5500}
		]
	}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPaths = append(requestPaths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")

		// Mimic pypistats.org: only lowercase package names return data.
		if !strings.Contains(r.URL.Path, "/packages/django/") {
			http.NotFound(w, r)
			return
		}

		switch {
		case strings.HasSuffix(r.URL.Path, "/overall"):
			w.Write([]byte(overallResp)) //nolint:errcheck
		case strings.HasSuffix(r.URL.Path, "/python_minor"):
			w.Write([]byte(pythonResp)) //nolint:errcheck
		case strings.HasSuffix(r.URL.Path, "/system"):
			w.Write([]byte(systemResp)) //nolint:errcheck
		default:
			http.NotFound(w, r)
		}
	}))

	return srv, &requestPaths
}

func setupHandler(t *testing.T, mock *httptest.Server) (*chi.Mux, *cache.Cache) {
	t.Helper()
	c, err := cache.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	statsClient := stats.NewClient(stats.WithBaseURL(mock.URL))
	h := handler.NewStatsHandler(statsClient, c)

	r := chi.NewRouter()
	r.Get("/api/packages/{name}/stats", h.Get)
	return r, c
}

// TestStatsUsesLowercaseForPypistats verifies that the handler lowercases
// the package name when calling pypistats.org, since their API is case-sensitive.
func TestStatsUsesLowercaseForPypistats(t *testing.T) {
	mock, paths := mockPypiStats(t)
	defer mock.Close()

	router, c := setupHandler(t, mock)
	defer c.Close()

	// Request with mixed-case name like "Django"
	req := httptest.NewRequest(http.MethodGet, "/api/packages/Django/stats", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// All pypistats requests should use lowercase package name
	for _, p := range *paths {
		if strings.Contains(p, "Django") {
			t.Errorf("pypistats request used original casing: %s (should be lowercase)", p)
		}
	}

	// Should still return aggregated data (mock is case-sensitive like real API)
	var body handler.CombinedStats
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if len(body.Overall) == 0 {
		t.Error("overall is empty — pypistats was likely called with wrong casing")
	}
	if len(body.Systems) == 0 {
		t.Error("systems is empty — pypistats was likely called with wrong casing")
	}
}

// TestStatsAggregatesOverallByWeek verifies that daily overall data is
// aggregated into weekly buckets using only "without_mirrors" entries.
func TestStatsAggregatesOverallByWeek(t *testing.T) {
	mock, _ := mockPypiStats(t)
	defer mock.Close()

	router, c := setupHandler(t, mock)
	defer c.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/packages/django/stats", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var body handler.CombinedStats
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	// Mock data has without_mirrors entries on 2026-03-03, 2026-03-04 (week 10)
	// and 2026-03-10, 2026-03-11 (week 11). Should produce 2 weekly buckets.
	if body.Overall == nil {
		t.Fatal("overall is nil, expected aggregated weekly data")
	}
	if len(body.Overall) != 2 {
		t.Fatalf("expected 2 weekly buckets, got %d: %+v", len(body.Overall), body.Overall)
	}

	// Week 10: 400000 + 410000 = 810000
	if body.Overall[0].Downloads != 810000 {
		t.Errorf("week 1 downloads: expected 810000, got %d", body.Overall[0].Downloads)
	}
	// Week 11: 420000 + 430000 = 850000
	if body.Overall[1].Downloads != 850000 {
		t.Errorf("week 2 downloads: expected 850000, got %d", body.Overall[1].Downloads)
	}
}

// TestStatsAggregatesPythonVersionsByCategory verifies that daily python version
// data is summed per version, sorted descending, with "null" entries excluded.
func TestStatsAggregatesPythonVersionsByCategory(t *testing.T) {
	mock, _ := mockPypiStats(t)
	defer mock.Close()

	router, c := setupHandler(t, mock)
	defer c.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/packages/django/stats", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	var body handler.CombinedStats
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	// Should have 3 versions (3.12, 3.11, 3.10), "null" excluded
	if len(body.PythonVersions) != 3 {
		t.Fatalf("expected 3 python versions, got %d: %+v", len(body.PythonVersions), body.PythonVersions)
	}

	// Should be sorted descending by downloads
	// 3.12: 200000+210000=410000, 3.11: 150000+140000=290000, 3.10: 50000+55000=105000
	expected := []struct {
		cat       string
		downloads int64
	}{
		{"3.12", 410000},
		{"3.11", 290000},
		{"3.10", 105000},
	}
	for i, exp := range expected {
		if body.PythonVersions[i].Category != exp.cat {
			t.Errorf("python_versions[%d] category: expected %q, got %q", i, exp.cat, body.PythonVersions[i].Category)
		}
		if body.PythonVersions[i].Downloads != exp.downloads {
			t.Errorf("python_versions[%d] downloads: expected %d, got %d", i, exp.downloads, body.PythonVersions[i].Downloads)
		}
	}
}

// TestStatsAggregatesSystemsByCategory verifies that daily system data is
// summed per OS, sorted descending, with "null" entries excluded.
func TestStatsAggregatesSystemsByCategory(t *testing.T) {
	mock, _ := mockPypiStats(t)
	defer mock.Close()

	router, c := setupHandler(t, mock)
	defer c.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/packages/django/stats", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	var body handler.CombinedStats
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	// Should have 3 systems (Linux, Darwin, Windows), "null" excluded
	if len(body.Systems) != 3 {
		t.Fatalf("expected 3 systems, got %d: %+v", len(body.Systems), body.Systems)
	}

	// Sorted desc: Linux 610000, Darwin 102000, Windows 82000
	if body.Systems[0].Category != "Linux" || body.Systems[0].Downloads != 610000 {
		t.Errorf("systems[0]: expected Linux/610000, got %s/%d", body.Systems[0].Category, body.Systems[0].Downloads)
	}
	if body.Systems[1].Category != "Darwin" || body.Systems[1].Downloads != 102000 {
		t.Errorf("systems[1]: expected Darwin/102000, got %s/%d", body.Systems[1].Category, body.Systems[1].Downloads)
	}
	if body.Systems[2].Category != "Windows" || body.Systems[2].Downloads != 82000 {
		t.Errorf("systems[2]: expected Windows/82000, got %s/%d", body.Systems[2].Category, body.Systems[2].Downloads)
	}
}

// TestStatsPreservesOriginalCaseInResponse verifies that even though we
// lowercase the name for pypistats, the response preserves the original casing.
func TestStatsPreservesOriginalCaseInResponse(t *testing.T) {
	mock, _ := mockPypiStats(t)
	defer mock.Close()

	router, c := setupHandler(t, mock)
	defer c.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/packages/Django/stats", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	var body handler.CombinedStats
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if body.Package != "Django" {
		t.Errorf("expected package name 'Django' (original case), got %q", body.Package)
	}
}
