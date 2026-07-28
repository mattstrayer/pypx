package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/pypx/api/internal/middleware"
)

// okHandler is a simple handler that always returns 200.
var okHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
})

func TestRateLimiter_AllowsRequestsWithinLimit(t *testing.T) {
	// burst=5, rate=10 — plenty of headroom for 3 requests
	limiter := middleware.NewRateLimiter(10, 5)
	handler := limiter.Limit(okHandler)

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "192.168.1.1:1234"
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("request %d: got %d, want %d", i+1, rr.Code, http.StatusOK)
		}
	}
}

func TestRateLimiter_Returns429WhenExhausted(t *testing.T) {
	// burst=2, rate=0.001 — refills almost never, so 3rd request should be rejected
	limiter := middleware.NewRateLimiter(0.001, 2)
	handler := limiter.Limit(okHandler)

	ip := "10.0.0.1:5000"

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = ip
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("request %d should be allowed, got %d", i+1, rr.Code)
		}
	}

	// Third request should be rate-limited
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = ip
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", rr.Code)
	}
}

func TestRateLimiter_TokenRefillOverTime(t *testing.T) {
	// burst=1, rate=100 — refills 100 tokens/sec, so after 20ms we should have ~2 tokens
	limiter := middleware.NewRateLimiter(100, 1)
	handler := limiter.Limit(okHandler)

	ip := "10.0.0.2:5000"

	// Consume the single burst token
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = ip
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("first request should succeed, got %d", rr.Code)
	}

	// Next request immediately should fail
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = ip
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("immediate second request should fail, got %d", rr.Code)
	}

	// Wait for refill: 100 tokens/sec → 1 token in 10ms; sleep 20ms to be safe
	time.Sleep(20 * time.Millisecond)

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = ip
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("after refill, request should succeed, got %d", rr.Code)
	}
}

func TestRateLimiter_XForwardedForExtraction(t *testing.T) {
	// burst=1, rate=0.001 — one request allowed then blocked per IP.
	// Extraction now uses the RIGHTMOST XFF entry (set by the nearest
	// trusted proxy), so two requests with the SAME rightmost entry but
	// DIFFERENT (attacker-controlled) leftmost entries must still land in
	// the same bucket.
	limiter := middleware.NewRateLimiter(0.001, 1)
	handler := limiter.Limit(okHandler)

	// First request — leftmost entry is attacker-controlled, rightmost is
	// the trusted proxy's appended peer IP.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.5, 10.0.0.1")
	req.RemoteAddr = "10.0.0.1:1234" // proxy IP
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("first XFF request should succeed, got %d", rr.Code)
	}

	// Second request — leftmost entry rotated (simulating a spoofing
	// attacker), but rightmost entry is unchanged — should still be limited.
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "198.51.100.9, 10.0.0.1")
	req.RemoteAddr = "10.0.0.1:1235"
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("second XFF request (rotated leftmost) should be limited, got %d", rr.Code)
	}
}

