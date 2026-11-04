package handler

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pypx/api/internal/pypi"
	"github.com/pypx/goopy/model"
)

func TestDocsWork_ConcurrentCallersShareOneExtraction(t *testing.T) {
	c := newTTLCache()
	var calls atomic.Int32
	release := make(chan struct{})
	h := docsCacheHandler(c, func(context.Context, string, string) (*model.Package, error) {
		calls.Add(1)
		<-release
		return &model.Package{}, nil
	})

	const callers = 20
	var wg sync.WaitGroup
	errs := make([]error, callers)
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, _, errs[i] = h.fetchDocsAtVersion(context.Background(), "p", "1")
		}(i)
	}
	time.Sleep(100 * time.Millisecond) // let every caller reach the shared call
	close(release)
	wg.Wait()

	if got := calls.Load(); got != 1 {
		t.Errorf("extract ran %d times, want 1", got)
	}
	for i, err := range errs {
		if err != nil {
			t.Errorf("caller %d: %v", i, err)
		}
	}
}

func TestDocsWork_ExtractionsAreCapped(t *testing.T) {
	c := newTTLCache()
	var running, peak atomic.Int32
	release := make(chan struct{})
	h := docsCacheHandler(c, func(context.Context, string, string) (*model.Package, error) {
		n := running.Add(1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		<-release
		running.Add(-1)
		return &model.Package{}, nil
	})

	var wg sync.WaitGroup
	for _, v := range []string{"1", "2", "3", "4", "5", "6"} {
		wg.Add(1)
		go func(v string) {
			defer wg.Done()
			if _, _, err := h.fetchDocsAtVersion(context.Background(), "p", v); err != nil {
				t.Errorf("version %s: %v", v, err)
			}
		}(v)
	}
	time.Sleep(150 * time.Millisecond) // the first two fill the budget, the rest wait
	const want = extractionBudget / unweighedExtractionCost
	if got := running.Load(); got != want {
		t.Errorf("running = %d, want %d", got, want)
	}
	close(release)
	wg.Wait()
	if got := peak.Load(); got != want {
		t.Errorf("peak concurrency = %d, want %d", got, want)
	}
}

func TestDocsWork_WaitingCallerCancelWritesNothingAndOthersSucceed(t *testing.T) {
	c := newTTLCache()
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	var workCtxErr atomic.Value
	h := docsCacheHandler(c, func(ctx context.Context, _, _ string) (*model.Package, error) {
		started <- struct{}{}
		<-release
		workCtxErr.Store(ctx.Err() == nil)
		return &model.Package{}, nil
	})

	// Caller A starts the shared work and then leaves.
	ctxA, cancelA := context.WithCancel(context.Background())
	aDone := make(chan error, 1)
	go func() {
		_, _, err := h.fetchDocsAtVersion(ctxA, "p", "1")
		aDone <- err
	}()
	<-started

	// Caller B joins the same work.
	bDone := make(chan error, 1)
	go func() {
		_, _, err := h.fetchDocsAtVersion(context.Background(), "p", "1")
		bDone <- err
	}()
	time.Sleep(50 * time.Millisecond)

	cancelA()
	if err := <-aDone; !errors.Is(err, ErrDocsExtractionFailed) {
		t.Fatalf("caller A err = %v, want ErrDocsExtractionFailed", err)
	}
	if c.wrote(DocsErrKey("p", "1")) {
		t.Error("cancelled waiter wrote the error key")
	}

	close(release)
	if err := <-bDone; err != nil {
		t.Fatalf("caller B err = %v, want nil", err)
	}
	if ok, _ := workCtxErr.Load().(bool); !ok {
		t.Error("shared work context was cancelled by caller A")
	}
	if c.wrote(DocsErrKey("p", "1")) {
		t.Error("error key written although extraction succeeded")
	}
	if !c.wrote(DocsCacheKey("p", "1")) {
		t.Error("docs key not written")
	}
}

func TestDocsWork_TimeoutWritesErrorKeyNotDocsKey(t *testing.T) {
	c := newTTLCache()
	h := docsCacheHandler(c, func(ctx context.Context, _, _ string) (*model.Package, error) {
		<-ctx.Done() // the shared work context reaches its deadline
		return &model.Package{}, nil
	})
	h.workTimeout = 20 * time.Millisecond
	_, _, err := h.fetchDocsAtVersion(context.Background(), "p", "1")
	if !errors.Is(err, ErrDocsExtractionFailed) {
		t.Fatalf("err = %v, want ErrDocsExtractionFailed", err)
	}
	if c.wrote(DocsCacheKey("p", "1")) {
		t.Error("docs key written after timeout")
	}
	if !c.wrote(DocsErrKey("p", "1")) {
		t.Error("error key not written after timeout")
	}
}

func TestDocsWork_SlotWaitCountsAgainstTimeout(t *testing.T) {
	c := newTTLCache()
	release := make(chan struct{})
	defer close(release)
	h := docsCacheHandler(c, func(context.Context, string, string) (*model.Package, error) {
		<-release
		return &model.Package{}, nil
	})
	h.workTimeout = 50 * time.Millisecond
	if !h.budget.TryAcquire(extractionBudget) { // occupy the whole budget
		t.Fatal("budget not free")
	}
	_, _, err := h.fetchDocsAtVersion(context.Background(), "p", "1")
	if !errors.Is(err, ErrDocsExtractionFailed) {
		t.Fatalf("err = %v, want ErrDocsExtractionFailed", err)
	}
}

// trackRunning returns an extract function that blocks until release closes, and
// reports which packages run.
func trackRunning(release <-chan struct{}) (func(context.Context, string, string) (*model.Package, error), func() []string) {
	var mu sync.Mutex
	running := map[string]bool{}
	return func(_ context.Context, name, _ string) (*model.Package, error) {
			mu.Lock()
			running[name] = true
			mu.Unlock()
			<-release
			mu.Lock()
			delete(running, name)
			mu.Unlock()
			return &model.Package{}, nil
		}, func() []string {
			mu.Lock()
			defer mu.Unlock()
			out := make([]string, 0, len(running))
			for name := range running {
				out = append(out, name)
			}
			slices.Sort(out)
			return out
		}
}

func TestDocsWork_LargeExtractionsDoNotOverlapButSmallOnesFit(t *testing.T) {
	weights := map[string]int64{
		"huge-a":  extractionBudget - 1<<20,
		"huge-b":  extractionBudget - 1<<20,
		"small-a": 1 << 20,
		"small-b": 1 << 20,
	}
	release := make(chan struct{})
	extract, running := trackRunning(release)
	h := docsCacheHandler(newTTLCache(), extract)
	h.weigh = func(_ context.Context, name, _ string) (int64, error) { return weights[name], nil }

	var wg sync.WaitGroup
	start := func(name string) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, _, err := h.fetchDocsAtVersion(context.Background(), name, "1"); err != nil {
				t.Errorf("%s: %v", name, err)
			}
		}()
		time.Sleep(50 * time.Millisecond) // arrive in this order
	}
	// small-a and small-b fit beside each other. huge-a then waits for them,
	// and huge-b waits behind huge-a.
	start("small-a")
	start("small-b")
	start("huge-a")
	start("huge-b")
	if got := running(); !slices.Equal(got, []string{"small-a", "small-b"}) {
		t.Errorf("running = %v, want the two small packages", got)
	}
	close(release)
	wg.Wait()
}

