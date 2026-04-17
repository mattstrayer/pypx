# Launch Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Harden pypx for a Reddit/Product Hunt launch spike and migrate the droplet to the correct DigitalOcean team at a larger size.

**Architecture:** Four independent code changes (Caddy, Docker Compose, Nuxt, Go API) are deployed to the existing droplet first, then the droplet is snapshotted and recreated in the correct team at 2GB/2vCPU. This ordering ensures the snapshot captures the hardened config so the new droplet is ready immediately.

**Tech Stack:** Caddy 2, Docker Compose, Nuxt 4/TypeScript, Go 1.26, `golang.org/x/sync/singleflight`, doctl CLI

---

## Files Changed

| File | Change |
|------|--------|
| `Caddyfile` | Add `encode gzip zstd` + read/write/idle timeouts in `servers {}` block |
| `docker-compose.yml` | Add CPU limits to all services; bump web memory 256M→512M |
| `web/nuxt.config.ts` | Disable devtools; add `routeRules` with Cache-Control headers |
| `api/internal/handler/packages.go` | Add `singleflight.Group` field; wrap cache-miss fetch path in `sf.Do` |
| `api/go.mod` / `api/go.sum` | Add `golang.org/x/sync` dependency |

---

## Task 1: Caddy — gzip compression + connection timeouts

**Files:**
- Modify: `Caddyfile`

- [ ] **Step 1: Add encode and server timeout directives**

Replace the entire `Caddyfile` with:

```caddy
{
    servers {
        trusted_proxies static 173.245.48.0/20 103.21.244.0/22 103.22.200.0/22 103.31.4.0/22 141.101.64.0/18 108.162.192.0/18 190.93.240.0/20 188.114.96.0/20 197.234.240.0/22 198.41.128.0/17 162.158.0.0/15 104.16.0.0/13 104.24.0.0/14 172.64.0.0/13 131.0.72.0/22 2400:cb00::/32 2606:4700::/32 2803:f800::/32 2405:b500::/32 2405:8100::/32 2a06:98c0::/29 2c0f:f248::/32
        read_timeout 10s
        write_timeout 30s
        idle_timeout 90s
    }
}

{$DOMAIN:localhost} {
    tls internal

    encode gzip zstd

    header {
        # Security headers
        X-Content-Type-Options nosniff
        X-Frame-Options DENY
        Referrer-Policy strict-origin-when-cross-origin
        Strict-Transport-Security "max-age=31536000; includeSubDomains; preload"
        -Server
    }

    handle /api/* {
        reverse_proxy api:8080
    }

    handle {
        reverse_proxy web:3000
    }
}
```

- [ ] **Step 2: Verify syntax locally**

```bash
docker run --rm -v "$PWD/Caddyfile:/etc/caddy/Caddyfile" caddy:2-alpine caddy validate --config /etc/caddy/Caddyfile
```

Expected: `Valid configuration`

- [ ] **Step 3: Commit**

```bash
git add Caddyfile
git commit -m "feat: add gzip compression and connection timeouts to Caddy"
```

---

## Task 2: Docker Compose — CPU limits + Nuxt memory bump

**Files:**
- Modify: `docker-compose.yml`

- [ ] **Step 1: Add CPU limits and bump web memory**

In `docker-compose.yml`, update all three `deploy.resources.limits` blocks:

```yaml
  caddy:
    # ... (existing config unchanged)
    deploy:
      resources:
        limits:
          memory: 128M
          cpus: '0.5'

  api:
    # ... (existing config unchanged)
    deploy:
      resources:
        limits:
          memory: 512M
          cpus: '1.5'

  web:
    # ... (existing config unchanged)
    deploy:
      resources:
        limits:
          memory: 512M      # was 256M
          cpus: '0.75'
```

- [ ] **Step 2: Verify compose file parses**

```bash
docker compose config --quiet
```

Expected: no output (no errors)

- [ ] **Step 3: Commit**

