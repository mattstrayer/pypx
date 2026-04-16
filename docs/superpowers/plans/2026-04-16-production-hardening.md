# Production Hardening & Launch Polish

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Harden pypx for public launch by fixing all P0/P1 reliability issues and adding three high-impact UX features.

**Architecture:** Backend hardening focuses on SQLite resilience (busy_timeout, connection pools), external API circuit breakers, rate limiting, and graceful shutdown coordination. Frontend work adds persistent search, keyboard hints, and README outline navigation. Changes are designed to be independently deployable — each task produces a working commit.

**Tech Stack:** Go 1.26 (chi v5, modernc.org/sqlite), Nuxt 4 (Vue 3, Tailwind 4, VueUse), Caddy 2, Docker Compose

---

## File Structure

### New Files
| File | Responsibility |
|---|---|
| `api/internal/middleware/ratelimit.go` | Token-bucket rate limiter middleware for chi |
| `api/internal/circuitbreaker/breaker.go` | Simple circuit breaker wrapping HTTP clients |
| `web/app/components/ReadmeOutline.vue` | Sidebar TOC for description/README content |
| `web/app/composables/useKeyboardShortcuts.ts` | Global keyboard shortcut registry + hint rendering |
| `web/app/components/KbdHint.vue` | Reusable keyboard hint badge component |

### Modified Files
| File | Changes |
|---|---|
| `api/internal/cache/sqlite.go` | Add `busy_timeout`, connection pool limits |
| `api/internal/search/index.go` | Add `busy_timeout`, connection pool limits, atomic FTS5 rebuild |
| `api/internal/cache/memory.go` | Fix race condition in Get() promote path |
| `api/cmd/server/main.go` | Wire rate limiter, add server timeouts, coordinate shutdown |
| `api/internal/handler/changelog.go` | Add stale-cache fallback |
| `api/internal/worker/background.go` | Reuse HTTP client, add retry with backoff |
| `goopy/goopy.go` | Thread context through goroutine pool |
| `goopy/wheel/wheel.go` | Add timeout to default HTTP client |
| `Caddyfile` | Add HSTS header |
| `docker-compose.yml` | Bump Caddy memory limit, add logging config |
| `web/app/components/AppHeader.vue` | Already has persistent search — add keyboard hint styling |
| `web/app/components/PackageOverview.vue` | Add README outline navigation |
| `web/app/pages/packages/[name].vue` | Wire keyboard shortcuts for tabs |
| `web/app/layouts/default.vue` | Wire global keyboard shortcut provider |

---

## Task 1: SQLite Busy Timeout & Connection Pool Limits (P0)

**Files:**
- Modify: `api/internal/cache/sqlite.go:24-51`
- Modify: `api/internal/search/index.go:25-74`

- [ ] **Step 1: Add busy_timeout and pool limits to cache SQLite**

In `api/internal/cache/sqlite.go`, add after the WAL/synchronous PRAGMAs (after line 37):

```go
if _, err := db.Exec(`PRAGMA busy_timeout=5000`); err != nil {
    db.Close()
    return nil, err
}

db.SetMaxOpenConns(25)
db.SetMaxIdleConns(5)
db.SetConnMaxLifetime(5 * time.Minute)
```

- [ ] **Step 2: Add busy_timeout and pool limits to search index SQLite**

In `api/internal/search/index.go`, add after the WAL PRAGMA (after line 35):

```go
if _, err := db.Exec(`PRAGMA busy_timeout=5000`); err != nil {
    db.Close()
    return nil, fmt.Errorf("search: set busy_timeout: %w", err)
}

db.SetMaxOpenConns(25)
db.SetMaxIdleConns(5)
db.SetConnMaxLifetime(5 * time.Minute)
```

- [ ] **Step 3: Run tests**

Run: `cd api && go test ./internal/cache/... ./internal/search/...`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add api/internal/cache/sqlite.go api/internal/search/index.go
git commit -m "fix: add SQLite busy_timeout and connection pool limits"
```

---

## Task 2: Atomic FTS5 Rebuild on Startup (P0)

**Files:**
- Modify: `api/internal/search/index.go:49-72`

The current startup drops and recreates `packages_fts`, leaving a window where search queries fail with "no such table". Fix by building into a temp table and renaming atomically.

- [ ] **Step 1: Replace the FTS5 rebuild block**

Replace lines 49-72 in `api/internal/search/index.go` (from `// Rebuild the FTS5 table` through the closing brace of the populate exec) with:

