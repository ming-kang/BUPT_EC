# Server Deployment

This guide covers configuring a server and installing BUPT_EC for the first time. For upgrading an existing installation see [upgrading.md](upgrading.md); for day-to-day operation see [operations.md](operations.md).

The recommended path is the one-command installer on a Debian/Ubuntu server. A manual systemd + Nginx setup is described at the end for other environments.

## Prerequisites

- A Debian/Ubuntu server (or another apt-based system) with `amd64` or `arm64` CPU.
- A domain (for example `ec.example.com`) pointing at the server: an `A` record for IPv4 and, if available, an `AAAA` record for IPv6.
- Inbound TCP ports `80` and `443` open in the cloud firewall/security group and in the host firewall if one is enabled.
- An SSL certificate and private key already on the server (see below).
- Network access from the server to GitHub (for release downloads) and to the BUPT teaching affairs system.
- A valid BUPT teaching affairs account.

## TLS certificate

The installer runs in an existing-certificate mode: it configures Nginx to use a certificate that is already present, but it does **not** request or renew certificates for you.

A common Let's Encrypt standalone flow:

```bash
sudo apt-get update
sudo apt-get install -y certbot
sudo certbot certonly --standalone -d ec.example.com
```

The standalone challenge needs port `80` reachable and unoccupied while the certificate is issued. DNS validation, a commercial certificate, or another certificate manager also work as long as you know the final file paths.

For a default Let's Encrypt certificate the paths are usually:

```text
/etc/letsencrypt/live/ec.example.com/fullchain.pem
/etc/letsencrypt/live/ec.example.com/privkey.pem
```

Verify before installing:

```bash
sudo test -f /etc/letsencrypt/live/ec.example.com/fullchain.pem && echo cert ok
sudo test -f /etc/letsencrypt/live/ec.example.com/privkey.pem && echo key ok
```