func TestDocsWork_ExtractionAboveBudgetRunsAlone(t *testing.T) {
	release := make(chan struct{})
	extract, running := trackRunning(release)
	h := docsCacheHandler(newTTLCache(), extract)
	h.weigh = func(_ context.Context, name, _ string) (int64, error) {
		if name == "enormous" {
			return 10 * extractionBudget, nil
		}
		return 1, nil
	}

	var wg sync.WaitGroup
	for _, name := range []string{"enormous", "tiny"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, _, err := h.fetchDocsAtVersion(context.Background(), name, "1"); err != nil {
				t.Errorf("%s: %v", name, err)
			}
		}()
		time.Sleep(50 * time.Millisecond)
	}
	if got := running(); !slices.Equal(got, []string{"enormous"}) {
		t.Errorf("running = %v, want only the enormous package", got)
	}
	close(release)
	wg.Wait()
}

func TestDocsWork_WeighFailureFallsThroughToExtraction(t *testing.T) {
	extractErr := errors.New("real failure")
	c := newTTLCache()
	h := docsCacheHandler(c, func(context.Context, string, string) (*model.Package, error) {
		return nil, extractErr
	})
	h.weigh = func(context.Context, string, string) (int64, error) {
		return 0, errors.New("metadata unavailable")
	}
	if _, _, err := h.fetchDocsAtVersion(context.Background(), "p", "1"); !errors.Is(err, ErrDocsExtractionFailed) {
		t.Fatalf("err = %v, want ErrDocsExtractionFailed", err)
	}
	if !c.wrote(DocsErrKey("p", "1")) {
		t.Error("extraction did not run after the weigh failed")
	}
	if !h.budget.TryAcquire(extractionBudget) {
		t.Error("budget still held")
	}
}

