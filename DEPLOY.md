# Deploying Tablić on Hetzner

**Source code:** https://github.com/Pajke/tablic

Self-hosted guide for a single VPS running Docker + Caddy.
The Go binary serves the built PixiJS client as embedded static files, so there is only one process to manage.

---

## 1. Provision a server

Any Hetzner VPS works. The game is lightweight — a **CX22** (2 vCPU, 4 GB RAM, ~€4/mo) is more than enough for a friend group.

1. Create a server in the Hetzner Cloud Console
   - Image: **Ubuntu 24.04**
   - Location: choose the one closest to your players
   - Add your SSH public key during creation
2. Note the server's public IP address.

---

## 2. Initial server setup

SSH in and run these once:

```bash
ssh root@<YOUR_IP>

# System updates
apt update && apt upgrade -y

# Install Docker
apt install -y ca-certificates curl
install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] \
  https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo "$VERSION_CODENAME") stable" \
  > /etc/apt/sources.list.d/docker.list
apt update && apt install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin

# Install Caddy (reverse proxy + automatic TLS)
apt install -y debian-keyring debian-archive-keyring apt-transport-https
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | \
  gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | \
  tee /etc/apt/sources.list.d/caddy-stable.list
apt update && apt install -y caddy

# Create app directory and data volume directory
mkdir -p /opt/tablic/data
```

---

## 3. Point your domain

In your DNS provider, add an **A record**:

```
tablic.yourdomain.com  →  <YOUR_IP>
```

Wait a few minutes for DNS to propagate before continuing.

---

## 4. Deploy the application

### Option A — build on the server (simplest)

```bash
# Install Git, Node, Go build deps (one-time)
apt install -y git nodejs npm golang-go

# Clone the repo
cd /opt
git clone https://github.com/Pajke/tablic.git
cd tablic

# Build
chmod +x build.sh
./build.sh          # outputs server/tablic binary

# Copy binary and set permissions
cp server/tablic /opt/tablic/tablic
chmod +x /opt/tablic/tablic
```

### Option B — build locally, copy binary (recommended for CI)

On your dev machine:

```bash
./build.sh
scp server/tablic root@<YOUR_IP>:/opt/tablic/tablic
```

### Option C — Docker (if you prefer containers)

```bash
cd /opt/tablic
docker build -t tablic:latest .
docker run -d \
  --name tablic \
  --restart unless-stopped \
  -p 127.0.0.1:8080:8080 \
  -v /opt/tablic/data:/data \
  -e PORT=8080 \
  -e DB_PATH=/data/tablic.db \
  tablic:latest
```

Skip to step 6 (Caddy config) if using Docker.

---

## 5. Run as a systemd service (Option A/B)

Create `/etc/systemd/system/tablic.service`:

```ini
[Unit]
Description=Tablic card game server
After=network.target

[Service]
Type=simple
User=nobody
WorkingDirectory=/opt/tablic
ExecStart=/opt/tablic/tablic
Environment=PORT=8080
Environment=DB_PATH=/opt/tablic/data/tablic.db
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
systemctl daemon-reload
systemctl enable --now tablic
systemctl status tablic
```

---

## 6. Configure Caddy (HTTPS + reverse proxy)

Edit `/etc/caddy/Caddyfile`:

```
tablic.yourdomain.com {
    reverse_proxy localhost:8080
}
```

Caddy automatically obtains a Let's Encrypt TLS certificate. Reload:

```bash
systemctl reload caddy
```

Your game is now live at `https://tablic.yourdomain.com`.

---

## 7. Verify it works

```bash
# Health check
curl https://tablic.yourdomain.com/health
# Should return: ok

# Check logs
journalctl -u tablic -f          # systemd
docker logs -f tablic             # Docker
```

Open the URL in a browser, create a room, and join from another tab.

---

## 8. Updating the game

### Systemd deploy

```bash
cd /opt/tablic
git pull
./build.sh
cp server/tablic /opt/tablic/tablic
systemctl restart tablic
```

### Docker deploy

```bash
cd /opt/tablic
git pull
docker build -t tablic:latest .
docker stop tablic && docker rm tablic
docker run -d \
  --name tablic \
  --restart unless-stopped \
  -p 127.0.0.1:8080:8080 \
  -v /opt/tablic/data:/data \
  -e PORT=8080 \
  -e DB_PATH=/data/tablic.db \
  tablic:latest
```

---

## 9. SQLite backups

The database lives at `/opt/tablic/data/tablic.db`.
SQLite supports hot-backup with the `.backup` command:

```bash
# One-off backup
sqlite3 /opt/tablic/data/tablic.db ".backup /opt/tablic/data/tablic.db.bak"

# Daily cron backup (add to crontab -e)
0 3 * * * sqlite3 /opt/tablic/data/tablic.db ".backup /opt/tablic/data/tablic_$(date +\%Y\%m\%d).db"
```

---

## 10. Firewall

Hetzner's cloud firewall (or `ufw`) — only ports 22, 80, 443 need to be open:

```bash
ufw allow OpenSSH
ufw allow 80
ufw allow 443
ufw enable
```

Port 8080 stays bound to `127.0.0.1` only (Caddy talks to it locally).

---

## Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT`   | `3579`  | Port the Go server listens on |
| `DB_PATH`| `tablic.db` | Path to SQLite database file |

---

## Troubleshooting

**WebSocket connections fail**
Caddy proxies WebSockets automatically. If using nginx instead, add to the location block:
```nginx
proxy_http_version 1.1;
proxy_set_header Upgrade $http_upgrade;
proxy_set_header Connection "upgrade";
```

**"Failed to load history"**
Check server logs — likely the DB file is not writable. Verify `DB_PATH` directory exists and the service user has write access.

**Players can't reconnect after server restart**
Reconnect tokens are in-memory only — they are lost on restart. Players will need to re-join manually.
