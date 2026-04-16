# pypx DigitalOcean Droplet Deployment

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deploy pypx (Go API + Nuxt 4 frontend + Caddy) to a DigitalOcean droplet using Docker Compose, with GitHub Actions CI/CD.

**Architecture:** Single $6/mo droplet running Docker Compose with 3 containers (Caddy, Go API, Nuxt web). GitHub Actions builds Docker images, pushes to GHCR, then SSHes to the droplet to pull and restart. Cloudflare handles DNS and edge SSL; Caddy uses `tls internal` behind Cloudflare's proxy.

**Tech Stack:** Docker, Docker Compose, GitHub Actions, GHCR, Caddy, Cloudflare, doctl

---

## File Map

| File | Purpose |
|------|---------|
| `deploy/provision.sh` | One-time droplet setup (Docker, firewall, swap, app user) |
| `deploy/docker-compose.prod.yml` | Production compose override (GHCR images, restart policies, .env) |
| `.github/workflows/deploy.yml` | CI/CD: build images, push GHCR, SSH deploy, health check |
| `.env.example` | Already exists — no changes needed |
| `api/Dockerfile` | Already exists — no changes needed |
| `web/Dockerfile` | Already exists — no changes needed |
| `Caddyfile` | Needs minor tweak for `tls internal` behind Cloudflare |
| `docker-compose.yml` | Already exists — this stays as the local dev compose |

---

### Task 1: Create the DigitalOcean Droplet

**Files:** None (infrastructure only)

- [ ] **Step 1: Create the droplet with doctl**

```bash
doctl compute droplet create pypx \
  --region nyc3 \
  --size s-1vcpu-1gb \
  --image ubuntu-24-04-x64 \
  --ssh-keys 45018925 \
  --tag-names pypx \
  --wait
```

Expected: Droplet created, IP address returned.

- [ ] **Step 2: Record the droplet IP**

```bash
doctl compute droplet get pypx --format PublicIPv4 --no-header
```

Save this IP — needed for Cloudflare DNS and SSH.

- [ ] **Step 3: Verify SSH access**

```bash
ssh root@<DROPLET_IP> "uname -a"
```

Expected: `Linux pypx ... Ubuntu ...`

---

### Task 2: Write the Provision Script

**Files:**
- Create: `deploy/provision.sh`

- [ ] **Step 1: Create `deploy/provision.sh`**

```bash
#!/bin/bash
# pypx — One-time droplet provisioning script
# Run as root on a fresh Ubuntu 24.04 droplet (1 vCPU, 1GB RAM)
set -euo pipefail

echo "=== pypx Droplet Provisioning ==="

# --- System updates ---
apt-get update && apt-get upgrade -y

# --- Swap (critical for 1GB droplet — Docker needs headroom) ---
if [ ! -f /swapfile ]; then
  echo "Setting up 1GB swap..."
  fallocate -l 1G /swapfile
  chmod 600 /swapfile
  mkswap /swapfile
  swapon /swapfile
  echo '/swapfile none swap sw 0 0' >> /etc/fstab
fi

# --- Docker ---
apt-get install -y ca-certificates curl gnupg
install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
chmod a+r /etc/apt/keyrings/docker.asc

echo \
  "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu \
  $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | \
  tee /etc/apt/sources.list.d/docker.list > /dev/null

apt-get update
apt-get install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin

systemctl enable docker
systemctl start docker

# --- Firewall ---
ufw allow 22/tcp
ufw allow 80/tcp
ufw allow 443/tcp
ufw --force enable

# --- App directory ---
mkdir -p /opt/pypx
chmod 755 /opt/pypx

echo ""
echo "=== Provisioning complete ==="
echo "Next steps:"
echo "  1. Copy .env to /opt/pypx/.env"
echo "  2. Copy docker-compose.prod.yml to /opt/pypx/docker-compose.yml"
echo "  3. Copy Caddyfile to /opt/pypx/Caddyfile"
echo "  4. Run: cd /opt/pypx && docker compose up -d"
echo "  5. Configure Cloudflare DNS (A record → $(curl -s ifconfig.me))"
```

