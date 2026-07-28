# Cloudflare Setup for pypx.app (Free Tier)

## DNS Records

| Type | Name | Content | Proxy | TTL |
|------|------|---------|-------|-----|
| A | `pypx.app` | `<droplet-ip>` | Proxied (orange cloud) | Auto |
| A | `www` | `<droplet-ip>` | Proxied (orange cloud) | Auto |
| CNAME | `www` | `pypx.app` | Proxied | Auto |

Use either the A record or CNAME for `www` — not both. CNAME is simpler if you ever change the droplet IP.

## SSL/TLS

**Settings → SSL/TLS → Overview:**
- Encryption mode: **Full (strict)**
  - Caddy auto-provisions a certificate from Let's Encrypt. "Full (strict)" means Cloudflare verifies the origin cert is valid — this is the most secure option.

**Settings → SSL/TLS → Edge Certificates:**
- Always Use HTTPS: **On**
- Minimum TLS Version: **TLS 1.2**
- TLS 1.3: **On**
- Automatic HTTPS Rewrites: **On**

## Speed / Performance

**Settings → Speed → Optimization:**

*Content optimization:*
- Auto Minify: **On** (HTML, CSS, JS)
- Brotli: **On** (better compression than gzip)

*Image optimization:*
- Not available on free tier — skip

**Settings → Speed → Optimization → Protocol Optimization:**
- HTTP/2: **On** (default)
- HTTP/3 (QUIC): **On**
- 0-RTT Connection Resumption: **On**

## Caching

**Settings → Caching → Configuration:**
- Caching Level: **Standard**
- Browser Cache TTL: **Respect Existing Headers** (our Go API already sets Cache-Control)

**Settings → Caching → Tiered Cache:**
- Tiered Cache: **On** (free — reduces origin hits by serving from Cloudflare's upper-tier data centers)

### Cache Rules (Settings → Rules → Cache Rules)

Create these rules to control what gets cached at the edge:

**Rule 1: Cache API package responses**
- When: URI Path starts with `/api/packages/`
- Then: Cache eligible, Edge TTL override 1 hour, Browser TTL respect origin
- Why: Package data changes infrequently. This is your biggest win — most page loads will be served from Cloudflare's edge.

**Rule 2: Don't cache search**
- When: URI Path starts with `/api/search`
- Then: Bypass cache
- Why: Search results are dynamic and personalized by query. The 5-minute Cache-Control header handles browser caching; edge caching would serve stale results to different users.

**Rule 3: Cache static assets aggressively**
- When: URI Path matches `/_nuxt/*`
- Then: Cache eligible, Edge TTL override 30 days, Browser TTL 30 days
- Why: Nuxt static assets have content hashes in filenames — they're immutable. Cache forever.

See "AI agent traffic" below for the `/llms.txt` cache rule (pending owner setup).

## AI agent traffic (REQUIRED end-state)

pypx is agent-first: coding agents (Claude Code, Cursor, ClaudeBot, GPTBot,
plain curl) MUST receive 200s with real bodies on `/llms.txt` and all
`/api/**` routes — never a challenge or block.

- **Bot Fight Mode: OFF.** It challenges unverified automated clients,
  which is exactly our target audience. Abuse is handled by the
  rate-limit rule below plus the origin limiter (30 req/s, burst 60).
- **No challenge actions on `/api/*` or `/llms.txt`.** If a managed
  challenge is ever needed for the HTML site, scope it to paths NOT
  matching `/api/*` and `/llms.txt`.
- **Keep:** rate-limit rule — `/api/*` over 60 req/min/IP → block 10 min
  (advertised to agents via llms.txt and 429 Retry-After headers).
- **Cache rule (add):** `/llms.txt` — Cache eligible, Edge TTL 1 hour
  (it lives outside `/api/*` so the API cache rule does not cover it).

Verify after any dashboard change:
    ./scripts/check-agent-access.sh

## Security

**Settings → Security → Settings:**
- Security Level: **Medium**
- Challenge Passage: **30 minutes**
- Browser Integrity Check: **On**

**Settings → Security → WAF:**
- Free tier gets managed rulesets — leave defaults on

### DDoS

Cloudflare's DDoS protection is always on (free tier). No configuration needed. Our Caddyfile already trusts Cloudflare's IP ranges for proper client IP forwarding.

## Page Rules (Settings → Rules → Page Rules — 3 free)

**Rule 1: Force HTTPS**
- URL: `http://pypx.app/*`
- Setting: Always Use HTTPS
- (Redundant with the SSL setting, but belt-and-suspenders)

**Rule 2: Redirect www to apex**
- URL: `www.pypx.app/*`
- Setting: Forwarding URL (301) → `https://pypx.app/$1`
- Why: Single canonical domain for SEO

**Rule 3: Cache everything on static paths**
- URL: `pypx.app/_nuxt/*`
- Setting: Cache Level → Cache Everything, Edge Cache TTL → 1 month
- (Reinforces Cache Rule 3 above)

## Firewall / WAF Rules (Settings → Security → WAF → Custom rules — 5 free)

**Rule 1: Rate limit API**
- When: URI Path starts with `/api/` AND Rate exceeds 60 requests per minute per IP
- Then: Block for 10 minutes
- Why: Prevents a single IP from overwhelming the origin — this is the only
  abuse control on `/api/*`. See "AI agent traffic" above: no challenge or
  CAPTCHA rule may target `/api/*` or `/llms.txt`.

## Analytics

**Settings → Analytics & Logs → Web Analytics:**
- Enable Web Analytics (free, privacy-friendly, no JS tag needed when proxied)
- Gives you traffic, performance, and error metrics

## Network

**Settings → Network:**
- HTTP/2 to Origin: **On**
- WebSockets: **Off** (we don't use them)
- Onion Routing: **Off**
- Pseudo IPv4: **Off**

## Verification Checklist

After setup, verify:

```bash
# DNS resolves through Cloudflare
dig pypx.app +short
# Should return Cloudflare IPs (104.x.x.x or 172.x.x.x), NOT your droplet IP

# HTTPS works
curl -I https://pypx.app
# Should return 200 with cf-ray header

# API works through Cloudflare
curl -s https://pypx.app/api/health
# {"status":"ok"}

# www redirects
curl -I https://www.pypx.app
# Should return 301 → https://pypx.app/

# Caching works
curl -sI https://pypx.app/api/packages/requests | grep cf-cache
# Should show cf-cache-status: HIT (on second request)

# Security headers present
curl -sI https://pypx.app | grep -i "x-content-type"
# x-content-type-options: nosniff
```
