# Upgrading

How to update an existing server installation. For first-time setup see [deployment.md](deployment.md).

## Standard upgrade

Rerun the installer — the same command used for installation.

**Production upgrades** should pin a stable tag (or use GitHub `latest` stable).
Existing installations reuse their saved `RELEASE_VERSION`; older installations
without that field fall back to `nightly` unless the command passes `VERSION`.

Stable:

```bash
curl -fsSL https://github.com/ming-kang/BUPT_EC/releases/latest/download/install.sh | sudo VERSION=latest bash
# or:
curl -fsSL https://github.com/ming-kang/BUPT_EC/releases/download/v0.1.6/install.sh | sudo VERSION=v0.1.6 bash
```

Nightly (edge):

```bash
curl -fsSL https://github.com/ming-kang/BUPT_EC/releases/download/nightly/install.sh | sudo VERSION=nightly bash
```

Existing configuration from `/etc/bupt-ec/bupt-ec.env` is offered back as defaults, so pressing Enter at each prompt keeps the current values. Secrets (password/token) are kept by pressing Enter at their prompts. The installer downloads and verifies the archive, renders every candidate file before touching the installation, snapshots the current targets, then atomically replaces the binary, env, systemd unit, and Nginx site.

The selected release channel or tag is stored as `RELEASE_VERSION`. Rerunning
the installer without `VERSION` reuses it; pass an explicit value to switch
between `latest`, `nightly`, or a pinned `vX.Y.Z` release.

See [CHANGELOG.md](../CHANGELOG.md) for what changed between versions.

## Upgrading without GitHub access

If the server cannot reach GitHub, set an explicit HTTPS `DOWNLOAD_BASE_URL` to
a mirror you control (and already trust). The URL must not include credentials,
query parameters, or fragments; invalid saved mirrors fail validation before
download or snapshot. Package and `checksums.txt` are both fetched from that
base under the same curl protocol policy. This is operator-chosen trust, not an
automatic proxy fallback; same-origin checksums verify integrity, not
independent publisher identity. See
[deployment.md](deployment.md#offline-or-restricted-networks-explicit-mirrors).

## Automatic transaction rollback

After committing the candidates, the installer runs `systemctl daemon-reload`, enables the unit, validates Nginx, restarts and checks `bupt-ec`, reloads Nginx, and probes loopback `/healthz`. It prints success only after these checks pass.

If any commit or validation step fails, the installer exits non-zero and restores the previous binary, env, systemd unit/enablement, and Nginx site/enablement. It snapshots prior service active/enabled state, stops any unit that may have been started during the failed commit, reloads Nginx after restoring or removing sites, and only starts the service again when it was active before the upgrade. A failed first install removes the new target files, stops a newly started unit, and reloads Nginx so no half-installed service or site remains.

Candidate and backup directories are mode `0700`; env candidates, backups, and installed env files are mode `0600`. If automatic rollback itself is incomplete, the error output names a root-only recovery directory containing the snapshot. Preserve that directory until the service is repaired, and do not copy or expose its env file to non-root users.

## Verify the upgrade

```bash
sudo systemctl status bupt-ec
curl -s http://127.0.0.1:8080/healthz
curl -s http://127.0.0.1:8080/readyz | head -c 400; echo
sudo journalctl -u bupt-ec -n 50 --no-pager
```

`/healthz` should return `{"status":"ok"}` immediately. `/readyz` returns 200 once the first classroom refresh has succeeded (this may take a few seconds after a restart while the warmup login runs).

Then open `https://<your-domain>/` in a browser and confirm the page loads today's data.

## Roll back to an earlier release

Rerun the installer pinned to the previous version:

```bash
curl -fsSL https://github.com/ming-kang/BUPT_EC/releases/download/v0.1.2/install.sh | sudo VERSION=v0.1.2 bash
```

Stable releases are immutable, so a successful transaction installs the exact previous binary. Configuration prompts still default to the current values; adjust them if the earlier version requires different settings. This deliberate version rollback is separate from the installer's automatic recovery from a failed upgrade.

## Certificate renewal

Certificate renewal is independent of upgrades. If your certificate manager renews files in place (same paths), reload Nginx afterwards:

```bash
sudo systemctl reload nginx
```

Rerunning the installer is only needed when the certificate paths change.
