# Upgrading

How to update an existing server installation. For first-time setup see [deployment.md](deployment.md).

## Standard update

On a v0.3.0+ installation, use the installed operations CLI. It safely reads
the root-only deployment snapshot, fetches the **current/latest** compatible
Installer, and delegates the verified archive/checksum/staging/transaction path
to that Installer. It asks no questions, does not read a TTY, and skips apt in
update mode.

```bash
# Reuse the saved selector with no prompts (also works with stdin closed).
sudo bupt-ec update < /dev/null
# Select a CLI-bearing stable release explicitly.
sudo bupt-ec update v0.3.1
# Inspect strict readiness/state before or after an update.
bupt-ec status
```

The target argument selects an archive, never an old Installer implementation.
That lets an updated CLI bootstrap fixes in the current Installer while the
Installer continues to verify the selected tarball checksum. `bupt-ec update`
accepts `latest` and stable tags **v0.3.0 or newer** only.

If the CLI is unavailable, or the target is below v0.3.0, use the direct
current-Installer fallback instead:

```bash
curl -fsSL https://github.com/ming-kang/BUPT_EC/releases/latest/download/install.sh | \
  sudo VERSION=v0.2.0 bash -s -- --mode=update
```

The remote form needs `bash -s -- --mode=update`; placing `--mode` directly on
`bash` does not pass it to a script read from stdin. The fallback remains
noninteractive with closed standard input.

| Mode | When to use it | Release selection | Terminal behavior |
| --- | --- | --- | --- |
| omitted / `--mode=install` | Compatible full interactive install flow | `VERSION` → saved version → `latest` | TTY required |
| `--mode=update` | Normal existing-deployment update | `VERSION` → saved version | No TTY and no prompts |
| `--mode=reconfigure` | Change saved deployment settings interactively | Saved version only; ignores `VERSION` | TTY required |

An explicit `VERSION=vX.Y.Z` in direct Installer update mode is the supported
way to select any specific stable release. In the CLI, the same selector is
accepted only for `v0.3.0+`; with no argument it uses saved `RELEASE_VERSION`.
Reconfigure never turns into an implicit release change: it keeps the saved
version and redownloads it through the normal transaction.

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
atomically replaces the binary, env, CLI/public metadata when applicable,
systemd unit, and Nginx site. See [CHANGELOG.md](../CHANGELOG.md) for what
changed between versions.

## Configuration and service controls

Use `sudo bupt-ec config` for an interactive reconfiguration that keeps the
saved release selector, and `sudo bupt-ec config show` for a fixed safe view of
saved fields. Password and token are always displayed as `***`; the CLI never
prints the raw env or source output. Use `sudo bupt-ec start`, `stop`, or
`restart` for controls, and `bupt-ec logs [-f] [-n N]` for the fixed unit's
journal (local journal ACLs still apply).

## Upgrading without GitHub access

If the server cannot reach GitHub, save an explicit HTTPS `DOWNLOAD_BASE_URL`
for a mirror you control (and already trust) through interactive install or
`--mode=reconfigure`. CLI update/config bootstrap expects that mirror to publish
its current self-contained `${DOWNLOAD_BASE_URL}/install.sh` too; absence fails
closed without GitHub fallback. Update intentionally ignores a one-off
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

If any commit or validation step fails, the installer exits non-zero and restores the previous binary, private env, CLI/public metadata, systemd unit/enablement, and Nginx site/enablement. It snapshots prior service active/enabled state, stops any unit that may have been started during the failed commit, reloads Nginx after restoring or removing sites, and only starts the service again when it was active before the upgrade. A failed first install removes the new target files, stops a newly started unit, and reloads Nginx so no half-installed service or site remains.

Candidate and backup directories are mode `0700`; env candidates, backups, and installed env files are mode `0600`. If automatic rollback itself is incomplete, the error output names a root-only recovery directory containing the snapshot. Preserve that directory until the service is repaired, and do not copy or expose its env file to non-root users.

## Verify the upgrade

```bash
bupt-ec status       # strict: nonzero until both probes are ready
bupt-ec version      # configured selector, running build, CLI build
bupt-ec logs -n 50
```

Raw commands remain useful fallback diagnostics when the CLI is unavailable:

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

For a CLI-bearing target (`v0.3.0+`), use the same CLI form:

```bash
sudo bupt-ec update v0.3.0
```

The CLI intentionally rejects `v0.2.x` and earlier before it calls curl. Those
archives never shipped `bupt-ec-cli`, so use the current Installer fallback for
that transition:

```bash
curl -fsSL https://github.com/ming-kang/BUPT_EC/releases/latest/download/install.sh | \
  sudo VERSION=v0.2.0 bash -s -- --mode=update
```

The current Installer understands both archive layouts. A successful legacy
rollback removes `/usr/local/bin/bupt-ec` and `/etc/bupt-ec/deployment.meta` in
the same transaction as the older service; a late failure restores them. Upgrade
again with the current Installer to return to a CLI-bearing release.

Stable releases are immutable, so a successful transaction installs the exact
previous binary while preserving the saved configuration. If that configuration
must change for the older release, run `--mode=reconfigure` separately; it keeps
the saved version and does not treat a configuration edit as a rollback. This
deliberate version rollback is separate from the Installer's automatic recovery
from a failed update.

## Certificate renewal

Certificate renewal is independent of upgrades. If your certificate manager renews files in place (same paths), reload Nginx afterwards:

```bash
sudo systemctl reload nginx
```

Rerunning the installer is only needed when the certificate paths change.