Renewal stays the responsibility of your certificate manager (for example Certbot's renewal timer). If renewed files keep the same paths, reloading Nginx after renewal is enough — rerunning the installer is not required.

## One-command install

**Production:** prefer an immutable stable tag (or GitHub `latest` stable). On
a first install with no explicit or saved release choice, the fallback remains
the rolling `nightly` prerelease (edge / freshest `main`).

Stable release (recommended for production):

```bash
curl -fsSL https://github.com/ming-kang/BUPT_EC/releases/latest/download/install.sh | sudo VERSION=latest bash
# or a fixed version:
curl -fsSL https://github.com/ming-kang/BUPT_EC/releases/download/v0.1.4/install.sh | sudo VERSION=v0.1.4 bash
```

Rolling nightly (edge):

```bash
curl -fsSL https://github.com/ming-kang/BUPT_EC/releases/download/nightly/install.sh | sudo VERSION=nightly bash
```

The installer stores the selected `latest`, `nightly`, or fixed `vX.Y.Z` value
as `RELEASE_VERSION` in `/etc/bupt-ec/bupt-ec.env`. Rerunning it without an
explicit `VERSION` keeps that channel or pinned tag; an explicit `VERSION`
always overrides the saved value. A first-time install with no `VERSION` keeps
the historical `nightly` default.

The script asks interactively for:

- GitHub repository (default `ming-kang/BUPT_EC`)
- domain name
- SSL certificate and private key paths (defaults follow the Let's Encrypt layout)
- BUPT teaching affairs username and password, or an optional token override
- backend listen address (default `127.0.0.1:8080`)
- Gin mode (default `release`)

Environment variables can pre-seed or override choices, for example:

```bash
curl -fsSL https://github.com/ming-kang/BUPT_EC/releases/download/v0.1.4/install.sh | sudo REPO=ming-kang/BUPT_EC VERSION=v0.1.4 bash
```

## What the installer does

- Installs `ca-certificates`, `curl`, `tar`, and `nginx` via apt.
- Creates a dedicated `bupt-ec` system user and group.
- Downloads the release tarball matching the CPU architecture and requires a matching `checksums.txt` entry (install fails if the checksum file is missing or verification fails). Set `SKIP_CHECKSUM=1` only as an explicit break-glass to skip verification.
- Extracts the archive and renders candidate env, systemd, and Nginx files in a root-only staging directory before changing any installed target.
- Snapshots the existing binary, env, systemd unit/enablement, and Nginx site/enablement, then replaces files with same-filesystem atomic renames.
- Installs the binary to `/opt/bupt-ec/bupt-ec`, owned by root so the running service cannot rewrite its own executable. Only `/opt/bupt-ec/run_log` is writable by the service user.
- Writes the configuration to `/etc/bupt-ec/bupt-ec.env` (mode `0600`, owned by root).
- Installs a hardened systemd unit (`NoNewPrivileges`, `PrivateTmp`, `ProtectHome`, `ProtectSystem=full`, empty capability bounding set, and more) and enables it.
- Writes an Nginx site with HTTP→HTTPS redirect, TLS 1.2/1.3, security headers, and rate limiting on `/api/` (30 requests/minute per IP with a burst of 20).
- Validates Nginx, restarts and checks the service, reloads Nginx, and checks `/healthz` when `APP_ADDR` is loopback. A failure restores the snapshot (or removes newly created first-install files) and attempts to restart the previous service before the installer exits non-zero.

The installer prints its success message only after all commit validations pass. After installation the site is served at `https://<your-domain>/`.

## Offline or restricted networks (explicit mirrors)

By default the installer downloads only from official GitHub release URLs. It
does **not** auto-select third-party proxies. If GitHub is unreachable, the
installer fails before changing installed files and tells you how to pass an
explicit mirror.

When you control a trusted mirror (for example on an IPv6-only network that
cannot reach GitHub), copy the matching release assets there and point the
installer at that base URL:

```bash
# Obtain install.sh from a machine that can reach GitHub (or your mirror),
# inspect it, then run on the target host:
sudo VERSION=v0.1.4 DOWNLOAD_BASE_URL=https://your-mirror.example/releases/v0.1.4 bash install.sh
```

The mirror directory must contain `bupt-ec-linux-amd64.tar.gz` or
`bupt-ec-linux-arm64.tar.gz` and a `checksums.txt` that lists the package hash
(verification is required unless `SKIP_CHECKSUM=1`). Same-origin checksums prove
download integrity only; they are not independent proof of GitHub publisher
identity if the mirror itself is compromised.

`DOWNLOAD_BASE_URL` must use HTTPS. For a trusted local mirror only, set
`ALLOW_INSECURE_DOWNLOAD_BASE_URL=true` to allow plain HTTP. A saved
`DOWNLOAD_BASE_URL` from a previous explicit choice is reused on upgrades; the
installer never writes a mirror URL discovered by network probing.

Do not pipe installers from unknown third-party hosts into `sudo bash`.

## Manual deployment

For non-apt systems or custom setups, deploy the release tarball by hand.

Download and unpack a release from [GitHub Releases](https://github.com/ming-kang/BUPT_EC/releases), then:

```text
/opt/bupt-ec/
  bupt-ec          # binary from the tarball

/etc/bupt-ec/
  bupt-ec.env      # environment file, mode 0600
```

`bupt-ec.env` needs at least:

```bash
JW_USERNAME=your_username
JW_PASSWORD=your_password
APP_ADDR=127.0.0.1:8080
GIN_MODE=release
```

Example systemd unit (`/etc/systemd/system/bupt-ec.service`):

```ini
[Unit]
Description=BUPT_EC
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=/opt/bupt-ec
ExecStart=/opt/bupt-ec/bupt-ec
Restart=always
RestartSec=5
EnvironmentFile=/etc/bupt-ec/bupt-ec.env

[Install]
WantedBy=multi-user.target
```

For production, also add the hardening directives used by the installer (see `scripts/install.sh::render_systemd_service`).

Enable and start:

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now bupt-ec
sudo systemctl status bupt-ec
```

Minimal Nginx reverse proxy:

```nginx
server {
    listen 80;
    server_name your.domain.example;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

The full production site written by the installer (TLS, security headers, rate limiting) is in `scripts/install.sh::render_nginx_site` and can be used as a template.
