# Upgrading

How to update an existing server installation. For first-time setup see [deployment.md](deployment.md).

## Standard update

For an existing deployment, use the explicit noninteractive update mode. It
loads the saved deployment configuration, asks no questions, does not read a
TTY, skips apt package installation, and runs the same verified staging and
transaction path as install.

```bash
# Reuse the saved stable version.
curl -fsSL https://github.com/ming-kang/BUPT_EC/releases/latest/download/install.sh | sudo bash -s -- --mode=update
# Pin a stable update (or select a known version for rollback).
curl -fsSL https://github.com/ming-kang/BUPT_EC/releases/latest/download/install.sh | sudo VERSION=v0.1.6 bash -s -- --mode=update
```

The remote form needs `bash -s -- --mode=update`; placing `--mode` directly on
`bash` does not pass it to a script read from stdin. A downloaded script also
works with no terminal and closed standard input:

```bash
curl -fsSLo /tmp/bupt-ec-install.sh https://github.com/ming-kang/BUPT_EC/releases/latest/download/install.sh
sudo VERSION=v0.1.6 bash /tmp/bupt-ec-install.sh --mode=update < /dev/null
```

| Mode | When to use it | Release selection | Terminal behavior |
| --- | --- | --- | --- |
| omitted / `--mode=install` | Compatible full interactive install flow | `VERSION` → saved version → `latest` | TTY required |
| `--mode=update` | Normal existing-deployment update | `VERSION` → saved version | No TTY and no prompts |
| `--mode=reconfigure` | Change saved deployment settings interactively | Saved version only; ignores `VERSION` | TTY required |

An explicit `VERSION=vX.Y.Z` in update mode is the supported way to select a
specific stable release, whether that is a forward update or a rollback. With
no `VERSION`, update uses the saved `RELEASE_VERSION`. Reconfigure never turns
into an implicit release change: it keeps the saved version and redownloads it
through the normal transaction.

No-mode install remains compatible for operators who want the historical
interactive prompts; on an existing installation it offers saved values as
defaults. Use `--mode=reconfigure` when the goal is to change settings rather
than select a release. Password/token prompts keep the existing secret when
left empty.

If update reports a missing domain, certificate path, listen address, or valid
JW credential combination, run `--mode=reconfigure` from a terminal to repair
the saved configuration. If it reports missing `RELEASE_REPO` or
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

## Upgrading without GitHub access

If the server cannot reach GitHub, save an explicit HTTPS `DOWNLOAD_BASE_URL`
for a mirror you control (and already trust) through interactive install or
`--mode=reconfigure`. Update intentionally ignores a one-off
`DOWNLOAD_BASE_URL` environment override and reuses the saved value, so it
cannot silently change its trust source. The URL must not include credentials,
query parameters, or fragments; invalid saved mirrors fail validation before
download or snapshot. Package and `checksums.txt` are both fetched from that
base under the same curl protocol policy. This is operator-chosen trust, not an
automatic proxy fallback; same-origin checksums verify integrity, not
independent publisher identity. A saved custom base cannot prove that it serves
a newly selected version, so update rejects `VERSION` changes while that mirror
is active. Use interactive install with the target VERSION and a matching
trusted `DOWNLOAD_BASE_URL` to change both deliberately; install will not
silently inherit the old mirror during that version change. See
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

`/healthz` should return `{"status":"ok"}` immediately. `/readyz` returns 200 once the first classroom refresh has succeeded (this may take a few seconds after a restart while the warmup login runs). `/readyz` shows only `status`+`version` by default; set `READYZ_DIAGNOSTICS=1` if you need the full diagnostics body for deeper checks (or read `journalctl -u bupt-ec`).

Confirm the running build is the one you just installed: the `/readyz` body carries a `version` field that must equal the installed tag (for example `v0.1.6`). If it still shows the previous version, the service was not restarted or a different artifact was installed — recheck `systemctl status bupt-ec` and rerun the installer with an explicit `VERSION`.

Then open `https://<your-domain>/` in a browser and confirm the page loads today's data.

## Roll back to an earlier release

Use the current installer in update mode and select the earlier stable tag
explicitly:

```bash
curl -fsSL https://github.com/ming-kang/BUPT_EC/releases/latest/download/install.sh | sudo VERSION=v0.1.2 bash -s -- --mode=update
```

Stable releases are immutable, so a successful transaction installs the exact
previous binary while preserving the saved configuration. If that configuration
must change for the older release, run `--mode=reconfigure` separately; it keeps
the saved version and does not treat a configuration edit as a rollback. This
deliberate version rollback is separate from the installer's automatic recovery
from a failed update.

## Certificate renewal

Certificate renewal is independent of upgrades. If your certificate manager renews files in place (same paths), reload Nginx afterwards:

```bash
sudo systemctl reload nginx
```

Rerunning the installer is only needed when the certificate paths change.
