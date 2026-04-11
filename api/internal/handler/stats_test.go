package handler_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pypx/api/internal/cache"
	"github.com/pypx/api/internal/handler"
	"github.com/pypx/api/internal/stats"
)

// generateDailyData builds mock pypistats daily entries spanning the given
// number of days back from today for the specified categories.
func generateDailyData(categories []string, days int, downloadsPerDay int64) string {
	var entries []string
	now := time.Now().UTC().Truncate(24 * time.Hour)
	for d := days - 1; d >= 0; d-- {
		date := now.AddDate(0, 0, -d).Format("2006-01-02")
		for _, cat := range categories {
			entries = append(entries, fmt.Sprintf(
				`{"category":%q,"date":%q,"downloads":%d}`,
				cat, date, downloadsPerDay,
			))
		}
	}
	return "[" + strings.Join(entries, ",") + "]"
}

// mockPypiStatsWide returns a test server with 180 days of data, suitable for
// period-filter tests. Also records request paths.
func mockPypiStatsWide(t *testing.T) (*httptest.Server, *[]string) {
	t.Helper()
	var requestPaths []string

	overallData := generateDailyData([]string{"with_mirrors", "without_mirrors"}, 180, 100000)
	pythonData := generateDailyData([]string{"3.12", "3.11", "3.10"}, 180, 50000)
	systemData := generateDailyData([]string{"Linux", "Darwin", "Windows"}, 180, 30000)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPaths = append(requestPaths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")

		if !strings.Contains(r.URL.Path, "/packages/django/") {
			http.NotFound(w, r)
			return
		}

		switch {
		case strings.HasSuffix(r.URL.Path, "/overall"):
			fmt.Fprintf(w, `{"package":"django","type":"overall","data":%s}`, overallData)
		case strings.HasSuffix(r.URL.Path, "/python_minor"):
			fmt.Fprintf(w, `{"package":"django","type":"python_minor","data":%s}`, pythonData)
		case strings.HasSuffix(r.URL.Path, "/system"):
			fmt.Fprintf(w, `{"package":"django","type":"system","data":%s}`, systemData)
		default:
			http.NotFound(w, r)
		}
	}))

	return srv, &requestPaths
}