```bash
git add docker-compose.yml
git commit -m "feat: add CPU limits and increase web container memory to 512M"
```

---

## Task 3: Nuxt — disable devtools + add cache headers

**Files:**
- Modify: `web/nuxt.config.ts`

- [ ] **Step 1: Disable devtools and add routeRules**

Replace the `devtools` line and add `routeRules` in `web/nuxt.config.ts`:

```typescript
export default defineNuxtConfig({
  compatibilityDate: '2026-04-09',
  devtools: { enabled: false },

  modules: [
    '@vueuse/nuxt',
    '@nuxtjs/seo',
    '@nuxtjs/color-mode',
  ],

  routeRules: {
    '/packages/**': {
      headers: { 'Cache-Control': 'public, max-age=60, stale-while-revalidate=300' },
    },
    '/': {
      headers: { 'Cache-Control': 'public, max-age=30, stale-while-revalidate=120' },
    },
    '/search': {
      headers: { 'Cache-Control': 'no-store' },
    },
  },

  colorMode: {
    classSuffix: '',
    defaultValue: 'system',
    storageKey: 'pypx-color-mode',
  },

  css: ['~/assets/css/main.css'],

  app: {
    head: {
      link: [
        { rel: 'icon', type: 'image/svg+xml', href: '/favicon.svg' },
      ],
    },
  },

  vite: {
    plugins: [
      import('@tailwindcss/vite').then((m) => m.default()),
    ],
  },

  site: {
    url: 'https://pypx.app',
    name: 'pypx',
    description: 'The Python Package Index, reimagined. Fast search, dependency insights, download trends, and changelogs — all in one place.',
    defaultLocale: 'en',
  },

  ogImage: {
    enabled: true,
  },

  schemaOrg: {
    reactive: true,
  },

  sitemap: {
    sitemaps: {
      popular: {
        sources: ["/api/__sitemap__/urls"],
      },
      cached: {
        sources: ["/api/__sitemap__/urls"],
      },
    },
  },

  robots: {
    allow: '/',
  },

  runtimeConfig: {
    apiBase: process.env.API_BASE || 'http://localhost:8080/api',
    public: {
      apiBase: process.env.NUXT_PUBLIC_API_BASE || '/api',
    },
  },

})
```

- [ ] **Step 2: Verify build still works**

```bash
cd web && npm run build
```

Expected: build completes with no errors

- [ ] **Step 3: Commit**

```bash
git add web/nuxt.config.ts
git commit -m "feat: disable devtools in prod, add cache-control route rules"
```

---

## Task 4: Go API — singleflight deduplication for cache misses

**Files:**
- Modify: `api/internal/handler/packages.go`
- Modify: `api/internal/handler/packages_test.go`
- Modify: `api/go.mod`, `api/go.sum` (via `go get`)

Singleflight collapses concurrent requests for the same package hitting a cache miss into a single upstream PyPI call. All waiting goroutines receive the same result when the one in-flight call completes.

- [ ] **Step 1: Add the golang.org/x/sync dependency**

```bash
cd api && go get golang.org/x/sync@latest
```

Expected: `go.mod` and `go.sum` updated, no errors.

- [ ] **Step 2: Write the failing test**

Add this test to `api/internal/handler/packages_test.go` (inside the existing `package handler_test` — add after the last existing test function). Also add `"sync"` to the existing import block if it isn't already there.

```go
func TestPackageHandler_Get_CacheMissSingleFlight(t *testing.T) {
	// Count how many times the mock PyPI server is called.
	var upstream atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstream.Add(1)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockRequestsResponse)
	}))
	defer srv.Close()

	c, err := cache.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	pypiClient := pypi.NewClient(pypi.WithBaseURL(srv.URL))
	h := handler.NewPackageHandler(pypiClient, c)

	router := chi.NewRouter()
	router.Get("/packages/{name}", h.Get)

	const concurrency = 20
	var wg sync.WaitGroup
	wg.Add(concurrency)
	ready := make(chan struct{})

	for range concurrency {
		go func() {
			defer wg.Done()
			<-ready // all goroutines start at the same time
			req := httptest.NewRequest(http.MethodGet, "/packages/requests", nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Errorf("expected 200, got %d", rec.Code)
			}
		}()
	}

	close(ready) // release all goroutines simultaneously
	wg.Wait()

	if got := upstream.Load(); got > 3 {
		t.Errorf("expected at most 3 upstream PyPI calls (singleflight), got %d", got)
	}
}
```

