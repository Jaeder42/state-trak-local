# Deploying StateTrak to a VPS (Linode)

Target shape: one Linux box, the Go binary behind a reverse proxy. The app has
no built-in auth — **the proxy's basic auth is the auth layer**, so it must
stay enabled. Friends get a username/password; the browser handles the prompt
for both the UI and the API (same origin).

If you already run nginx on the box, use the nginx section below — the only
required pieces are auth, `client_max_body_size`, and the proxy block.
Caddy is documented as the from-scratch alternative.

~620 MB of disk per uploaded demo (file copy + round JSON), so size the
volume for the demo library you want to keep.

## 1. Build (on your machine)

```sh
GOOS=linux GOARCH=amd64 go build -o statetrak-linux .     # x86_64 VPS
GOOS=linux GOARCH=arm64 go build -o statetrak-linux .     # Ampere/ARM VPS
```

The client is embedded in the binary — run `make client` first if the UI
isn't built yet (`web/dist` must be current).

## 2. Server setup (Ubuntu/Debian)

```sh
# as root:
useradd --system --home /opt/statetrak --shell /usr/sbin/nologin statetrak
mkdir -p /opt/statetrak/controllers/data
```

Copy files over:

```sh
scp statetrak-linux user@your-vps:/tmp/
scp deploy/statetrak.service user@your-vps:/tmp/
```

On the server:

```sh
mv /tmp/statetrak-linux /opt/statetrak/statetrak
chmod 755 /opt/statetrak/statetrak
chown -R statetrak:statetrak /opt/statetrak
```

Create `/opt/statetrak/.env` (owned by statetrak, mode 600):

```
TYPESAFE_API_KEY=apikey_...
# optional:
# UPLOAD_MAX_BYTES=1073741824   # 1 GB default
# PARSE_CONCURRENCY=1          # concurrent demo parses (RAM cap)
# ALLOWED_ORIGIN=              # only needed for a cross-origin client
```

## 3. systemd

```sh
cp /tmp/statetrak.service /etc/systemd/system/statetrak.service
systemctl daemon-reload
systemctl enable --now statetrak
systemctl status statetrak      # should be active, listening on :3007
```

The app writes to `controllers/data/` relative to `WorkingDirectory`
(`/opt/statetrak`), which the unit allows via `ReadWritePaths`.

## 4. Reverse proxy — nginx

You already run nginx: merge the three marked blocks from
[`deploy/nginx.conf`](nginx.conf) into your certbot-managed server block:

```nginx
auth_basic "StateTrak";
auth_basic_user_file /etc/nginx/.htpasswd;   # friends' logins
client_max_body_size 1G;                     # demos are hundreds of MB
location / {
    proxy_pass http://127.0.0.1:3007;
}
```

Password file:

```sh
apt install -y apache2-utils
htpasswd -c /etc/nginx/.htpasswd friend        # first user
htpasswd /etc/nginx/.htpasswd secondfriend     # add more friends
```

Two nginx gotchas that matter here:

- **`client_max_body_size` defaults to 1 MB** — without the override, every
  demo upload dies with `413 Request Entity Too Large`.
- **Timeouts**: the first JEV analysis request runs ~7-10 s. The default
  60 s `proxy_read_timeout` is fine, but the config bumps it to 300 s for
  slow mobile uploads.

Then:

```sh
nginx -t && systemctl reload nginx
```

### Alternative: Caddy (from-scratch boxes)

Not already running a proxy? Caddy gives you TLS + auth in one package:

```sh
apt install -y caddy    # use the official caddy apt repo, see caddyserver.com
```

Start from [`deploy/Caddyfile`](Caddyfile): set the domain, generate the hash
with `caddy hash-password`, then `systemctl reload caddy`.

## 5. Firewall

```sh
ufw allow 22/tcp
ufw allow 80/tcp
ufw allow 443/tcp
ufw enable
```

`:3007` stays unexposed — the proxy reaches it over loopback.

## 6. Verify

```sh
curl -u friend:PASSWORD https://your-domain/ping     # {"message":"pong"}
```

Then open `https://your-domain` in a browser, log in, upload a demo, and hit
**JEV analysis** in the demo menu (☰). First run takes a few seconds and is
billed once; the result is cached in the demo's directory as `analysis.json`.

## Operations notes

- **Logs**: `journalctl -u statetrak -f`
- **Updates**: rebuild locally, `scp` + `mv` over the binary, `systemctl restart statetrak`
- **Disk**: each demo ≈ its file size ×2.3; delete demos via the UI (removes
  upload, round JSON and the JEV cache together)
- **RAM**: parses are capped at `PARSE_CONCURRENCY` (default 1); a 266 MB demo
  peaks in the hundreds of MB. A 2 GB Linode is comfortable.
- **The JEV key** never leaves the server; the client only sees analysis
  results. Without the key the `/analysis` route returns 503 and everything
  else still works.