func TestDocsWork_MissDecodesLikeAHit(t *testing.T) {
	c := newTTLCache()
	h := docsCacheHandler(c, func(context.Context, string, string) (*model.Package, error) {
		return &model.Package{Modules: []*model.Module{{
			Name:      "p",
			Functions: []*model.Function{{Name: "f"}},
		}}}, nil
	})

	missResp, missBytes, err := h.fetchDocsAtVersion(context.Background(), "p", "1")
	if err != nil {
		t.Fatal(err)
	}
	if !missResp.Available || len(missResp.Modules) != 1 {
		t.Fatalf("miss response = %+v", missResp)
	}
	hitResp, hitBytes, err := h.fetchDocsAtVersion(context.Background(), "p", "1")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(missResp, hitResp) {
		t.Errorf("miss = %+v, hit = %+v", missResp, hitResp)
	}
	if !bytes.Equal(missBytes, hitBytes) {
		t.Error("miss and hit returned different bytes")
	}

	rawResp, rawBytes, err := h.docsAtVersion(context.Background(), "p", "2", false)
	if err != nil {
		t.Fatal(err)
	}
	if rawResp.Package != "" || len(rawBytes) == 0 {
		t.Errorf("decode=false returned resp %+v and %d bytes", rawResp, len(rawBytes))
	}
}

func TestDocsWork_PanicInWeighReleasesBudget(t *testing.T) {
	c := newTTLCache()
	h := docsCacheHandler(c, func(context.Context, string, string) (*model.Package, error) {
		return &model.Package{}, nil
	})
	h.weigh = func(context.Context, string, string) (int64, error) { panic("boom") }
	if _, _, err := h.fetchDocsAtVersion(context.Background(), "p", "1"); !errors.Is(err, ErrDocsExtractionFailed) {
		t.Fatalf("err = %v, want ErrDocsExtractionFailed", err)
	}
	if !h.budget.TryAcquire(extractionBudget) {
		t.Error("budget still held after a panic in weigh")
	}
}

func TestDocsAvailable(t *testing.T) {
	cases := []struct {
		name      string
		in        string
		available bool
		ok        bool
	}{
		{"true", `{"package":"p","version":"1","available":true,"modules":[{"name":"m"}]}`, true, true},
		{"false", `{"package":"p","version":"1","available":false,"modules":[]}`, false, true},
		{"after modules", `{"modules":[{"a":1}],"available":true}`, true, true},
		{"string containing key text", `{"package":"\"available\":true","available":false}`, false, true},
		{"missing", `{"package":"p"}`, false, false},
		{"truncated", `{"package":"p","avail`, false, false},
		{"not an object", `[1,2]`, false, false},
		{"empty", ``, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a, ok := docsAvailable([]byte(tc.in))
			if a != tc.available || ok != tc.ok {
				t.Errorf("docsAvailable = (%v, %v), want (%v, %v)", a, ok, tc.available, tc.ok)
			}
		})
	}
}