Also add `"sync"` to the imports in `packages_test.go` if not already present.

- [ ] **Step 3: Run the test to verify it fails**

```bash
cd api && go test ./internal/handler/ -run TestPackageHandler_Get_CacheMissSingleFlight -v -count=1
```

Expected: FAIL — upstream call count will be ~20 (no deduplication yet).

- [ ] **Step 4: Add singleflight to PackageHandler**

In `api/internal/handler/packages.go`, make three changes:

**4a. Add the import:**

```go
import (
    "encoding/json"
    "log"
    "net/http"
    "sort"
    "strings"
    "time"

    "github.com/go-chi/chi/v5"
    "golang.org/x/sync/singleflight"
    "github.com/pypx/api/internal/cache"
    "github.com/pypx/api/internal/enrichment"
    "github.com/pypx/api/internal/markdown"
    "github.com/pypx/api/internal/rst"
    "github.com/pypx/api/internal/pypi"
)
```

**4b. Add the field to `PackageHandler`:**

```go
type PackageHandler struct {
    pypiClient *pypi.Client
    cache      cache.Cacher
    sf         singleflight.Group
}
```

**4c. Replace the cache-miss block in `Get()` (lines 168–192) with:**

```go
    // Cache miss — fetch, enrich, and cache. singleflight collapses concurrent
    // misses for the same package into one upstream call.
    v, err, _ := h.sf.Do(cacheKey, func() (any, error) {
        resp, err := h.fetchPackage(name)
        if err != nil {
            return nil, err
        }
        pkg := buildPackageResponse(resp)
        b, err := json.Marshal(pkg)
        if err != nil {
            return nil, err
        }
        h.cache.Set(cacheKey, b, packageTTL) //nolint:errcheck
        return b, nil
    })
    if err != nil {
        if strings.Contains(err.Error(), "not found") {
            http.Error(w, "package not found", http.StatusNotFound)
            return
        }
        http.Error(w, "failed to fetch package", http.StatusBadGateway)
        return
    }

    encoded := v.([]byte)

    w.Header().Set("Cache-Control", "public, max-age=3600")
    w.Header().Set("Content-Type", "application/json")
    w.Write(encoded) //nolint:errcheck
```

- [ ] **Step 5: Run the test to verify it passes**

```bash
cd api && go test ./internal/handler/ -run TestPackageHandler_Get_CacheMissSingleFlight -v -count=1
```

Expected: PASS — upstream call count should be 1 or 2 (singleflight may allow a second call if the first finishes before all goroutines start, but not 20).

- [ ] **Step 6: Run the full handler test suite to check for regressions**

```bash
cd api && go test ./internal/handler/ -v -count=1
```

Expected: all existing tests pass.

- [ ] **Step 7: Commit**

```bash
cd api
git add go.mod go.sum internal/handler/packages.go internal/handler/packages_test.go
git commit -m "feat: add singleflight deduplication for package cache misses"
```

---

## Task 5: Deploy to existing droplet

Deploy all code changes to the live droplet before snapshotting.

- [ ] **Step 1: Push to main**

```bash
git push origin master
```

- [ ] **Step 2: SSH into the droplet and pull + rebuild**

```bash
ssh root@104.236.205.222
cd /path/to/pypx   # wherever the repo lives on the server
git pull origin master
docker compose build --no-cache api web
docker compose up -d
```

- [ ] **Step 3: Verify services are healthy**