func TestRateLimiter_SpoofedXFFDoesNotBypassLimit(t *testing.T) {
	// burst=1, rate=0.001. A client behind Cloudflare cannot spoof
	// CF-Connecting-IP, so bucketing on it must not be bypassable by
	// rotating the client-controlled leftmost XFF entry (or RemoteAddr port).
	limiter := middleware.NewRateLimiter(0.001, 1)
	handler := limiter.Limit(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("CF-Connecting-IP", "203.0.113.7")
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	req.RemoteAddr = "10.0.0.1:1234"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("first request should succeed, got %d", rr.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("CF-Connecting-IP", "203.0.113.7")
	req.Header.Set("X-Forwarded-For", "9.9.9.9") // rotated, attacker-controlled
	req.RemoteAddr = "10.0.0.1:9999"             // rotated port too
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("second request with rotated XFF/port should still be limited, got %d", rr.Code)
	}
}

func TestRateLimiter_CFConnectingIPPreferred(t *testing.T) {
	// burst=1, rate=0.001. Different CF-Connecting-IP values must get
	// independent buckets even when X-Forwarded-For is identical.
	limiter := middleware.NewRateLimiter(0.001, 1)
	handler := limiter.Limit(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("CF-Connecting-IP", "203.0.113.7")
	req.Header.Set("X-Forwarded-For", "8.8.8.8")
	req.RemoteAddr = "10.0.0.1:1111"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("request from CF-Connecting-IP 203.0.113.7 should succeed, got %d", rr.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("CF-Connecting-IP", "203.0.113.8")
	req.Header.Set("X-Forwarded-For", "8.8.8.8")
	req.RemoteAddr = "10.0.0.1:2222"
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("request from different CF-Connecting-IP 203.0.113.8 should succeed, got %d", rr.Code)
	}
}

func TestRateLimiter_RemoteAddrFallbackWhenNoHeaders(t *testing.T) {
	// burst=1, rate=0.001. With no proxy headers at all, fall back to
	// RemoteAddr (port stripped) — same host, different port, same bucket.
	limiter := middleware.NewRateLimiter(0.001, 1)
	handler := limiter.Limit(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "9.9.9.9:1000"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("first request should succeed, got %d", rr.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "9.9.9.9:2000"
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("second request from same host different port should be limited, got %d", rr.Code)
	}
}

func TestRateLimiter_DifferentIPsHaveIndependentLimits(t *testing.T) {
	// burst=1, rate=0.001
	limiter := middleware.NewRateLimiter(0.001, 1)
	handler := limiter.Limit(okHandler)

	ips := []string{"1.2.3.4:100", "5.6.7.8:100", "9.10.11.12:100"}

	// Each IP gets its own bucket — first request from each should succeed
	for _, ip := range ips {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = ip
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("IP %s first request should succeed, got %d", ip, rr.Code)
		}
	}

	// Second request from each IP should now be blocked
	for _, ip := range ips {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = ip
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusTooManyRequests {
			t.Errorf("IP %s second request should be blocked, got %d", ip, rr.Code)
		}
	}
}

func TestRateLimiter_CloseStopsCleanupGoroutine(t *testing.T) {
	// This test verifies that Close() stops the cleanup goroutine without hanging.
	// We run it with -race to catch any use-after-close data races.
	limiter := middleware.NewRateLimiter(10, 5)

	// Seed a visitor so the limiter is doing real work.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	rr := httptest.NewRecorder()
	limiter.Limit(okHandler).ServeHTTP(rr, req)

	// Close must return promptly (goroutine receives on stop channel).
	done := make(chan struct{})
	go func() {
		limiter.Close()
		close(done)
	}()

	select {
	case <-done:
		// success
	case <-time.After(time.Second):
		t.Fatal("Close() did not return within 1s — cleanup goroutine may be stuck")
	}
}

func TestRateLimitHeaders(t *testing.T) {
	rl := middleware.NewRateLimiter(1, 1) // 1 req/s, burst 1 — second request must 429
	defer rl.Close()
	h := rl.Limit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.RemoteAddr = "10.1.2.3:555"

	rec1 := httptest.NewRecorder()
	h.ServeHTTP(rec1, req)
	if rec1.Code != http.StatusOK {
		t.Fatalf("first request: %d", rec1.Code)
	}

	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req)
	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("second request: %d, want 429", rec2.Code)
	}
	if ra := rec2.Header().Get("Retry-After"); ra == "" {
		t.Error("missing Retry-After")
	} else if n, err := strconv.Atoi(ra); err != nil || n < 1 {
		t.Errorf("Retry-After = %q, want integer >= 1", ra)
	}
	if rec2.Header().Get("X-RateLimit-Limit") != "1" {
		t.Errorf("X-RateLimit-Limit = %q", rec2.Header().Get("X-RateLimit-Limit"))
	}
	if rec2.Header().Get("X-RateLimit-Remaining") != "0" {
		t.Errorf("X-RateLimit-Remaining = %q", rec2.Header().Get("X-RateLimit-Remaining"))
	}
}

func TestRateLimiter_CleanupStaleVisitors(t *testing.T) {
	limiter := middleware.NewRateLimiter(10, 5)

	// Seed a visitor
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "99.99.99.99:1234"
	rr := httptest.NewRecorder()
	limiter.Limit(okHandler).ServeHTTP(rr, req)

	initialCount := limiter.VisitorCount()
	if initialCount == 0 {
		t.Fatal("expected at least one visitor after a request")
	}

	// Mark the visitor as stale by setting last-seen to 11 minutes ago
	limiter.SetLastSeen("99.99.99.99", time.Now().Add(-11*time.Minute))

	// Run cleanup
	limiter.Cleanup()

	if limiter.VisitorCount() != 0 {
		t.Errorf("expected 0 visitors after cleanup, got %d", limiter.VisitorCount())
	}
}
