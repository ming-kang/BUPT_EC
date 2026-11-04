# Upgrading

How to update an existing server installation. For first-time setup see [deployment.md](deployment.md).

## Standard update

Run the current Installer in `--mode=update`. It is prompt-free, does not read
a TTY, skips apt, and delegates the verified
download/checksum/staging/transaction path to the selected release:

```bash
# Reuse the saved selector with no prompts (also works with stdin closed).
sudo bash -s -- --mode=update < /dev/null
```

In practice, fetch the current Installer first and forward the mode:

```bash
curl -fsSL https://github.com/ming-kang/BUPT_EC/releases/latest/download/install.sh | \
  sudo bash -s -- --mode=update
# Select an explicit stable target (normal pinned upgrade or rollback).
curl -fsSL https://github.com/ming-kang/BUPT_EC/releases/latest/download/install.sh | \
  sudo VERSION=v0.2.0 bash -s -- --mode=update
```

The remote form needs `bash -s -- --mode=update`; placing `--mode` directly on
`bash` does not pass it to a script read from stdin. The fallback remains
noninteractive with closed standard input.

The installer modes and their version-selection rules are documented in
[deployment.md](deployment.md#installer-modes); this page covers the update
path specifically.

An explicit `VERSION=vX.Y.Z` in update mode is the supported way to select any
specific stable release, including rolling back to one. With no `VERSION`,
update uses the saved `RELEASE_VERSION`. Reconfigure never turns into an
implicit release change: it keeps the saved version and redownloads it through
the normal transaction.

The no-mode install is the same non-interactive flow; on an existing
installation it uses saved values as defaults. Use `--mode=reconfigure` when
the goal is to change settings rather than select a release; supply only the
variables that change.

If update reports a missing domain, certificate path, listen address, or valid
JW credential combination, rerun with `--mode=reconfigure`, supplying the
corrected values as environment variables. If it reports missing `RELEASE_REPO` or
`RELEASE_VERSION`, run the default install mode to establish that metadata. A
safe-loader failure is different: inspect the env as root, repair its
root-controlled ownership/mode/syntax, or move it aside and use default install
to rebuild it. Do not bypass the failure by sourcing an untrusted env file.

> The `nightly` channel was removed in v0.3.0. A server still carrying
> `RELEASE_VERSION=nightly` fails the installer version check with a message
> pointing at `VERSION=latest`; update with that value both installs the stable
> release and rewrites the saved `RELEASE_VERSION`.

The installer downloads and verifies the archive, renders every candidate file
before touching the installation, snapshots the current targets, then
atomically replaces the binary, env, systemd unit, and Nginx site. See
[CHANGELOG.md](../CHANGELOG.md) for what changed between versions.

## Configuration and service controls

Use `--mode=reconfigure` for a configuration change that keeps the saved
release selector (see [deployment.md](deployment.md#installer-modes)). For
service controls and logs use plain systemctl/journalctl:

```bash
sudo systemctl restart bupt-ec
sudo systemctl status bupt-ec
sudo journalctl -u bupt-ec -n 50 --no-pager
```

The private env stays root-only; edit it only through the installer or, as
root, by hand followed by an explicit service restart.

## Upgrading without GitHub access

If the server cannot reach GitHub, save an explicit HTTPS `DOWNLOAD_BASE_URL`
for a mirror you control (and already trust) through install mode or
`--mode=reconfigure`. Update intentionally ignores a one-off
`DOWNLOAD_BASE_URL` environment override and reuses the saved value, so it
cannot silently change its trust source. The URL must not include credentials,
query parameters, or fragments; invalid saved mirrors fail validation before
download or snapshot. Package and `checksums.txt` are both fetched from that
base under the same curl protocol policy. This is operator-chosen trust, not an
automatic proxy fallback; same-origin checksums verify integrity, not
independent publisher identity. A saved custom base cannot prove that it serves
a newly selected version, so update rejects `VERSION` changes while that mirror
is active. Use install mode with the target VERSION and a matching
trusted `DOWNLOAD_BASE_URL` to change both deliberately; install will not
silently inherit the old mirror during that version change. See
[deployment.md](deployment.md#offline-or-restricted-networks-explicit-mirrors).

## Automatic transaction rollback

After committing the candidates, the installer runs `systemctl daemon-reload`, enables the unit, validates Nginx, restarts and checks `bupt-ec`, reloads Nginx, and probes loopback `/healthz`. It prints success only after these checks pass.

If any commit or validation step fails, the installer exits non-zero and restores exactly what it snapshotted before the commit: the binary, private env, systemd unit, managed Nginx site, and both enablement links (each restored if it existed, removed if it did not). Rollback stops any unit started during the failed commit, restarts the service only if it was active before, and reloads Nginx after a successful `nginx -t`. A failed first install therefore leaves no new files, links, or running service behind.

The snapshot is a persistent journal at `/var/lib/bupt-ec/transaction` (mode `0700`; the env backup is `0600`). If the installer is killed or the host loses power mid-commit, the next installer run detects the journal before doing anything else and restores the previous state automatically. If automatic rollback itself is incomplete, the journal is kept and the error output names it; rerunning the installer retries the restore. Do not copy or expose its env file to non-root users.

## Verify the upgrade

```bash
sudo systemctl status bupt-ec
curl -s http://127.0.0.1:8080/healthz
curl -s http://127.0.0.1:8080/readyz | head -c 400; echo
sudo journalctl -u bupt-ec -n 50 --no-pager
```

`/healthz` should return `{"status":"ok"}` immediately. `/readyz` returns 200 once the first classroom refresh has succeeded (this may take a few seconds after a restart while the first login runs); its body is deliberately minimal (`status` + `version`). For deeper runtime checks read the structured logs (`journalctl -u bupt-ec`), which always carry the same diagnostics with a `log_id`.

Confirm the running build is the one you just installed: the `/readyz` body carries a `version` field that must equal the installed tag (for example `v0.1.6`). If it still shows the previous version, the service was not restarted or a different artifact was installed — recheck `systemctl status bupt-ec` and rerun the installer with an explicit `VERSION`.

Then open `https://<your-domain>/` in a browser and confirm the page loads today's data.

## Roll back to an earlier release

Rollback is the same update form with an explicit stable target:

```bash
curl -fsSL https://github.com/ming-kang/BUPT_EC/releases/latest/download/install.sh | \
  sudo VERSION=v0.2.0 bash -s -- --mode=update
```

An explicit `VERSION` is the supported rollback selector; the installer
downloads the matching archive and verifies its checksum like any update.

Stable releases are immutable, so a successful transaction installs the exact
previous binary while preserving the saved configuration. If that configuration must change for the older release, run `--mode=reconfigure` separately; it keeps the saved version and does not treat a configuration edit as a rollback. This deliberate version rollback is separate from the Installer's automatic recovery from a failed update.

## Certificate renewal

Certificate renewal is independent of upgrades. If your certificate manager renews files in place (same paths), reload Nginx afterwards:

```bash
sudo systemctl reload nginx
```

Rerunning the installer is only needed when the certificate paths change.