```go
// Build FTS5 in a temporary table, then swap atomically to avoid
// search downtime during restarts.
db.Exec(`DROP TABLE IF EXISTS packages_fts_new`) //nolint:errcheck

if _, err := db.Exec(`
    CREATE VIRTUAL TABLE packages_fts_new USING fts5(
        name,
        summary,
        downloads UNINDEXED,
        tokenize='porter unicode61'
    )
`); err != nil {
    db.Close()
    return nil, fmt.Errorf("search: create fts_new table: %w", err)
}

if _, err := db.Exec(`
    INSERT INTO packages_fts_new (name, summary, downloads)
    SELECT name, summary, downloads FROM packages_meta
`); err != nil {
    db.Close()
    return nil, fmt.Errorf("search: populate fts_new from meta: %w", err)
}

// Atomic swap: drop old, rename new.
db.Exec(`DROP TABLE IF EXISTS packages_fts`) //nolint:errcheck
if _, err := db.Exec(`ALTER TABLE packages_fts_new RENAME TO packages_fts`); err != nil {
    db.Close()
    return nil, fmt.Errorf("search: rename fts table: %w", err)
}
```

- [ ] **Step 2: Run tests**

Run: `cd api && go test ./internal/search/...`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add api/internal/search/index.go
git commit -m "fix: atomic FTS5 rebuild to prevent search downtime on restart"
```

---

## Task 3: HSTS Header & Caddy Memory Bump (P0 + P1)

**Files:**
- Modify: `Caddyfile:10-16`
- Modify: `docker-compose.yml:17-20`

- [ ] **Step 1: Add HSTS to Caddyfile**

In `Caddyfile`, add after the `Referrer-Policy` line (line 14):

```
Strict-Transport-Security "max-age=31536000; includeSubDomains; preload"
```

- [ ] **Step 2: Bump Caddy memory limit**

In `docker-compose.yml`, change line 20 from `memory: 128M` to:

```yaml
          memory: 256M
```

- [ ] **Step 3: Add Docker logging config**

In `docker-compose.yml`, add a logging block to the `api` service (after `restart: unless-stopped`, line 34):

```yaml
    logging:
      driver: "json-file"
      options:
        max-size: "50m"
        max-file: "3"
```

Add the same logging block to the `web` and `caddy` services.

- [ ] **Step 4: Commit**

```bash
git add Caddyfile docker-compose.yml
git commit -m "fix: add HSTS header, bump Caddy memory, add log rotation"
```

---

## Task 4: Rate Limiting Middleware (P0)

**Files:**
- Create: `api/internal/middleware/ratelimit.go`
- Modify: `api/cmd/server/main.go:90-101`

- [ ] **Step 1: Create the rate limiter**

Create `api/internal/middleware/ratelimit.go`:

```go
package middleware

import (
	"net/http"
	"sync"
	"time"
)

// visitor tracks request timestamps for a single IP.
type visitor struct {
	tokens   float64
	lastSeen time.Time
}

// RateLimiter is a per-IP token bucket rate limiter.
type RateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	rate     float64 // tokens per second
	burst    int     // max tokens
}

// NewRateLimiter creates a rate limiter. rate is requests/second, burst is the
// max burst size.
func NewRateLimiter(rate float64, burst int) *RateLimiter {
	rl := &RateLimiter{
		visitors: make(map[string]*visitor),
		rate:     rate,
		burst:    burst,
	}
	go rl.cleanup()
	return rl
}

func (rl *RateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	v, exists := rl.visitors[ip]
	if !exists {
		rl.visitors[ip] = &visitor{tokens: float64(rl.burst) - 1, lastSeen: time.Now()}
		return true
	}

	elapsed := time.Since(v.lastSeen).Seconds()
	v.lastSeen = time.Now()
	v.tokens += elapsed * rl.rate
	if v.tokens > float64(rl.burst) {
		v.tokens = float64(rl.burst)
	}

	if v.tokens < 1 {
		return false
	}
	v.tokens--
	return true
}