- [ ] **Step 2: Commit**

```bash
git add deploy/provision.sh
git commit -m "chore: add droplet provisioning script"
```

---

### Task 3: Write the Production Docker Compose

**Files:**
- Create: `deploy/docker-compose.prod.yml`

- [ ] **Step 1: Create `deploy/docker-compose.prod.yml`**

This replaces the local build-from-source compose with one that pulls pre-built images from GHCR.

```yaml
services:
  caddy:
    image: caddy:2-alpine
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./Caddyfile:/etc/caddy/Caddyfile
      - caddy_data:/data
      - caddy_config:/config
    depends_on:
      api:
        condition: service_healthy
      web:
        condition: service_healthy
    restart: unless-stopped
    deploy:
      resources:
        limits:
          memory: 128M

  api:
    image: ghcr.io/mattstrayer/pypx/api:latest
    env_file: .env
    environment:
      - API_PORT=8080
      - SQLITE_PATH=/data/pypx.db
    volumes:
      - api_data:/data
    expose:
      - "8080"
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "-q", "--spider", "http://localhost:8080/api/health"]
      interval: 30s
      timeout: 5s
      retries: 3
      start_period: 10s
    deploy:
      resources:
        limits:
          memory: 512M

  web:
    image: ghcr.io/mattstrayer/pypx/web:latest
    environment:
      - API_BASE=http://api:8080
      - NUXT_PUBLIC_API_BASE=/api
    expose:
      - "3000"
    depends_on:
      api:
        condition: service_healthy
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "-q", "--spider", "http://localhost:3000"]
      interval: 30s
      timeout: 5s
      retries: 3
      start_period: 15s
    deploy:
      resources:
        limits:
          memory: 256M

volumes:
  caddy_data:
  caddy_config:
  api_data:
```

- [ ] **Step 2: Commit**

```bash
git add deploy/docker-compose.prod.yml
git commit -m "chore: add production docker-compose with GHCR images"
```

---

### Task 4: Update Caddyfile for Cloudflare SSL

**Files:**
- Modify: `Caddyfile`

- [ ] **Step 1: Update `Caddyfile` to use `tls internal` for Cloudflare**

The existing Caddyfile uses `{$DOMAIN:localhost}` which is good. Add `tls internal` so Caddy uses self-signed certs behind Cloudflare's proxy (Cloudflare "Full" SSL mode — Cloudflare terminates public SSL, connects to origin over HTTPS with self-signed cert).

```
{$DOMAIN:localhost} {
    tls internal

    # Trust Cloudflare proxy headers
    servers {
        trusted_proxies static 173.245.48.0/20 103.21.244.0/22 103.22.200.0/22 103.31.4.0/22 141.101.64.0/18 108.162.192.0/18 190.93.240.0/20 188.114.96.0/20 197.234.240.0/22 198.41.128.0/17 162.158.0.0/15 104.16.0.0/13 104.24.0.0/14 172.64.0.0/13 131.0.72.0/22 2400:cb00::/32 2606:4700::/32 2803:f800::/32 2405:b500::/32 2405:8100::/32 2a06:98c0::/29 2c0f:f248::/32
    }

    header {
        # Security headers
        X-Content-Type-Options nosniff
        X-Frame-Options DENY
        Referrer-Policy strict-origin-when-cross-origin
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

- [ ] **Step 2: Commit**

```bash
git add Caddyfile
git commit -m "chore: add tls internal for Cloudflare Full SSL mode"
```

---

### Task 5: Write the GitHub Actions Deploy Workflow

**Files:**
- Create: `.github/workflows/deploy.yml`

- [ ] **Step 1: Create `.github/workflows/deploy.yml`**

```yaml
name: Deploy

on:
  workflow_dispatch:
    inputs:
      bump:
        description: 'Version bump'
        required: true
        type: choice
        options:
          - patch
          - minor
          - major

concurrency:
  group: deploy
  cancel-in-progress: false