// mockPypiStatsNarrow returns a test server with a small fixed dataset for
// simple aggregation tests (not period-dependent).
func mockPypiStatsNarrow(t *testing.T) (*httptest.Server, *[]string) {
	t.Helper()
	var requestPaths []string

	// Use dates within the last 28 days so they always pass a 4w filter.
	now := time.Now().UTC().Truncate(24 * time.Hour)
	d1 := now.AddDate(0, 0, -10).Format("2006-01-02")
	d2 := now.AddDate(0, 0, -9).Format("2006-01-02")
	d3 := now.AddDate(0, 0, -3).Format("2006-01-02")
	d4 := now.AddDate(0, 0, -2).Format("2006-01-02")

	overallResp := fmt.Sprintf(`{
		"package": "django", "type": "overall_downloads",
		"data": [
			{"category": "with_mirrors", "date": "%s", "downloads": 500000},
			{"category": "without_mirrors", "date": "%s", "downloads": 400000},
			{"category": "with_mirrors", "date": "%s", "downloads": 520000},
			{"category": "without_mirrors", "date": "%s", "downloads": 410000},
			{"category": "with_mirrors", "date": "%s", "downloads": 530000},
			{"category": "without_mirrors", "date": "%s", "downloads": 420000},
			{"category": "with_mirrors", "date": "%s", "downloads": 540000},
			{"category": "without_mirrors", "date": "%s", "downloads": 430000}
		]
	}`, d1, d1, d2, d2, d3, d3, d4, d4)

	pythonResp := fmt.Sprintf(`{
		"package": "django", "type": "python_minor_downloads",
		"data": [
			{"category": "3.12", "date": "%s", "downloads": 200000},
			{"category": "3.11", "date": "%s", "downloads": 150000},
			{"category": "3.10", "date": "%s", "downloads": 50000},
			{"category": "3.12", "date": "%s", "downloads": 210000},
			{"category": "3.11", "date": "%s", "downloads": 140000},
			{"category": "3.10", "date": "%s", "downloads": 55000},
			{"category": "null", "date": "%s", "downloads": 1000},
			{"category": "null", "date": "%s", "downloads": 1100}
		]
	}`, d1, d1, d1, d2, d2, d2, d1, d2)

	systemResp := fmt.Sprintf(`{
		"package": "django", "type": "system_downloads",
		"data": [
			{"category": "Linux", "date": "%s", "downloads": 300000},
			{"category": "Darwin", "date": "%s", "downloads": 50000},
			{"category": "Windows", "date": "%s", "downloads": 40000},
			{"category": "null", "date": "%s", "downloads": 5000},
			{"category": "Linux", "date": "%s", "downloads": 310000},
			{"category": "Darwin", "date": "%s", "downloads": 52000},
			{"category": "Windows", "date": "%s", "downloads": 42000},
			{"category": "null", "date": "%s", "downloads": 5500}
		]
	}`, d1, d1, d1, d1, d2, d2, d2, d2)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPaths = append(requestPaths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")

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

func fetchStats(t *testing.T, router *chi.Mux, url string) handler.CombinedStats {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, url, nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var body handler.CombinedStats
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	return body
}

// ---------------------------------------------------------------------------
// Existing tests (updated to use narrow mock with dynamic dates)
// ---------------------------------------------------------------------------

func TestStatsUsesLowercaseForPypistats(t *testing.T) {
	mock, paths := mockPypiStatsNarrow(t)
	defer mock.Close()

	router, c := setupHandler(t, mock)
	defer c.Close()

	body := fetchStats(t, router, "/api/packages/Django/stats")

	for _, p := range *paths {
		if strings.Contains(p, "Django") {
			t.Errorf("pypistats request used original casing: %s (should be lowercase)", p)
		}
	}
	if len(body.Overall) == 0 {
		t.Error("overall is empty — pypistats was likely called with wrong casing")
	}
	if len(body.Systems) == 0 {
		t.Error("systems is empty — pypistats was likely called with wrong casing")
	}
}

func TestStatsAggregatesOverallByWeek(t *testing.T) {
	mock, _ := mockPypiStatsNarrow(t)
	defer mock.Close()

	router, c := setupHandler(t, mock)
	defer c.Close()

	body := fetchStats(t, router, "/api/packages/django/stats")

	if body.Overall == nil {
		t.Fatal("overall is nil, expected aggregated weekly data")
	}
	if len(body.Overall) < 1 {
		t.Fatal("expected at least 1 weekly bucket")
	}

	// All without_mirrors data sums to 400000+410000+420000+430000 = 1660000
	var total int64
	for _, p := range body.Overall {
		total += p.Downloads
	}
	if total != 1660000 {
		t.Errorf("total overall downloads: expected 1660000, got %d", total)
	}
}

func TestStatsAggregatesPythonVersionsByCategory(t *testing.T) {
	mock, _ := mockPypiStatsNarrow(t)
	defer mock.Close()

	router, c := setupHandler(t, mock)
	defer c.Close()

	body := fetchStats(t, router, "/api/packages/django/stats")

	if len(body.PythonVersions) != 3 {
		t.Fatalf("expected 3 python versions, got %d: %+v", len(body.PythonVersions), body.PythonVersions)
	}

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

func TestStatsAggregatesSystemsByCategory(t *testing.T) {
	mock, _ := mockPypiStatsNarrow(t)
	defer mock.Close()

	router, c := setupHandler(t, mock)
	defer c.Close()

	body := fetchStats(t, router, "/api/packages/django/stats")

	if len(body.Systems) != 3 {
		t.Fatalf("expected 3 systems, got %d: %+v", len(body.Systems), body.Systems)
	}

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

func TestStatsDoesNotCacheEmptyResults(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")

		if callCount <= 3 {
			http.NotFound(w, r)
			return
		}

		now := time.Now().UTC().Format("2006-01-02")
		resp := fmt.Sprintf(`{"package":"test","type":"test","data":[
			{"category":"Linux","date":"%s","downloads":100}
		]}`, now)
		w.Write([]byte(resp)) //nolint:errcheck
	}))
	defer srv.Close()

	c, err := cache.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer c.Close()

	statsClient := stats.NewClient(stats.WithBaseURL(srv.URL))
	h := handler.NewStatsHandler(statsClient, c)

	r := chi.NewRouter()
	r.Get("/api/packages/{name}/stats", h.Get)

	req1 := httptest.NewRequest(http.MethodGet, "/api/packages/test/stats", nil)
	rr1 := httptest.NewRecorder()
	r.ServeHTTP(rr1, req1)

	req2 := httptest.NewRequest(http.MethodGet, "/api/packages/test/stats", nil)
	rr2 := httptest.NewRecorder()
	r.ServeHTTP(rr2, req2)

	var body2 handler.CombinedStats
	if err := json.NewDecoder(rr2.Body).Decode(&body2); err != nil {
		t.Fatalf("failed to decode second response: %v", err)
	}
	if len(body2.Systems) == 0 {
		t.Error("second request returned empty systems — empty result was cached from first failed request")
	}
}

func TestStatsPreservesOriginalCaseInResponse(t *testing.T) {
	mock, _ := mockPypiStatsNarrow(t)
	defer mock.Close()

	router, c := setupHandler(t, mock)
	defer c.Close()

	body := fetchStats(t, router, "/api/packages/Django/stats")
	if body.Package != "Django" {
		t.Errorf("expected package name 'Django' (original case), got %q", body.Package)
	}
}

// ---------------------------------------------------------------------------
// Period filter tests (NEW — should fail until implemented)
// ---------------------------------------------------------------------------

// TestStatsPeriodFiltersData verifies that period=4w only includes data from
// the last 28 days, while period=6m includes all 180 days.
func TestStatsPeriodFiltersData(t *testing.T) {
	mock, _ := mockPypiStatsWide(t)
	defer mock.Close()

	router, c := setupHandler(t, mock)
	defer c.Close()

	body4w := fetchStats(t, router, "/api/packages/django/stats?period=4w")
	body6m := fetchStats(t, router, "/api/packages/django/stats?period=6m")

	// 4w: ~28 days of data at 100k/day without_mirrors = ~2.8M total
	// 6m: ~180 days of data at 100k/day without_mirrors = ~18M total
	// The 6m total should be significantly larger than 4w.
	var total4w, total6m int64
	for _, p := range body4w.Overall {
		total4w += p.Downloads
	}
	for _, p := range body6m.Overall {
		total6m += p.Downloads
	}

	if total6m <= total4w {
		t.Errorf("6m total (%d) should be much larger than 4w total (%d)", total6m, total4w)
	}

	// 4w should have at most 4 weekly buckets
	if len(body4w.Overall) > 4 {
		t.Errorf("4w should have at most 4 weekly buckets, got %d", len(body4w.Overall))
	}
	// 6m should have many more buckets than 4w
	if len(body6m.Overall) <= len(body4w.Overall) {
		t.Errorf("6m buckets (%d) should be more than 4w buckets (%d)", len(body6m.Overall), len(body4w.Overall))
	}
}

// TestStatsPeriodDefaultsTo4w verifies that omitting the period param behaves
// the same as explicitly requesting period=4w.
func TestStatsPeriodDefaultsTo4w(t *testing.T) {
	mock, _ := mockPypiStatsWide(t)
	defer mock.Close()

	router, c := setupHandler(t, mock)
	defer c.Close()

	bodyDefault := fetchStats(t, router, "/api/packages/django/stats")
	body4w := fetchStats(t, router, "/api/packages/django/stats?period=4w")

	if bodyDefault.Period != "4w" {
		t.Errorf("default period: expected '4w', got %q", bodyDefault.Period)
	}
	if body4w.Period != "4w" {
		t.Errorf("explicit 4w period: expected '4w', got %q", body4w.Period)
	}

	// Same data should be returned
	if len(bodyDefault.Overall) != len(body4w.Overall) {
		t.Errorf("default and 4w should have same bucket count: %d vs %d",
			len(bodyDefault.Overall), len(body4w.Overall))
	}
}

// TestStatsPeriodInvalidFallback verifies that an invalid period value falls
// back to 4w.
func TestStatsPeriodInvalidFallback(t *testing.T) {
	mock, _ := mockPypiStatsWide(t)
	defer mock.Close()

	router, c := setupHandler(t, mock)
	defer c.Close()

	body := fetchStats(t, router, "/api/packages/django/stats?period=bogus")
	if body.Period != "4w" {
		t.Errorf("invalid period should fallback to '4w', got %q", body.Period)
	}
}

// TestStatsDateRangeInResponse verifies that the response includes period and
// date_range fields with correct values.
func TestStatsDateRangeInResponse(t *testing.T) {
	mock, _ := mockPypiStatsWide(t)
	defer mock.Close()

	router, c := setupHandler(t, mock)
	defer c.Close()

	body := fetchStats(t, router, "/api/packages/django/stats?period=3m")

	if body.Period != "3m" {
		t.Errorf("period: expected '3m', got %q", body.Period)
	}
	if body.DateRange == nil {
		t.Fatal("date_range is nil")
	}
	if body.DateRange.From == "" || body.DateRange.To == "" {
		t.Errorf("date_range fields should not be empty: from=%q, to=%q",
			body.DateRange.From, body.DateRange.To)
	}

	// Parse dates and verify the range is approximately 90 days
	from, err := time.Parse("2006-01-02", body.DateRange.From)
	if err != nil {
		t.Fatalf("invalid from date: %v", err)
	}
	to, err := time.Parse("2006-01-02", body.DateRange.To)
	if err != nil {
		t.Fatalf("invalid to date: %v", err)
	}
	days := to.Sub(from).Hours() / 24
	if days < 80 || days > 95 {
		t.Errorf("3m date range should span ~90 days, got %.0f (%s to %s)",
			days, body.DateRange.From, body.DateRange.To)
	}
}