func (rl *RateLimiter) cleanup() {
	for {
		time.Sleep(5 * time.Minute)
		rl.mu.Lock()
		for ip, v := range rl.visitors {
			if time.Since(v.lastSeen) > 10*time.Minute {
				delete(rl.visitors, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// Limit returns middleware that rate-limits requests per IP.
func (rl *RateLimiter) Limit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr
		// Use X-Forwarded-For if behind proxy (Caddy/Cloudflare).
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			ip = xff
		}
		if !rl.allow(ip) {
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}
```

- [ ] **Step 2: Wire into main.go**

In `api/cmd/server/main.go`, add the import:

```go
mw "github.com/pypx/api/internal/middleware"
```

After the CORS middleware (line 100), add:

```go
// Rate limit: 30 req/s per IP with burst of 60.
limiter := mw.NewRateLimiter(30, 60)
r.Use(limiter.Limit)
```

- [ ] **Step 3: Add server timeout configuration**

In `api/cmd/server/main.go`, update the server config (lines 122-125) to:

```go
srv := &http.Server{
    Addr:           ":" + port,
    Handler:        r,
    ReadTimeout:    15 * time.Second,
    WriteTimeout:   60 * time.Second, // 60s to cover docs endpoint
    MaxHeaderBytes: 1 << 20, // 1 MB
}
```

- [ ] **Step 4: Run tests**

Run: `cd api && go build ./cmd/server`
Expected: BUILD SUCCESS

- [ ] **Step 5: Commit**

```bash
git add api/internal/middleware/ratelimit.go api/cmd/server/main.go
git commit -m "feat: add per-IP rate limiting and server timeout hardening"
```

---

## Task 5: Memory Cache Race Condition Fix (P1)

**Files:**
- Modify: `api/internal/cache/memory.go:32-56`

The current Get() releases the read lock, reads SQLite, then re-acquires a write lock to promote. Between unlock and lock, another goroutine can write, causing a race. Fix by holding the write lock for the promote operation using double-check locking.

- [ ] **Step 1: Fix the race condition**

Replace the `Get` method in `api/internal/cache/memory.go` (lines 32-56) with:

```go
// Get checks memory first, then falls through to SQLite.
// If found in SQLite but not memory, promotes to memory.
func (mc *MemoryCache) Get(key string, ttl time.Duration) (data []byte, fresh bool, err error) {
	mc.mu.RLock()
	item, ok := mc.items[key]
	mc.mu.RUnlock()

	if ok {
		age := time.Since(item.createdAt)
		return item.data, age < ttl, nil
	}

	// Fall through to SQLite.
	data, fresh, err = mc.sqlite.Get(key, ttl)
	if err != nil || data == nil {
		return data, fresh, err
	}

	// Promote to memory cache with double-check to avoid overwriting
	// a fresher write that happened between RUnlock and Lock.
	mc.mu.Lock()
	if _, exists := mc.items[key]; !exists {
		if len(mc.items) >= mc.maxSize {
			mc.evictOldest()
		}
		mc.items[key] = &memItem{data: data, createdAt: time.Now()}
	}
	mc.mu.Unlock()

	return data, fresh, nil
}
```

- [ ] **Step 2: Run tests**

Run: `cd api && go test ./internal/cache/... -race`
Expected: PASS (no race detected)

- [ ] **Step 3: Commit**

```bash
git add api/internal/cache/memory.go
git commit -m "fix: memory cache race condition in Get() promote path"
```

---

## Task 6: Changelog Stale-Cache Fallback (P1)

**Files:**
- Modify: `api/internal/handler/changelog.go:55-73`

The changelog handler returns 502 when PyPI is down, unlike security/stats/packages which serve stale cache. Add the same stale-while-revalidate pattern.

- [ ] **Step 1: Add stale fallback**

Replace lines 55-73 of `api/internal/handler/changelog.go` with:

```go
	cacheKey := "changelog:" + strings.ToLower(name)

	// Serve from cache if fresh.
	if data, fresh, err := h.cache.Get(cacheKey, changelogTTL); err == nil && data != nil {
		if fresh {
			w.Header().Set("Cache-Control", "public, max-age=604800")
			w.Header().Set("Content-Type", "application/json")
			w.Write(data) //nolint:errcheck
			return
		}
		// Stale data exists — serve it immediately and let the next request refresh.
		w.Header().Set("Cache-Control", "public, max-age=60")
		w.Header().Set("Content-Type", "application/json")
		w.Write(data) //nolint:errcheck
		return
	}

	// Fetch PyPI package info to get project URLs.
	pypiResp, err := h.pkg.FetchPackage(name)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, "package not found", http.StatusNotFound)
			return
		}
		// Last resort: try serving any cached data regardless of TTL.
		if data, _, cacheErr := h.cache.Get(cacheKey, 0); cacheErr == nil && data != nil {
			w.Header().Set("Cache-Control", "public, max-age=60")
			w.Header().Set("Content-Type", "application/json")
			w.Write(data) //nolint:errcheck
			return
		}
		http.Error(w, "failed to fetch package", http.StatusBadGateway)
		return
	}
```

- [ ] **Step 2: Run tests**

Run: `cd api && go build ./cmd/server`
Expected: BUILD SUCCESS

- [ ] **Step 3: Commit**

```bash
git add api/internal/handler/changelog.go
git commit -m "fix: add stale-cache fallback to changelog handler"
```

---

## Task 7: Worker HTTP Client Reuse (P1)

**Files:**
- Modify: `api/internal/worker/background.go:23-49,53-64,117-128`

The worker creates a new `http.Client` on every sync call, defeating connection pooling. Store a shared client on the Worker struct.

- [ ] **Step 1: Add HTTP client to Worker struct**

In `api/internal/worker/background.go`, update the Worker struct (lines 24-29) to:

```go
// Worker periodically syncs the PyPI Simple API index into the search index.
type Worker struct {
	pypi       *pypi.Client
	cache      cache.Cacher
	index      *search.Index
	httpClient *http.Client
	config     Config
}
```

Update the `New` function (after line 43, before the return) to include:

```go
return &Worker{
    pypi:       pypiClient,
    cache:      c,
    index:      idx,
    httpClient: &http.Client{Timeout: 5 * time.Minute},
    config:     cfg,
}
```

- [ ] **Step 2: Replace inline client creation in SyncIndex**

In `SyncIndex`, replace line 63 (`client := &http.Client{Timeout: 5 * time.Minute}`) and line 64 (`resp, err := client.Do(req)`) with:

```go
resp, err := w.httpClient.Do(req)
```

- [ ] **Step 3: Replace inline client creation in SyncDownloads**

In `SyncDownloads`, replace line 128 (`client := &http.Client{Timeout: 2 * time.Minute}`) and line 129 (`resp, err := client.Do(req)`) with:

```go
resp, err := w.httpClient.Do(req)
```

- [ ] **Step 4: Run tests**

Run: `cd api && go build ./cmd/server`
Expected: BUILD SUCCESS

- [ ] **Step 5: Commit**

```bash
git add api/internal/worker/background.go
git commit -m "fix: reuse HTTP client in background worker for connection pooling"
```

---

## Task 8: Goopy Context Cancellation (P1)

**Files:**
- Modify: `goopy/goopy.go:32-100`
- Modify: `goopy/wheel/wheel.go:44-51`

The goroutine pool in `ExtractPackage` ignores context — if a request times out, workers keep parsing. Thread context through.

- [ ] **Step 1: Add context parameter to ExtractPackage**

Update the `ExtractPackage` signature (line 35) to accept context:

```go
func ExtractPackage(ctx context.Context, name string, files map[string][]byte, topLevelPkgs []string) *model.Package {
```

Update the small-package fast path (lines 54-63) to check context:

```go
// For small packages, skip goroutine overhead.
if len(items) <= 4 {
    pkg := &model.Package{Name: name}
    for _, item := range items {
        if ctx.Err() != nil {
            break
        }
        mod, _ := ExtractModule(item.modName, item.src)
        if hasContent(mod) {
            pkg.Modules = append(pkg.Modules, mod)
        }
    }
    return pkg
}
```

Update the parallel worker loop (lines 80-90) to check context:

```go
wg.Add(workers)
for range workers {
    go func() {
        defer wg.Done()
        defer func() { recover() }()
        for idx := range ch {
            if ctx.Err() != nil {
                return
            }
            mod, _ := ExtractModule(items[idx].modName, items[idx].src)
            results[idx] = mod
        }
    }()
}
```

- [ ] **Step 2: Update callers**

In `ExtractFromWheel` (line 124), pass context:

```go
pkg := ExtractPackage(ctx, name, contents.Files, contents.TopLevelPkgs)
```

- [ ] **Step 3: Add timeout to default wheel HTTP client**

In `goopy/wheel/wheel.go`, update `NewSource` (lines 45-51) to:

```go
func NewSource() *Source {
	return &Source{
		HTTPClient: &http.Client{Timeout: 60 * time.Second},
		MaxSize:    DefaultMaxSize,
		BaseURL:    pypiBaseURL,
	}
}
```

Add `"time"` to the import block.

- [ ] **Step 4: Run tests**

Run: `cd api && go test ./... && cd ../goopy && go test ./...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add goopy/goopy.go goopy/wheel/wheel.go
git commit -m "fix: thread context through goopy for proper cancellation on timeout"
```

---

## Task 9: Graceful Shutdown Coordination (P1)

**Files:**
- Modify: `api/cmd/server/main.go:86-88,138-148`

Currently the worker context is cancelled *after* the HTTP server shutdown completes (via defer). The worker should stop first so in-flight DB writes finish before the server drains.

- [ ] **Step 1: Reorder shutdown sequence**

Replace lines 138-148 of `api/cmd/server/main.go` with:

```go
	<-quit
	log.Println("shutting down server...")

	// Stop background worker first so in-flight DB writes complete.
	workerCancel()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("forced shutdown: %v", err)
	}

	log.Println("server stopped")
```

And remove the `defer workerCancel()` from line 87 (since we now call it explicitly).

- [ ] **Step 2: Run build**

Run: `cd api && go build ./cmd/server`
Expected: BUILD SUCCESS

- [ ] **Step 3: Commit**

```bash
git add api/cmd/server/main.go
git commit -m "fix: coordinate worker shutdown before HTTP server drain"
```

---

## Task 10: Persistent Header Search Bar (Feature)

**Files:**
- Modify: `web/app/components/AppHeader.vue`

The header already has a search bar with typeahead. Reviewing the code, AppHeader.vue already contains a persistent search input on every page (lines 32-75). This is already implemented.

The remaining work is to ensure the search bar is visible on mobile (currently `max-w-md` may compress too much) and that it has proper accessibility labels.

- [ ] **Step 1: Add ARIA labels to search input**

In `web/app/components/AppHeader.vue`, update the `<input>` (line 48) to add an accessible label:

```html
            <input
              v-model="query"
              type="text"
              aria-label="Search Python packages"
              placeholder="Search packages..."
              class="w-full rounded-md border border-zinc-800 bg-zinc-900 py-1.5 pl-8 pr-12 text-sm text-zinc-50 placeholder-zinc-500 outline-none focus:border-[var(--color-brand-light)] focus:ring-1 focus:ring-[var(--color-brand-border)]"
              @keydown="onKeydown"
              @focus="query.trim() && (isOpen = true)"
            />
```

- [ ] **Step 2: Add ARIA labels to search dropdown buttons**

In `web/app/components/SearchDropdown.vue`, add `role="listbox"` to the container div (line 18) and `role="option"` + `aria-selected` to each result button (line 29):

Update the container div:
```html
  <div
    v-if="hasQuery"
    role="listbox"
    aria-label="Search results"
    class="overflow-hidden rounded-lg border border-zinc-800 bg-zinc-900/95 shadow-xl backdrop-blur"
  >
```

Update the button:
```html
      <button
        v-for="(result, index) in results"
        :key="result.name"
        role="option"
        :aria-selected="index === selectedIndex"
        class="cursor-pointer flex w-full items-center gap-2 px-3 py-2 text-left transition-colors"
```

- [ ] **Step 3: Commit**

```bash
git add web/app/components/AppHeader.vue web/app/components/SearchDropdown.vue
git commit -m "fix: add ARIA labels to search components for accessibility"
```

---

## Task 11: Keyboard Shortcut Hints (Feature)

**Files:**
- Create: `web/app/components/KbdHint.vue`
- Create: `web/app/composables/useKeyboardShortcuts.ts`
- Modify: `web/app/pages/packages/[name].vue:92-107`
- Modify: `web/app/layouts/default.vue`

- [ ] **Step 1: Create the KbdHint component**

Create `web/app/components/KbdHint.vue`:

```vue
<script setup lang="ts">
defineProps<{
  keys: string
}>()
</script>

<template>
  <kbd
    class="pointer-events-none ml-1 hidden rounded bg-zinc-800 px-1 py-0.5 font-mono text-[10px] text-zinc-600 group-hover:text-zinc-400 sm:inline-block"
  >
    {{ keys }}
  </kbd>
</template>
```

- [ ] **Step 2: Create the keyboard shortcuts composable**

Create `web/app/composables/useKeyboardShortcuts.ts`:

```typescript
type ShortcutHandler = () => void

const shortcuts = new Map<string, ShortcutHandler>()
let initialized = false

function handleKeydown(e: KeyboardEvent) {
  // Ignore when typing in inputs.
  const tag = (e.target as HTMLElement)?.tagName
  if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return

  const handler = shortcuts.get(e.key)
  if (handler) {
    e.preventDefault()
    handler()
  }
}

export function useKeyboardShortcuts() {
  function register(key: string, handler: ShortcutHandler) {
    shortcuts.set(key, handler)
  }

  function unregister(key: string) {
    shortcuts.delete(key)
  }

  if (import.meta.client && !initialized) {
    initialized = true
    window.addEventListener('keydown', handleKeydown)
  }

  return { register, unregister }
}
```

- [ ] **Step 3: Add keyboard hints to package page tabs**

In `web/app/pages/packages/[name].vue`, update the `inPageTabs` definition (lines 35-39) to include shortcut keys:

```typescript
const inPageTabs = [
  { key: "overview", label: "Overview", shortcut: "1" },
  { key: "dependencies", label: "Dependencies", shortcut: "2" },
  { key: "versions", label: "Versions", shortcut: "3" },
  { key: "stats", label: "Stats", shortcut: "4" },
];
```

Add the keyboard shortcut registration in the `<script setup>` block (after `inPageTabs`):

```typescript
const { register, unregister } = useKeyboardShortcuts();

onMounted(() => {
  for (const tab of inPageTabs) {
    register(tab.shortcut, () => (activeTab.value = tab.key));
  }
  register("/", () => {
    document.querySelector<HTMLInputElement>('header input[type="text"]')?.focus();
  });
});

onUnmounted(() => {
  for (const tab of inPageTabs) {
    unregister(tab.shortcut);
  }
  unregister("/");
});
```

Update the tab button template (lines 93-101) to include the hint:

```html
        <button
          v-for="tab in inPageTabs"
          :key="tab.key"
          class="group cursor-pointer whitespace-nowrap rounded-t px-4 py-2 text-sm font-medium transition-colors"
          :class="
            activeTab === tab.key ? 'bg-zinc-800 text-zinc-50' : 'text-zinc-500 hover:text-zinc-300'
          "
          @click="activeTab = tab.key"
        >
          {{ tab.label }}
          <KbdHint :keys="tab.shortcut" />
        </button>
```

- [ ] **Step 4: Add "/" shortcut hint to header search**

In `web/app/components/AppHeader.vue`, update the kbd element (lines 56-60) to also show "/" when focused outside:

Replace the existing `<kbd>` block:
```html
            <kbd
              class="pointer-events-none absolute right-2 top-1/2 -translate-y-1/2 hidden rounded bg-zinc-800 px-1.5 py-0.5 font-mono text-[10px] text-zinc-500 sm:inline"
            >
              /
            </kbd>
```

- [ ] **Step 5: Commit**

```bash
git add web/app/components/KbdHint.vue web/app/composables/useKeyboardShortcuts.ts web/app/pages/packages/[name].vue web/app/components/AppHeader.vue
git commit -m "feat: add keyboard shortcut hints to tabs and search"
```

---

## Task 12: README with Outline Navigation (Feature)

**Files:**
- Create: `web/app/components/ReadmeOutline.vue`
- Modify: `web/app/components/PackageOverview.vue:25-34`

The description/README HTML is rendered via `v-html` in PackageOverview. We need to parse headings from the rendered HTML and show a sticky outline sidebar.

- [ ] **Step 1: Create the ReadmeOutline component**

Create `web/app/components/ReadmeOutline.vue`:

```vue
<script setup lang="ts">
interface OutlineItem {
  id: string
  text: string
  level: number
}

const props = defineProps<{
  containerSelector: string
}>()

const items = ref<OutlineItem[]>([])
const activeId = ref('')

function buildOutline() {
  const container = document.querySelector(props.containerSelector)
  if (!container) return

  const headings = container.querySelectorAll('h1, h2, h3, h4')
  const outline: OutlineItem[] = []

  headings.forEach((heading, index) => {
    const el = heading as HTMLElement
    // Ensure each heading has an id for scroll targeting.
    if (!el.id) {
      el.id = `readme-heading-${index}`
    }
    outline.push({
      id: el.id,
      text: el.textContent?.trim() || '',
      level: parseInt(el.tagName[1]),
    })
  })

  items.value = outline
}

function scrollTo(id: string) {
  const el = document.getElementById(id)
  if (el) {
    el.scrollIntoView({ behavior: 'smooth', block: 'start' })
    activeId.value = id
  }
}

// Track active heading on scroll.
function onScroll() {
  const headings = items.value
    .map((item) => ({
      id: item.id,
      top: document.getElementById(item.id)?.getBoundingClientRect().top ?? Infinity,
    }))
    .filter((h) => h.top <= 120)

  if (headings.length > 0) {
    activeId.value = headings[headings.length - 1].id
  }
}

onMounted(() => {
  // Wait for v-html to render.
  nextTick(() => {
    buildOutline()
    window.addEventListener('scroll', onScroll, { passive: true })
  })
})

onUnmounted(() => {
  window.removeEventListener('scroll', onScroll)
})

// Rebuild if container content changes.
watch(() => props.containerSelector, () => nextTick(buildOutline))
</script>

<template>
  <nav v-if="items.length >= 3" class="hidden xl:block">
    <h3 class="mb-2 text-xs font-semibold uppercase tracking-wider text-zinc-500">On this page</h3>
    <ul class="space-y-1 border-l border-zinc-800">
      <li v-for="item in items" :key="item.id">
        <button
          class="block w-full truncate border-l-2 py-0.5 text-left text-xs transition-colors"
          :class="[
            activeId === item.id
              ? 'border-[var(--color-brand)] text-zinc-200'
              : 'border-transparent text-zinc-500 hover:text-zinc-300',
            item.level <= 2 ? 'pl-3' : 'pl-5',
          ]"
          @click="scrollTo(item.id)"
        >
          {{ item.text }}
        </button>
      </li>
    </ul>
  </nav>
