# Upgrading

How to update an existing server installation. For first-time setup see [deployment.md](deployment.md).

## Standard upgrade

Rerun the installer — the same command used for installation:

```bash
curl -fsSL https://github.com/ming-kang/BUPT_EC/releases/download/nightly/install.sh | sudo bash
```

Existing configuration from `/etc/bupt-ec/bupt-ec.env` is offered back as defaults, so pressing Enter at each prompt keeps the current values. Secrets (password/token) are kept by pressing Enter at their prompts. The installer downloads the new binary, replaces `/opt/bupt-ec/bupt-ec`, rewrites the systemd unit and Nginx site, and restarts the service.

By default this installs the rolling `nightly` prerelease (the freshest `main` build). To upgrade to a specific stable release instead:

```bash
curl -fsSL https://github.com/ming-kang/BUPT_EC/releases/latest/download/install.sh | sudo VERSION=v0.1.3 bash
```

See [CHANGELOG.md](../CHANGELOG.md) for what changed between versions.

## Verify the upgrade

```bash
sudo systemctl status bupt-ec
curl -s http://127.0.0.1:8080/healthz
curl -s http://127.0.0.1:8080/readyz | head -c 400; echo
sudo journalctl -u bupt-ec -n 50 --no-pager
```

`/healthz` should return `{"status":"ok"}` immediately. `/readyz` returns 200 once the first classroom refresh has succeeded (this may take a few seconds after a restart while the warmup login runs).

Then open `https://<your-domain>/` in a browser and confirm the page loads today's data.

## Rollback

Rerun the installer pinned to the previous version:

```bash
curl -fsSL https://github.com/ming-kang/BUPT_EC/releases/latest/download/install.sh | sudo VERSION=v0.1.2 bash
```

Stable releases are immutable, so this restores the exact previous binary. Configuration in `/etc/bupt-ec/bupt-ec.env` is not versioned; if a new version introduced config changes you also reverted, adjust the prompts accordingly.

## Certificate renewal

Certificate renewal is independent of upgrades. If your certificate manager renews files in place (same paths), reload Nginx afterwards:

```bash
sudo systemctl reload nginx
```

Rerunning the installer is only needed when the certificate paths change.
