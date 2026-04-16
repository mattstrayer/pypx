// Package middleware provides HTTP middleware for the chi router.
package middleware

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const staleAfter = 10 * time.Minute

// visitor holds the rate limiter and last-seen time for a single IP.
type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// RateLimiter is a per-IP token bucket rate limiter middleware.
type RateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	rate     rate.Limit // tokens per second
	burst    int
	stop     chan struct{}
}

// NewRateLimiter creates a RateLimiter where each IP gets a token bucket
// with the given sustained rate (tokens/second) and burst size.
// A background goroutine automatically purges stale entries every minute.
func NewRateLimiter(r float64, burst int) *RateLimiter {
	rl := &RateLimiter{
		visitors: make(map[string]*visitor),
		rate:     rate.Limit(r),
		burst:    burst,
		stop:     make(chan struct{}),
	}
	go rl.cleanupLoop()
	return rl
}

// Limit is the chi-compatible middleware handler.
func (rl *RateLimiter) Limit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := extractIP(r)
		if !rl.allow(ip) {
			http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// allow checks the token bucket for ip and records the visit time.
func (rl *RateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	v, ok := rl.visitors[ip]
	if !ok {
		v = &visitor{limiter: rate.NewLimiter(rl.rate, rl.burst)}
		rl.visitors[ip] = v
	}
	v.lastSeen = time.Now()
	lim := v.limiter
	rl.mu.Unlock()

	return lim.Allow()
}

// extractIP returns the real client IP from X-Forwarded-For (first entry)
// or falls back to RemoteAddr (stripping the port).
func extractIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// X-Forwarded-For: client, proxy1, proxy2 — take leftmost
		if idx := strings.IndexByte(xff, ','); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}
	// RemoteAddr is host:port
	addr := r.RemoteAddr
	if idx := strings.LastIndexByte(addr, ':'); idx != -1 {
		return addr[:idx]
	}
	return addr
}

// Close stops the cleanup goroutine started by NewRateLimiter.
func (rl *RateLimiter) Close() {
	close(rl.stop)
}

// cleanupLoop periodically removes visitors that haven't been seen recently.
func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			rl.Cleanup()
		case <-rl.stop:
			return
		}
	}
}

// Cleanup removes visitors that haven't been seen in the last staleAfter window.
// Exported so tests can trigger it directly.
func (rl *RateLimiter) Cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	cutoff := time.Now().Add(-staleAfter)
	for ip, v := range rl.visitors {
		if v.lastSeen.Before(cutoff) {
			delete(rl.visitors, ip)
		}
	}
}

// VisitorCount returns the current number of tracked IPs. Used in tests.
func (rl *RateLimiter) VisitorCount() int {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	return len(rl.visitors)
}

// SetLastSeen overrides the last-seen time for an IP. Used in tests.
func (rl *RateLimiter) SetLastSeen(ip string, t time.Time) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	if v, ok := rl.visitors[ip]; ok {
		v.lastSeen = t
	}
}