</template>
```

- [ ] **Step 2: Integrate into PackageOverview**

In `web/app/components/PackageOverview.vue`, update the description section (lines 25-34) to add an id and wrap with the outline:

Replace the description block with:

```html
      <!-- Description -->
      <div
        v-if="pkg.description"
        class="overflow-hidden rounded-lg border border-zinc-800 bg-zinc-900/50 p-5"
      >
        <div class="flex items-start justify-between gap-2 mb-3">
          <h2 class="text-sm font-semibold uppercase tracking-wider text-zinc-500">
            Description
          </h2>
        </div>
        <div class="flex gap-6">
          <div class="min-w-0 flex-1">
            <div
              v-if="pkg.description_html"
              id="readme-content"
              class="prose prose-invert prose-sm max-w-none"
              v-html="pkg.description_html"
            />
            <div v-else class="whitespace-pre-wrap break-words text-sm leading-relaxed text-zinc-300">
              {{ pkg.description }}
            </div>
          </div>
          <div v-if="pkg.description_html" class="sticky top-20 w-48 shrink-0 self-start">
            <ReadmeOutline container-selector="#readme-content" />
          </div>
        </div>
      </div>
```

- [ ] **Step 3: Run dev server and verify**

Run: `cd web && npm run dev`

Verify:
1. Navigate to a package with a long README (e.g., `/packages/requests`)
2. Confirm the outline appears on the right side of the description on xl screens
3. Confirm clicking an outline item scrolls to the heading
4. Confirm the active heading highlights on scroll
5. Confirm the outline hides on smaller screens (< xl breakpoint)

- [ ] **Step 4: Commit**

```bash
git add web/app/components/ReadmeOutline.vue web/app/components/PackageOverview.vue
git commit -m "feat: add README outline navigation sidebar for long descriptions"
```

---

## Task 13: Circuit Breaker for External APIs (P0)

**Files:**
- Create: `api/internal/circuitbreaker/breaker.go`
- Modify: `api/internal/pypi/client.go`
- Modify: `api/internal/github/client.go`

This task adds a simple circuit breaker that trips after N consecutive failures and stays open for a cooldown period before allowing a probe request.

- [ ] **Step 1: Create the circuit breaker**

Create `api/internal/circuitbreaker/breaker.go`:

```go
package circuitbreaker