```bash
docker compose ps
curl -s -o /dev/null -w "%{http_code}" http://localhost/api/health
```

Expected: all containers `healthy`, health check returns `200`.

- [ ] **Step 4: Verify gzip is working**

```bash
curl -s -I -H "Accept-Encoding: gzip" https://pypx.app/api/packages/requests | grep -i content-encoding
```

Expected: `content-encoding: gzip`

---

## Task 6: Droplet migration — snapshot + team transfer + resize

This task has ~15 minutes of downtime. Do it during a low-traffic window (not immediately after posting to Reddit/PH).

**Before starting:** Check DNS TTL and lower it to 300s a few hours ahead.

- [ ] **Step 1: Lower DNS TTL (do this 1–2 hours before migration)**

In your DNS provider, find the A record for `pypx.app` and set TTL to 300 seconds.

- [ ] **Step 2: Gracefully stop the stack on the droplet**

```bash
ssh root@104.236.205.222
cd /path/to/pypx
docker compose down
```

- [ ] **Step 3: Power off the droplet**

```bash
# still on the server
shutdown -h now
```

Wait ~30 seconds for the droplet to show as `off` in DigitalOcean.

- [ ] **Step 4: Take a snapshot (from local machine, vandal context)**

```bash
doctl compute droplet-action snapshot 564145426 --snapshot-name pypx-launch-ready --wait
```

`--wait` blocks until the snapshot is complete (~5 minutes for a 25GB disk). Expected output ends with `status: completed`.

- [ ] **Step 5: Get the snapshot ID**

```bash
doctl compute snapshot list --resource-type droplet --format ID,Name,CreatedAt
```

Note the ID of `pypx-launch-ready` — you'll need it in step 7.

- [ ] **Step 6: Switch to the My Team context**

```bash
doctl auth switch --context default
doctl account get   # verify: Team should show "My Team" or your personal account
```

- [ ] **Step 7: Create new droplet in My Team from the snapshot**

```bash
doctl compute droplet create pypx \
  --size s-2vcpu-2gb \
  --region nyc3 \
  --image <snapshot-id-from-step-5> \
  --ssh-keys 45018925,34093466 \
  --wait
```

Note the new droplet's public IP from the output.

- [ ] **Step 8: SSH into the new droplet and verify Docker volumes**

```bash
ssh root@<new-ip>
docker volume ls     # should see caddy_data, caddy_config, api_data
docker compose up -d
docker compose ps    # wait for all containers to be healthy
```

If `docker compose` can't find the compose file, the repo may need to be at the same path — check with `ls /path/to/pypx`.

- [ ] **Step 9: Smoke test the new droplet directly**

```bash
curl -s -o /dev/null -w "%{http_code}" http://<new-ip>/api/health
```

Expected: `200`

- [ ] **Step 10: Update DNS to point to the new IP**

In your DNS provider, update the A record for `pypx.app` to `<new-ip>`.

- [ ] **Step 11: Verify DNS propagated and site works**

```bash
dig +short pypx.app           # should return new IP
curl -s https://pypx.app/api/health
```

Expected: new IP in dig output, `200` from health check.

- [ ] **Step 12: Delete the old droplet from Vandal team**

Switch back to vandal context and delete:

```bash
doctl auth switch --context vandal
doctl compute droplet delete 564145426 --force
```

---

## Success Criteria

- [ ] `curl -sI -H "Accept-Encoding: gzip" https://pypx.app/api/packages/requests | grep content-encoding` returns `gzip`
- [ ] Go test `TestPackageHandler_Get_CacheMissSingleFlight` passes (≤3 upstream calls for 20 concurrent misses)
- [ ] `docker compose ps` shows all containers healthy with new memory/CPU limits
- [ ] `https://pypx.app` loads with devtools absent from response headers
- [ ] DO console shows pypx droplet under My Team, size `s-2vcpu-2gb`
- [ ] Old droplet deleted from Vandal team