// fakePyPI serves {"info":{"version":v}} for every package and counts requests.
func fakePyPI(t *testing.T, version string, status int) (*pypi.Client, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if status != http.StatusOK {
			http.Error(w, "nope", status)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"info":{"name":"p","version":"` + version + `"},"urls":[],"releases":{}}`)) //nolint:errcheck
	}))
	t.Cleanup(srv.Close)
	return pypi.NewClient(pypi.WithBaseURL(srv.URL)), &hits
}

func TestDocsWork_FetchDocsBytesReturnsCachedBytesUnchanged(t *testing.T) {
	client, _ := fakePyPI(t, "1.0", http.StatusOK)
	c := newTTLCache()
	// Spacing and key order that json.Marshal would not reproduce.
	raw := []byte("{ \"available\" : true ,\n \"package\":\"p\", \"version\":\"1.0\", \"modules\":[] }")
	c.put(DocsCacheKey("p", "1.0"), raw, time.Hour)
	h := docsCacheHandler(c, func(context.Context, string, string) (*model.Package, error) {
		t.Fatal("extract must not run")
		return nil, nil
	})
	h.pypi = client

	got, err := h.fetchDocsBytes(context.Background(), "p")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(raw) {
		t.Errorf("bytes changed:\n got %q\nwant %q", got, raw)
	}
}

func TestDocsWork_FetchDocsBytesTreatsCorruptEntryAsMiss(t *testing.T) {
	client, _ := fakePyPI(t, "1.0", http.StatusOK)
	c := newTTLCache()
	c.put(DocsCacheKey("p", "1.0"), []byte(`{"package":`), time.Hour)
	var calls atomic.Int32
	h := docsCacheHandler(c, func(context.Context, string, string) (*model.Package, error) {
		calls.Add(1)
		return &model.Package{}, nil
	})
	h.pypi = client
	if _, err := h.fetchDocsBytes(context.Background(), "p"); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Errorf("extract ran %d times, want 1", calls.Load())
	}
}

func TestDocsWork_LatestVersionIsCached(t *testing.T) {
	client, hits := fakePyPI(t, "2.0", http.StatusOK)
	c := newTTLCache()
	h := docsCacheHandler(c, func(context.Context, string, string) (*model.Package, error) {
		return &model.Package{}, nil
	})
	h.pypi = client

	for i := 0; i < 2; i++ {
		if _, err := h.fetchDocsBytes(context.Background(), "P"); err != nil {
			t.Fatal(err)
		}
	}
	if got := hits.Load(); got != 1 {
		t.Errorf("PyPI called %d times, want 1", got)
	}
	if _, fresh, _ := c.Get("docs-latest:p", docsLatestTTL); !fresh {
		t.Error("docs-latest:p not cached under the lower-case name")
	}
}

func TestDocsWork_LatestVersionServesStaleWhenPyPIFails(t *testing.T) {
	client, hits := fakePyPI(t, "", http.StatusInternalServerError)
	c := newTTLCache()
	c.put("docs-latest:p", []byte("1.5"), time.Hour) // past the 5 minute TTL
	h := docsCacheHandler(c, nil)
	h.pypi = client

	got, err := h.latestVersion(context.Background(), "p")
	if err != nil || got != "1.5" {
		t.Fatalf("latestVersion = %q, %v; want 1.5, nil", got, err)
	}
	if hits.Load() == 0 {
		t.Error("PyPI was not tried before serving the stale value")
	}
}

func TestDocsWork_LatestVersionNotFoundStays404(t *testing.T) {
	client, _ := fakePyPI(t, "", http.StatusNotFound)
	c := newTTLCache()
	c.put("docs-latest:p", []byte("1.5"), time.Hour)
	h := docsCacheHandler(c, nil)
	h.pypi = client

	_, err := h.latestVersion(context.Background(), "p")
	if !errors.Is(err, ErrDocsPackageNotFound) {
		t.Fatalf("err = %v, want ErrDocsPackageNotFound", err)
	}
}

func TestDocsWork_LatestVersionUpstreamFailureWithoutStale(t *testing.T) {
	client, _ := fakePyPI(t, "", http.StatusInternalServerError)
	h := docsCacheHandler(newTTLCache(), nil)
	h.pypi = client
	_, err := h.latestVersion(context.Background(), "p")
	if !errors.Is(err, ErrDocsUpstreamFailed) {
		t.Fatalf("err = %v, want ErrDocsUpstreamFailed", err)
	}
}

func TestDocsWork_GetStatus(t *testing.T) {
	cases := []struct {
		name     string
		payload  string
		wantBody string
	}{
		{"available", `{"package":"p","version":"1.0","available":true,"modules":[]}`, `{"available":true}`},
		{"unavailable", `{"package":"p","version":"1.0","available":false,"modules":[]}`, `{"available":false}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client, _ := fakePyPI(t, "1.0", http.StatusOK)
			c := newTTLCache()
			c.put(DocsCacheKey("p", "1.0"), []byte(tc.payload), time.Minute)
			h := docsCacheHandler(c, func(context.Context, string, string) (*model.Package, error) {
				t.Fatal("extract must not run")
				return nil, nil
			})
			h.pypi = client

			r := chi.NewRouter()
			r.Get("/api/packages/{name}/docs/status", h.GetStatus)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/packages/p/docs/status", nil))

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d", rec.Code)
			}
			if got := strings.TrimSpace(rec.Body.String()); got != tc.wantBody {
				t.Errorf("body = %s, want %s", got, tc.wantBody)
			}
		})
	}
}