permissions:
  contents: write
  packages: write

jobs:
  deploy:
    runs-on: ubuntu-latest
    environment: production
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Compute next version
        id: version
        run: |
          LATEST=$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
          LATEST="${LATEST#v}"
          IFS='.' read -r MAJOR MINOR PATCH <<< "$LATEST"

          case "${{ inputs.bump }}" in
            major) MAJOR=$((MAJOR + 1)); MINOR=0; PATCH=0 ;;
            minor) MINOR=$((MINOR + 1)); PATCH=0 ;;
            patch) PATCH=$((PATCH + 1)) ;;
          esac

          VERSION="$MAJOR.$MINOR.$PATCH"
          echo "version=$VERSION" >> "$GITHUB_OUTPUT"
          echo "prev=v$LATEST" >> "$GITHUB_OUTPUT"
          echo "Deploying v$VERSION (${{ inputs.bump }} bump from v$LATEST)"

      - name: Log in to GHCR
        uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Build and push API image
        uses: docker/build-push-action@v6
        with:
          context: ./api
          push: true
          tags: |
            ghcr.io/mattstrayer/pypx/api:latest
            ghcr.io/mattstrayer/pypx/api:v${{ steps.version.outputs.version }}

      - name: Build and push Web image
        uses: docker/build-push-action@v6
        with:
          context: ./web
          push: true
          tags: |
            ghcr.io/mattstrayer/pypx/web:latest
            ghcr.io/mattstrayer/pypx/web:v${{ steps.version.outputs.version }}

      - name: Set up SSH
        run: |
          mkdir -p ~/.ssh
          echo "${{ secrets.DROPLET_SSH_KEY }}" > ~/.ssh/deploy_key
          chmod 600 ~/.ssh/deploy_key
          ssh-keyscan -H ${{ secrets.DROPLET_HOST }} >> ~/.ssh/known_hosts

      - name: Deploy to droplet
        env:
          HOST: ${{ secrets.DROPLET_HOST }}
        run: |
          SSH="ssh -i ~/.ssh/deploy_key -o StrictHostKeyChecking=no root@$HOST"
          SCP="scp -i ~/.ssh/deploy_key -o StrictHostKeyChecking=no"

          # Sync production configs
          $SCP deploy/docker-compose.prod.yml root@$HOST:/opt/pypx/docker-compose.yml
          $SCP Caddyfile root@$HOST:/opt/pypx/Caddyfile

          $SSH << 'DEPLOY'
            set -euo pipefail
            cd /opt/pypx

            # Log in to GHCR (uses a PAT stored on the droplet)
            echo "$GHCR_TOKEN" | docker login ghcr.io -u mattstrayer --password-stdin 2>/dev/null || true

            # Pull latest images and restart
            docker compose pull
            docker compose up -d --remove-orphans

            # Wait for health checks
            sleep 10

            # Verify API is responding
            curl -sf http://localhost:80/api/health > /dev/null
            echo "Deploy successful"
          DEPLOY

      - name: Tag and release
        env:
          GH_TOKEN: ${{ github.token }}
        run: |
          VERSION="v${{ steps.version.outputs.version }}"
          PREV="${{ steps.version.outputs.prev }}"

          git tag "$VERSION"
          git push origin "$VERSION"

          if [ "$PREV" != "v0.0.0" ]; then
            CHANGES=$(git log --pretty=format:"- %s" "$PREV"..HEAD)
          else
            CHANGES=$(git log --pretty=format:"- %s")
          fi

          gh release create "$VERSION" \
            --title "$VERSION" \
            --notes "## What's Changed
          $CHANGES"
