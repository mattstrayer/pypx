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