func TestDocsWork_GetStatusNotFound(t *testing.T) {
	client, _ := fakePyPI(t, "", http.StatusNotFound)
	h := docsCacheHandler(newTTLCache(), nil)
	h.pypi = client
	r := chi.NewRouter()
	r.Get("/api/packages/{name}/docs/status", h.GetStatus)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/packages/nope/docs/status", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestDocsWork_PanicInExtractionIsContained(t *testing.T) {
	c := newTTLCache()
	h := docsCacheHandler(c, func(_ context.Context, _, version string) (*model.Package, error) {
		if version == "1" {
			panic("boom")
		}
		return &model.Package{}, nil
	})

	_, _, err := h.fetchDocsAtVersion(context.Background(), "p", "1")
	if !errors.Is(err, ErrDocsExtractionFailed) {
		t.Fatalf("err = %v, want ErrDocsExtractionFailed", err)
	}
	if !c.wrote(DocsErrKey("p", "1")) {
		t.Error("error key not written after panic")
	}
	if c.wrote(DocsCacheKey("p", "1")) {
		t.Error("docs key written after panic")
	}
	if !h.budget.TryAcquire(extractionBudget) {
		t.Error("extraction budget still held after panic")
	} else {
		h.budget.Release(extractionBudget)
	}

	// Later extractions for other versions still run.
	for _, v := range []string{"2", "3", "4"} {
		if _, _, err := h.fetchDocsAtVersion(context.Background(), "p", v); err != nil {
			t.Errorf("version %s after panic: %v", v, err)
		}
	}
}

func TestDocsWork_NameCaseSharesOneExtraction(t *testing.T) {
	c := newTTLCache()
	var calls atomic.Int32
	release := make(chan struct{})
	h := docsCacheHandler(c, func(context.Context, string, string) (*model.Package, error) {
		calls.Add(1)
		<-release
		return &model.Package{}, nil
	})

	var wg sync.WaitGroup
	for _, name := range []string{"Flask", "flask", "FLASK", "flask"} {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			if _, _, err := h.fetchDocsAtVersion(context.Background(), name, "1"); err != nil {
				t.Errorf("%s: %v", name, err)
			}
		}(name)
	}
	time.Sleep(100 * time.Millisecond)
	close(release)
	wg.Wait()

	if got := calls.Load(); got != 1 {
		t.Errorf("extract ran %d times, want 1", got)
	}
}
