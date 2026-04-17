# pypx Launch Hardening Design

**Date:** 2026-04-17  
**Context:** Pre-launch hardening ahead of Reddit/Product Hunt publish  
**Droplet:** 1GB RAM / 1 vCPU (nyc3, Vandal team) → 2GB RAM / 2 vCPU (nyc3, My Team)

---

## Problem Statement

pypx runs on a single 1GB/1vCPU DigitalOcean droplet. A Reddit or Product Hunt launch routinely sends 500–2,000 concurrent visitors in the first hour. Without hardening, the three most likely failure modes are:

1. **Cache stampede** — 50 users hit the same package page on cache miss, spawning 50 parallel PyPI fetches
2. **OOMKill** — Nuxt web container (256M limit) is killed by Docker mid-request under SSR load
3. **Caddy bandwidth saturation** — uncompressed JSON/HTML responses consume 5–10x more bandwidth and keep connections open longer

The droplet is also in the wrong DigitalOcean team (Vandal) and needs to be moved to My Team.

---

## Scope

**In scope:**
- Caddy: gzip compression, rate limiting, connection/idle timeouts
- Go API: `singleflight` deduplication for package fetches, Docker CPU limits
- Nuxt: disable devtools, bump memory limit, add `routeRules` cache headers
- Droplet: resize to 2GB/2vCPU, transfer to My Team via snapshot

**Out of scope:**
- Parallelizing stats API calls (medium priority, not crash-causing)
- Full ISR/edge caching for Nuxt (future work)
- Redis integration (Redis droplet is in nyc1; latency cost not worth it for this use case)
- Horizontal scaling (overkill for current traffic levels)

---

## Architecture Changes

### 1. Caddy Hardening

**File:** `Caddyfile`

Add three things:

```
encode gzip zstd          # compress all responses
rate_limit {ip} 30r/s     # per-IP rate limit at proxy layer (belt + suspenders with API limiter)
servers {
  read_timeout 10s        # kill slow-reading clients
  write_timeout 30s
  idle_timeout 90s        # don't hold idle connections forever
  max_header_size 1mb
}
```

**Why:** Gzip alone reduces package API response sizes by ~70%. On a 1vCPU machine, smaller payloads mean shorter time-to-complete, fewer concurrent goroutines, less memory pressure. Rate limiting at Caddy is faster than the Go middleware and kills bad traffic before it reaches the app.

**Tradeoff:** `rate_limit` requires the Caddy `rate_limit` module — it's not built-in. We'll use the `francoispqt/caddy-rate-limit` module (already widely used). Alternatively, use Caddy's built-in `request_body` limiter as a simpler fallback.

> **Decision:** Use the global `servers` block for connection-level limits (built-in, no module needed) and keep the existing Go middleware rate limiter. Skip the Caddy rate_limit module to avoid rebuilding the Caddy binary.

So the actual Caddy change is just:
- `encode gzip` (built-in, one line)
- `servers` block with read/write/idle timeouts

### 2. Go API: `singleflight` for Package Fetches

**File:** `api/internal/handler/packages.go`

Wrap the "fetch + enrich" call with `golang.org/x/sync/singleflight`:

```go
var sfGroup singleflight.Group

func (h *PackageHandler) fetchPackage(name string) (*PackageResponse, error) {
    result, err, _ := sfGroup.Do("pkg:"+name, func() (any, error) {
        return h.fetchPackageForce(name)
    })
    // ...
}
```

This collapses all concurrent misses for the same package into one upstream call. The `_` (shared bool) tells you if the result was shared — useful for metrics later.

**Scope:** Apply to the main package fetch only. Stats, changelog, and docs each have their own cache TTLs and are already parallelized per-request.

**Why not the stale-while-revalidate goroutines too?** The SWR path already has stale data to return, so the stampede risk is lower. `singleflight` on the miss path is the high-value fix. We can revisit SWR deduplication post-launch.

### 3. Docker Compose: CPU Limits + Nuxt Memory Bump