import (
	"errors"
	"sync"
	"time"
)

// ErrOpen is returned when the circuit breaker is open.
var ErrOpen = errors.New("circuit breaker is open")

// State represents the circuit breaker state.
type State int

const (
	Closed   State = iota // Normal — requests pass through.
	Open                  // Tripped — requests are rejected.
	HalfOpen              // Probing — one request allowed to test recovery.
)

// Breaker is a simple circuit breaker.
type Breaker struct {
	mu           sync.Mutex
	state        State
	failures     int
	threshold    int           // consecutive failures to trip
	cooldown     time.Duration // how long to stay open
	lastFailedAt time.Time
}

// New creates a circuit breaker that trips after threshold consecutive failures
// and stays open for cooldown before allowing a probe.
func New(threshold int, cooldown time.Duration) *Breaker {
	return &Breaker{
		threshold: threshold,
		cooldown:  cooldown,
	}
}

// Allow returns true if the request should proceed.
// Returns ErrOpen if the circuit is open.
func (b *Breaker) Allow() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	switch b.state {
	case Closed:
		return nil
	case Open:
		if time.Since(b.lastFailedAt) > b.cooldown {
			b.state = HalfOpen
			return nil
		}
		return ErrOpen
	case HalfOpen:
		// Only one probe at a time — reject others.
		return ErrOpen
	}
	return nil
}

