# Server Deployment

This guide covers configuring a server and installing BUPT_EC for the first time. For upgrading an existing installation see [upgrading.md](upgrading.md); for day-to-day operation see [operations.md](operations.md).

The recommended path is the one-command installer on a Debian/Ubuntu server. A manual systemd + Nginx setup is described at the end for other environments.

**Supported topology:** one `bupt-ec` process behind one Nginx reverse proxy on a
single host (`APP_ADDR` loopback). Do not run multiple active app instances for
horizontal scale-out today — cache, token, refresh singleflight, and readiness
are process-local. See [operations.md](operations.md#deployment-topology-supported-today)
for limits and future options.

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

**Production:** prefer an immutable stable tag (or GitHub `latest` stable).
Stable releases are the only channel; a first install with no explicit or saved
release choice resolves to `latest`.

```bash
curl -fsSL https://github.com/ming-kang/BUPT_EC/releases/latest/download/install.sh | sudo VERSION=latest bash
# Or select an immutable version while using the current installer:
curl -fsSL https://github.com/ming-kang/BUPT_EC/releases/latest/download/install.sh | sudo VERSION=v0.1.6 bash
```

No `--mode` is deliberately the compatible, fully interactive `install` path:
its question order, defaults, and existing-installation defaults remain the
same. It stores the selected value as `RELEASE_VERSION` in
`/etc/bupt-ec/bupt-ec.env` along with the deployment settings.

### Installer modes

| Mode | Intended use | Version selection | TTY and package behavior |
| --- | --- | --- | --- |
| omitted / `--mode=install` | First install or the legacy-compatible interactive entrypoint | explicit `VERSION` → saved `RELEASE_VERSION` → `latest` | TTY required; installs supported packages through apt |
| `--mode=update` | Prompt-free update of an existing deployment | explicit `VERSION` → saved `RELEASE_VERSION` | No TTY or prompts; skips apt and preflights required tools |
| `--mode=reconfigure` | Interactively change deployment configuration | saved `RELEASE_VERSION` only; ignores `VERSION` | TTY required; installs supported packages and reruns the transaction |

Use `--mode=update` for a normal unattended upgrade. The installer accepts
both `--mode=value` and `--mode value`; the remote-script form below uses the
first spelling and must use `bash -s --` so the argument is forwarded to the
downloaded script:

```bash
# Reuse the saved stable version.
curl -fsSL https://github.com/ming-kang/BUPT_EC/releases/latest/download/install.sh | sudo bash -s -- --mode=update
# Pin a stable update or roll back to it explicitly.
curl -fsSL https://github.com/ming-kang/BUPT_EC/releases/latest/download/install.sh | sudo VERSION=v0.1.6 bash -s -- --mode=update
# Change settings interactively while retaining the installed version.
curl -fsSL https://github.com/ming-kang/BUPT_EC/releases/latest/download/install.sh | sudo bash -s -- --mode=reconfigure
```

`update` does not read `/dev/tty`, so a downloaded script is suitable for a
cron/automation-style invocation with closed stdin:

```bash
curl -fsSLo /tmp/bupt-ec-install.sh https://github.com/ming-kang/BUPT_EC/releases/latest/download/install.sh
sudo VERSION=v0.1.6 bash /tmp/bupt-ec-install.sh --mode=update < /dev/null
```

An explicit `VERSION=vX.Y.Z` in update mode is both a normal pinned upgrade and
the supported rollback selector. Without it, update uses the saved version. By
contrast, reconfigure always keeps the saved version even if `VERSION` is set.

If update reports a missing domain, certificate, address, or credential, repair
those settings with interactive `--mode=reconfigure`. If it reports missing
`RELEASE_REPO`/`RELEASE_VERSION`, use the default install mode to establish
release metadata. If it cannot load the env safely, inspect and repair the
root-controlled env ownership, mode, or syntax; if it cannot be repaired, move
it aside and use default install to rebuild it. These failures happen before a
download or snapshot.

The interactive install and reconfigure flows ask for:

- GitHub repository (default `ming-kang/BUPT_EC`)
- domain name
- SSL certificate and private key paths (defaults follow the Let's Encrypt layout)
- BUPT teaching affairs username and password, or an optional token override
- backend listen address (default `127.0.0.1:8080`)

Environment variables can pre-seed or override interactive choices, for example:

```bash
curl -fsSL https://github.com/ming-kang/BUPT_EC/releases/latest/download/install.sh | sudo REPO=ming-kang/BUPT_EC VERSION=v0.1.6 bash
```

`LOG_CALLER` and `READYZ_DIAGNOSTICS` are persisted deployment settings but are
not extra prompts; use an interactive install/reconfigure invocation to change
them. `update` preserves their saved values and only honors `VERSION` as an
override.

## What the installer does

- In `install` and `reconfigure` modes, installs `ca-certificates`, `curl`, `tar`, and `nginx` via apt. `update` never invokes apt; it first verifies the download and transaction tools already exist.
- Creates a dedicated `bupt-ec` system user and group.
- Downloads the release tarball matching the CPU architecture and requires a matching `checksums.txt` entry (install fails if the checksum file is missing or verification fails). Set `SKIP_CHECKSUM=1` only as an explicit break-glass to skip verification.
- Extracts the archive and renders candidate env, systemd, and Nginx files in a root-only staging directory before changing any installed target.
- Snapshots the existing binary, env, systemd unit/enablement, and Nginx site/enablement, then replaces files with same-filesystem atomic renames.
- Installs the binary to `/opt/bupt-ec/bupt-ec`, owned by root so the running service cannot rewrite its own executable. Only `/opt/bupt-ec/run_log` is writable by the service user.
- Writes the complete deployment configuration (including supported `LOG_CALLER` and `READYZ_DIAGNOSTICS` settings) to `/etc/bupt-ec/bupt-ec.env` (mode `0600`, owned by root).
- Installs a hardened systemd unit (`NoNewPrivileges`, `PrivateTmp`, `ProtectHome`, `ProtectSystem=full`, empty capability bounding set, and more) and enables it.
- Writes an Nginx site with HTTP→HTTPS redirect, TLS 1.2/1.3, security headers, and rate limiting on `/api/` (30 requests/minute per IP with a burst of 20). `/api/` uses `proxy_read_timeout 60s`, comfortably above the backend stack (5s cold-wait bound + 15s Go `WriteTimeout`, 60s proxy).
- Validates Nginx, restarts and checks the service, reloads Nginx, and checks `/healthz` when `APP_ADDR` is loopback. A failure restores the snapshot (or removes newly created first-install files), stops any newly started unit, reloads Nginx after site removal, and restores the previous service active/enabled state before the installer exits non-zero.

The installer prints its success message only after all commit validations pass. After installation the site is served at `https://<your-domain>/`.

## Offline or restricted networks (explicit mirrors)

By default the installer downloads only from official GitHub release URLs. It
does **not** auto-select third-party proxies. If GitHub is unreachable, the
installer fails before changing installed files. Configure an explicit mirror
only when you control and trust it.

When you control a trusted mirror (for example on an IPv6-only network that
cannot reach GitHub), copy the matching release assets there and set the base
URL through an interactive install/reconfigure path:

```bash
# Obtain install.sh from a machine that can reach GitHub (or your mirror),
# inspect it, then run on the target host. This is the compatible install path:
sudo VERSION=v0.1.6 DOWNLOAD_BASE_URL=https://your-mirror.example/releases/v0.1.6 bash install.sh
# On an existing host, save a mirror without changing its saved release version.
# The URL below is valid only when the saved RELEASE_VERSION is v0.1.6 and the
# mirrored package/checksum assets are for that same version:
sudo DOWNLOAD_BASE_URL=https://your-mirror.example/releases/v0.1.6 bash install.sh --mode=reconfigure
```

A later prompt-free `--mode=update` reuses the saved mirror and intentionally
ignores one-off configuration overrides, so it cannot silently change its
trusted source. Because a custom base URL does not encode a verifiable package
version, update refuses an explicit `VERSION` that differs from the saved
version while a mirror is configured; run the interactive install path with
that VERSION and an explicitly supplied matching trusted `DOWNLOAD_BASE_URL`
instead. The default install path likewise refuses to change VERSION while
silently inheriting an old mirror.

The mirror directory must contain `bupt-ec-linux-amd64.tar.gz` or
`bupt-ec-linux-arm64.tar.gz` and a `checksums.txt` that lists the package hash
(verification is required unless `SKIP_CHECKSUM=1`). Same-origin checksums prove
download integrity only; they are not independent proof of GitHub publisher
identity if the mirror itself is compromised.

`DOWNLOAD_BASE_URL` must be an absolute **HTTPS** URL with a non-empty host
(path optional). Userinfo (`user:pass@`), query strings, fragments, and empty
hosts are rejected before any download. For a trusted local mirror only, set
`ALLOW_INSECURE_DOWNLOAD_BASE_URL=true` to allow plain **HTTP** (not `file://`,
`ftp://`, or other schemes). curl is restricted to HTTPS-only redirects for
HTTPS sources, or HTTP+HTTPS for the HTTP break-glass path. Logs print a safe
host label only (never credentials or query tokens). A saved
`DOWNLOAD_BASE_URL` from a previous explicit choice is reused on upgrades after
the same validation; the installer never writes a mirror URL discovered by
network probing.

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

For production, also add the hardening directives used by the installer (see
`scripts/installer/30-render.sh::render_systemd_service`; the generated
`scripts/install.sh` contains the released copy).

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

The full production site written by the installer (TLS, security headers, rate
limiting) is in `scripts/installer/30-render.sh::render_nginx_site` and can be
used as a template; `scripts/install.sh` is its generated release artifact.