**File:** `docker-compose.yml`

```yaml
# API service
deploy:
  resources:
    limits:
      memory: 512M
      cpus: '1.5'         # add this

# Web (Nuxt) service
deploy:
  resources:
    limits:
      memory: 512M        # bump from 256M
      cpus: '0.75'        # add this

# Caddy
deploy:
  resources:
    limits:
      memory: 128M
      cpus: '0.5'         # add this
```

CPU budget: 1.5 + 0.75 + 0.5 = 2.75 vCPU on a 2vCPU host. Oversubscribed intentionally — Docker's CPU limits are soft caps (cgroup throttling), not hard reservations. Under real load, API gets priority.

### 4. Nuxt Hardening

**File:** `web/nuxt.config.ts`

```typescript
devtools: { enabled: false },   // was: true

routeRules: {
  '/packages/**': {
    headers: { 'Cache-Control': 'public, max-age=60, stale-while-revalidate=300' }
  },
  '/': {
    headers: { 'Cache-Control': 'public, max-age=30, stale-while-revalidate=120' }
  },
  '/search': {
    headers: { 'Cache-Control': 'no-store' }   // search is user-specific
  }
}
```

**Why:** Cache-Control headers let browsers and any CDN in front cache rendered pages. Even a 60-second TTL means a viral package page under Reddit load gets served from browser cache after the first hit — reducing SSR calls dramatically.

---

## Droplet Migration: Resize + Team Transfer

**Goal:** Move pypx droplet from Vandal team → My Team, and resize 1GB/1vCPU → 2GB/2vCPU

**Approach:** Snapshot → Transfer → Recreate (not live migration)

**Why snapshot over resize-in-place:** We need to change teams, which requires creating a new droplet. Since we're doing that anyway, we resize at the same time. The snapshot captures the entire disk including Docker volumes (SQLite DBs intact).

### Steps

1. **Deploy all code changes** to the existing droplet first (so the snapshot contains the hardened config)
2. **Power off** the pypx droplet gracefully (`docker compose down` then `shutdown`)
3. **Take snapshot** via doctl: `doctl compute droplet-action snapshot <id> --snapshot-name pypx-launch-hardened`
4. **Switch to My Team context**: `doctl auth switch --context default`
5. **Create new droplet** in My Team from the snapshot, size `s-2vcpu-2gb`, region `nyc3`
6. **Verify** Docker volumes are intact, containers start cleanly
7. **Update DNS** to point to the new droplet IP (record the old IP first)
8. **Delete old droplet** from Vandal team after DNS propagates and traffic is confirmed healthy

**Estimated downtime:** 10–20 minutes (snapshot takes ~5 min for a 25GB disk, droplet creation ~2 min, DNS TTL dependent)

**Data safety:** SQLite DBs live in named Docker volumes on the disk. The snapshot captures the full block device — no separate backup/restore needed.

**DNS consideration:** Check current TTL before starting. If it's 3600s, lower it to 300s a few hours before the migration so cutover is fast.

---

## Risk Assessment

| Change | Risk | Mitigation |
|--------|------|------------|
| Caddy gzip | Low — built-in module, well-tested | Test locally first |
| singleflight | Low — stdlib, well-tested | Unit test the dedup behavior |
| Nuxt devtools off | Negligible | Verify build still works |
| routeRules cache headers | Low | Verify search page excluded |
| Docker memory/CPU limits | Low | 2vCPU host gives us headroom |
| Droplet snapshot transfer | Medium — involves downtime | Deploy code first, take snapshot of known-good state |

---

## Success Criteria

- [ ] Package API responses are gzip-compressed (verify with `curl -H "Accept-Encoding: gzip" ...`)
- [ ] Concurrent requests for same package result in one upstream PyPI call (verify via logs)
- [ ] Nuxt container stays healthy under simulated load without OOMKill
- [ ] Droplet is visible in My Team in DO console
- [ ] All services start cleanly from snapshot
- [ ] DNS resolves to new IP, site is reachable