```

- [ ] **Step 2: Commit**

```bash
git add .github/workflows/deploy.yml
git commit -m "ci: add GitHub Actions deploy workflow with GHCR"
```

---

### Task 6: Provision the Droplet

**Files:** None (remote operations)

- [ ] **Step 1: Copy provision script to droplet and run it**

```bash
DROPLET_IP=$(doctl compute droplet get pypx --format PublicIPv4 --no-header)
scp deploy/provision.sh root@$DROPLET_IP:/root/provision.sh
ssh root@$DROPLET_IP "chmod +x /root/provision.sh && /root/provision.sh"
```

Expected: Script installs Docker, sets up firewall, creates swap, creates `/opt/pypx/`.

- [ ] **Step 2: Copy production configs to the droplet**

```bash
scp deploy/docker-compose.prod.yml root@$DROPLET_IP:/opt/pypx/docker-compose.yml
scp Caddyfile root@$DROPLET_IP:/opt/pypx/Caddyfile
```

- [ ] **Step 3: Create `.env` on the droplet**

```bash
ssh root@$DROPLET_IP "cat > /opt/pypx/.env << 'EOF'
DOMAIN=pypx.app
GITHUB_TOKEN=
EOF
chmod 600 /opt/pypx/.env"
```

Note: Add `GITHUB_TOKEN` later if needed for higher PyPI rate limits.

- [ ] **Step 4: Set up GHCR authentication on the droplet**

Create a GitHub PAT (classic) with `read:packages` scope, then store it on the droplet:

```bash
ssh root@$DROPLET_IP "echo '<PAT_TOKEN>' > /opt/pypx/.ghcr-token && chmod 600 /opt/pypx/.ghcr-token"
ssh root@$DROPLET_IP "cat /opt/pypx/.ghcr-token | docker login ghcr.io -u mattstrayer --password-stdin"
```

---

### Task 7: First Deploy (Manual)

**Files:** None (remote operations)

- [ ] **Step 1: Build and push images locally for the first deploy**

```bash
cd /Users/matt/dev/pypx
docker build -t ghcr.io/mattstrayer/pypx/api:latest ./api
docker build -t ghcr.io/mattstrayer/pypx/web:latest ./web
docker push ghcr.io/mattstrayer/pypx/api:latest
docker push ghcr.io/mattstrayer/pypx/web:latest
```

Note: You need to `docker login ghcr.io` first with a PAT that has `write:packages`.

- [ ] **Step 2: Pull and start on the droplet**

```bash
DROPLET_IP=$(doctl compute droplet get pypx --format PublicIPv4 --no-header)
ssh root@$DROPLET_IP "cd /opt/pypx && docker compose pull && docker compose up -d"
```

- [ ] **Step 3: Verify health**

```bash
ssh root@$DROPLET_IP "curl -sf http://localhost:80/api/health"
```

Expected: Health check response from the Go API.

---

### Task 8: Configure Cloudflare DNS

**Files:** None (Cloudflare dashboard)

- [ ] **Step 1: Add A record in Cloudflare**

In the Cloudflare dashboard for `pypx.app`:
- Type: `A`
- Name: `@`
- Content: `<DROPLET_IP>`
- Proxy status: **Proxied** (orange cloud)

- [ ] **Step 2: Set SSL mode to "Full"**

In Cloudflare dashboard → SSL/TLS → Overview:
- Set encryption mode to **Full** (not Full Strict — we're using self-signed on origin)

- [ ] **Step 3: Verify the site is live**

```bash
curl -sf https://pypx.app/api/health
```

Expected: Health check response through Cloudflare → Caddy → API.

---

### Task 9: Configure GitHub Actions Secrets

**Files:** None (GitHub settings)

- [ ] **Step 1: Add repository secrets**

In GitHub → `mattstrayer/pypx` → Settings → Secrets and variables → Actions:

1. `DROPLET_SSH_KEY` — The private SSH key for the droplet (the one matching the `mba` key fingerprint `45018925`)
2. `DROPLET_HOST` — The droplet's IP address

- [ ] **Step 2: Create the `production` environment**

In GitHub → `mattstrayer/pypx` → Settings → Environments:
- Create environment named `production`
- Optionally add required reviewers for deploy approval

- [ ] **Step 3: Test the workflow**

Go to Actions → Deploy → Run workflow → Select `patch` → Run.

Expected: Images build, push to GHCR, deploy to droplet, health check passes, v0.1.0 tag created.