// RecordSuccess resets the breaker to closed state.
func (b *Breaker) RecordSuccess() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures = 0
	b.state = Closed
}

// RecordFailure increments the failure counter and trips if threshold met.
func (b *Breaker) RecordFailure() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures++
	b.lastFailedAt = time.Now()
	if b.failures >= b.threshold {
		b.state = Open
	}
}
```

- [ ] **Step 2: Integrate with PyPI client**

In `api/internal/pypi/client.go`, add the circuit breaker to the Client struct. Add to imports:

```go
"github.com/pypx/api/internal/circuitbreaker"
```

Add to the Client struct:

```go
type Client struct {
    baseURL    string
    httpClient *http.Client
    breaker    *circuitbreaker.Breaker
}
```

In `NewClient`, initialize the breaker:

```go
c := &Client{
    baseURL:    "https://pypi.org",
    httpClient: &http.Client{Timeout: 15 * time.Second},
    breaker:    circuitbreaker.New(5, 30*time.Second),
}
```

In `FetchPackage`, add the breaker check at the start of the method (before the HTTP request):

```go
if err := c.breaker.Allow(); err != nil {
    return nil, fmt.Errorf("pypi: %w", err)
}
```

After a successful response, call `c.breaker.RecordSuccess()`. On error, call `c.breaker.RecordFailure()`.

- [ ] **Step 3: Integrate with GitHub client**

Apply the same pattern to `api/internal/github/client.go` — add a breaker field, initialize in `NewClient`, and wrap the `get` helper method with Allow/RecordSuccess/RecordFailure.

- [ ] **Step 4: Run tests**

Run: `cd api && go test ./...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add api/internal/circuitbreaker/breaker.go api/internal/pypi/client.go api/internal/github/client.go
git commit -m "feat: add circuit breaker to PyPI and GitHub API clients"
```

---

## Verification Checklist

After all tasks are complete, verify:

- [ ] `cd api && go test ./... -race` — all tests pass with race detector
- [ ] `cd api && go build ./cmd/server` — clean build
- [ ] `cd web && npm run build` — frontend builds without errors
- [ ] `docker compose build` — all containers build
- [ ] Manual test: visit a package page, confirm search works, tabs switch with keyboard shortcuts, README outline appears
- [ ] Manual test: confirm rate limiter returns 429 when hammered (`for i in {1..100}; do curl -s -o /dev/null -w "%{http_code}\n" http://localhost:8080/api/health; done`)
