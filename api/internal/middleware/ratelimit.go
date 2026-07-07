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

// extractIP returns the client IP for rate-limit bucketing.
//
// Precedence (matches the Cloudflare → Caddy → API deploy topology):
//  1. CF-Connecting-IP — set authoritatively by Cloudflare; a client cannot
//     spoof it through Cloudflare because Cloudflare overwrites it.
//  2. Rightmost X-Forwarded-For entry — appended by Caddy (the nearest
//     trusted proxy), so it is the immediate TCP peer, not client-supplied.
//     The LEFTMOST entry is fully client-controlled and must never be used:
//     Caddy trusts Cloudflare (Caddyfile trusted_proxies) and both append
//     to, rather than replace, a client-supplied XFF chain.
//  3. RemoteAddr (port stripped) — direct connections in local dev.
func extractIP(r *http.Request) string {
	if cf := strings.TrimSpace(r.Header.Get("CF-Connecting-IP")); cf != "" {
		return cf
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Rightmost entry — appended by the nearest trusted proxy.
		if idx := strings.LastIndexByte(xff, ','); idx != -1 {
			return strings.TrimSpace(xff[idx+1:])
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
