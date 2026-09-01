# 设计：bupt-ec 运维 CLI、公共元数据与事务化分发

任务：`08-22-bupt-ec-cli`
前置：`08-22-installer-modes` 已归档

## Architecture Overview

CLI 是薄运维入口，release archive 与生成式 Installer 仍是唯一部署引擎：

```text
scripts/bupt-ec-cli.sh
        │ release workflow 注入与 Go binary 同源的 build version
        ▼
bupt-ec-linux-${arch}/bupt-ec-cli
        │ 整个 tarball 由 checksums.txt 校验
        ▼
current/latest install.sh
        │ stage + snapshot + commit + validation/rollback
        ├─ /opt/bupt-ec/bupt-ec             service binary
        ├─ /usr/local/bin/bupt-ec            operations CLI
        ├─ /etc/bupt-ec/deployment.meta      public non-secret metadata
        └─ existing env/systemd/nginx targets
```

Installed CLI command flow:

```text
bupt-ec
├─ update ── safe private-config load ── fetch current/latest install.sh
│                                      └─ VERSION=<target> --mode=update
├─ config ── safe private-config load ── fetch current/latest install.sh
│                                      └─ --mode=reconfigure
├─ config show ── safe private-config load ── fixed registry + redaction
├─ status/version/health ── strict public metadata ── local probes/systemctl
├─ logs ── journalctl
└─ start/stop/restart ── systemctl
```

The generated `scripts/install.sh` remains self-contained. It never sources or
executes the CLI; the CLI is an independently installed release product, not an
Installer runtime dependency.

## Responsibility Boundary

| Capability | Owner |
| --- | --- |
| target release validation, archive download/checksum/extract | generated `install.sh` |
| rendering, snapshot, atomic replacement/removal, rollback | generated `install.sh` |
| interactive configuration | `install.sh --mode=reconfigure` |
| noninteractive deployment update | `install.sh --mode=update` |
| fetching the current/latest bootstrap installer | CLI only |
| command parsing, privilege checks, probes and presentation | CLI only |

The CLI must not implement release archive/checksum handling, package setup,
staging, systemd/Nginx rendering, snapshots, or rollback.

## CLI Source and Entrypoint

Repository source: `scripts/bupt-ec-cli.sh`.
Installed path: `/usr/local/bin/bupt-ec`, root-owned `0755`.

Stable signatures:

```bash
cli_main [command [args...]]
cli_usage
configure_cli_test_root <absolute-root>        # sourced tests only
require_cli_root <command...>
load_private_deployment_config
load_public_deployment_metadata
fetch_current_installer <destination>
run_update [target-version]
run_reconfigure
show_config
show_status
show_version
show_health
show_logs [-f] [-n N]
control_service <start|stop|restart>
```

Like generated `install.sh`, direct execution calls `cli_main`; sourcing defines
functions only. Production paths cannot be overridden through environment
variables. Tests may redirect them only through `configure_cli_test_root` after
sourcing.

No arguments, `-h`, or `--help` print usage and return 0. Unknown commands,
extra arguments, duplicate log flags, invalid `-n`, or malformed update versions
return usage status 2 before privileged or network work.

## Command Contracts

### Privilege matrix

| Command | Root required | Notes |
| --- | :---: | --- |
| `update`, `config`, `config show` | yes | private env is root-only |
| `start`, `stop`, `restart` | yes | mutates systemd state |
| `status`, `version`, `health`, `logs` | no | reads only public metadata/system APIs; journal policy may still deny logs |

Privilege failure names the exact retry, for example
`sudo bupt-ec update v0.3.1`, and occurs before private-file/network/system
side effects.

### Status and probe exit semantics

`health` probes both `http://${APP_ADDR}/healthz` and `/readyz` with bounded
connect/total timeouts, no proxy, and no redirects. It prints each HTTP status
and readiness summary. It returns 0 only when both status codes are 2xx.

`status` prints unit active/enabled state, configured release selector, running
version, and both probe outcomes. It returns 0 only when the unit is active and
both probes are 2xx. A normal post-restart readiness 503 is displayed as
not-ready/degraded and returns nonzero.

`version` prints configured selector, running build version parsed from the
`/readyz` body, and injected CLI version. A 503 readiness response still carries
the running version and is a successful version query when the body is valid;
unreachable/invalid metadata returns nonzero with `unavailable`, never a guessed
version.

### Logs and service controls

`logs` defaults to the latest 50 journal records and supports combinable `-f`
and `-n <positive integer>` exactly once each. It executes `journalctl` with a
fixed unit and propagates journalctl's status. It does not silently sudo.

`start` / `stop` / `restart` delegate one operation to `systemctl` and report
the resulting active state. They do not invoke strict full `status` immediately,
so a successful restart is not misreported as a command failure merely because
readiness warmup is still running.

## Configuration Read Boundaries

### Private deployment config

Path: `/etc/bupt-ec/bupt-ec.env`, root-owned exact `0600`, regular non-symlink,
inside a root-owned directory with no group/other write bits.

The CLI duplicates only the small field registry and the security boundary
needed before it can fetch an Installer. `load_private_deployment_config`:

1. validates directory/file owner, type and mode;
2. creates a mode-`0600` frame under fixed `/tmp` (never caller `TMPDIR`);
3. evaluates the trusted env in an isolated child after unsetting registered
   fields and one-shot controls;
4. emits only the twelve registered fields in strict NUL framing;
5. rejects source output/framing errors, clears state and removes the frame
   without printing values.

This is configuration transport, not a second deployment engine. The parent CLI
process never sources the env, and installed content cannot activate
`SKIP_CHECKSUM`, `ALLOW_INSECURE_DOWNLOAD_BASE_URL`, traps, or CLI dispatch.

`config show` iterates the fixed registry only. `JW_PASSWORD` and `JW_TOKEN` are
always rendered as `***` (including empty values); all other values are
single-line shell-escaped so embedded controls/newlines cannot corrupt terminal
output. It never echoes source stdout/stderr or unknown env statements.

### Public metadata

Path: `/etc/bupt-ec/deployment.meta`, root-owned exact `0644`, regular
non-symlink, in the same secure directory. Exact format and order:

```text
RELEASE_VERSION=<latest-or-vX.Y.Z>
APP_ADDR=<validated-host:port>
```

It is rendered directly from validated `CFG_RELEASE_VERSION` and
`CFG_APP_ADDR`, not copied from private env text. It contains no repo, mirror,
domain, certificate path, JW field, logging/readiness flag, or one-shot control.

`load_public_deployment_metadata` never sources the file. It checks the same
parent ownership/non-writability boundary, exact file owner/mode/type, exactly
one record for each registered key, no extras, and value shape. Missing or
untrusted metadata is an explicit failure; read-only commands never fall back
to the private env and never silently probe `127.0.0.1:8080` for a custom
installation.

## Installer Bootstrap and Version Policy

### Control plane versus target release

The CLI always fetches a current/latest compatible Installer implementation:

| Saved source | Bootstrap Installer URL |
| --- | --- |
| official GitHub | `https://github.com/${RELEASE_REPO}/releases/latest/download/install.sh` |
| explicit mirror | `${DOWNLOAD_BASE_URL}/install.sh` |

A CLI-capable mirror must publish the self-contained `install.sh` beside its
tarball and `checksums.txt`. Missing mirror Installer fails closed without
falling back to GitHub or a third-party source; this preserves the operator's
saved trust boundary.

The requested/saved `VERSION` is passed to that Installer and selects the
archive. It does not select an old Installer implementation. This preserves the
current documented rollback pattern and avoids invoking v0.2.x installers that
lack `--mode`.

Bootstrap download uses a mode-`0700` fixed `/tmp` directory, bounded curl,
protocol restrictions, and cleanup traps. Official downloads are HTTPS-only.
A saved HTTP mirror is accepted only with the same exact one-shot
`ALLOW_INSECURE_DOWNLOAD_BASE_URL=true`; it never widens to other protocols.
The mirror URL came from the protected installed config but is still shape-
checked without printing raw credential-bearing input. The downloaded script
has the same HTTPS/operator-mirror trust model as the existing documented
`curl | bash`; target tarballs remain checksum-verified by Installer.

`config` verifies root and an interactive TTY before bootstrap network work,
then invokes `--mode=reconfigure` and therefore retains the saved target
selector. `update` accepts at most one `latest` or stable `vX.Y.Z` target, with
command-line value over saved selector.

### CLI-bearing release floor

`v0.3.0` is the first CLI-bearing release. `bupt-ec update` rejects any explicit
or saved stable target below `v0.3.0` before curl and prints the current/latest
Installer fallback. CLI-supported update/rollback is therefore `latest` or
stable tags `>= v0.3.0`.

Direct current/latest Installer invocation retains its pre-existing ability to
select v0.2.x. This requires Installer staging to represent two target states:

| Target selector | Expected archive | CLI/metadata candidate action |
| --- | --- | --- |
| `latest` or stable `>= v0.3.0` | `bupt-ec` + `bupt-ec-cli` | install/replace both |
| stable `< v0.3.0` | legacy archive without CLI | remove both |

Thus a direct rollback to v0.2.x leaves the target release internally
consistent and removes the command introduced in v0.3. A failed transaction
restores the previous CLI and metadata.

## Release Composition and Version Injection

Architecture tarball layout becomes:

```text
bupt-ec-linux-${arch}/
  bupt-ec
  bupt-ec-cli
  .env.example
  README.md
  install.sh
```

Top-level published assets remain the two tarballs, `checksums.txt`, and
self-contained `install.sh`; there is no top-level CLI asset. Installer source
fragments and test modules remain repository-only.

`scripts/bupt-ec-cli.sh` contains one explicit build-version marker that reports
`dev` in a checkout. The release composition step validates exactly one marker
and substitutes the same value used by Go `-ldflags`: tag name for tag builds,
`main-<short-sha>` for dry runs. Both architecture packages receive byte-identical
CLI content for a given workflow run. Exact tar member assertions and installer
byte-parity checks remain release gates.

## Staging and Transaction Integration

New production constants:

```text
CLI_FILE=/usr/local/bin/bupt-ec
DEPLOYMENT_METADATA_FILE=/etc/bupt-ec/deployment.meta
CLI_MIN_RELEASE=v0.3.0
```

`stage_release` keeps the existing unambiguous `find ... -name bupt-ec` binary
path and separately searches exact `bupt-ec-cli`. For CLI-bearing targets,
missing CLI fails before snapshot; candidate is root-owned `0755`. For legacy
targets, absence is expected.

`render_deployment_metadata(destination)` reads validated `CFG_*`, writes the
strict two-record format, and sets root ownership / `0644` only for a
CLI-bearing target. Staging writes a protected internal action marker
(`install` or `remove`) so commit does not infer behavior from an accidentally
missing candidate.

`transaction_targets` adds `cli` and `metadata` unconditionally. Existing
`snapshot_installation` / `rollback_installation` iteration then handles prior
presence and first-install absence without CLI-specific rollback code.
`commit_installation` validates the action marker and either atomically installs
both candidates or unlinks both legacy-inapplicable targets, checking every
failure. Any later service/Nginx/health failure uses the existing transaction
rollback path.

Only these transaction-adjacent functions should require behavioral changes:
`stage_release`, `prepare_staging`, `transaction_targets`, and
`commit_installation`, plus a new metadata renderer/version-floor helper.
Record current function hashes before implementation and require all unrelated
transaction function bodies to remain byte-identical.

## Test Architecture

### Dedicated CLI suite

Add `scripts/cli_test.sh` (and focused modules under `scripts/cli_test/` if the
entrypoint would approach 1,000 lines). It sources `scripts/bupt-ec-cli.sh`,
redirects fixed paths through the explicit test seam, and injects mock
`curl`/`systemctl`/`journalctl` behavior.

Coverage:

- help/default/unknown/extra argument parsing;
- root matrix and exact sudo guidance;
- strict public metadata type/owner/mode/directory/shape validation;
- private config isolation, framing failures, one-shot non-activation and secret
  non-disclosure;
- status/version/health success, readiness 503, inactive unit, unreachable
  probes, version extraction and strict exit statuses;
- logs defaults, flag combinations, duplicate/invalid flags and status
  propagation;
- service control dispatch;
- official/mirror bootstrap URL and curl protocol arguments;
- zero-prompt update invocation, reconfigure invocation and target precedence;
- pre-v0.3 rejection before curl;
- `config show` multiline-secret redaction.

### Installer/release integration suite

Extend the generated-installer tests with archives containing separate binary
and CLI members and legacy archives without CLI. Assert:

- exact binary selection remains unchanged;
- CLI-bearing target missing `bupt-ec-cli` fails before snapshot;
- staged/installed CLI `0755` root:root and metadata `0644` root:root;
- successful upgrade replaces CLI/metadata;
- transaction failure restores both;
- failed first install removes both;
- direct legacy rollback removes both, and a later failure restores both;
- unrelated protected transaction function hashes do not drift.

The release dry-run assertion must include `bupt-ec-cli`, verify its injected
version, keep generated installer parity, and prove fragments/tests are absent.

## CI and Local Gates

Keep Taskfile, reusable quality workflow and development docs aligned:

```bash
bash scripts/generate-install.sh --check
find scripts -type f -name '*.sh' -exec bash -c 'for script; do bash -n "$script" || exit 1; done' bash {} +
bash scripts/install_test.sh
bash scripts/cli_test.sh
find scripts -type f -name '*.sh' -exec shellcheck {} +
```

`task installer:check` owns both shell behavior suites and remains part of
`task check`. Workflow edits also require actionlint and a local release-layout
simulation.

## Compatibility and Rollback

- Existing default `curl | bash` install and explicit Installer modes remain
  public and self-contained.
- A successful upgrade from v0.2.x to v0.3.0 installs binary, CLI and metadata
  together.
- CLI update/config never asks update questions; only reconfigure is interactive.
- Direct Installer rollback to pre-v0.3 removes CLI/metadata; CLI itself refuses
  to initiate that transition.
- Reverting this feature in source must include an explicit installed-file
  migration. Merely removing `transaction_targets` entries would leave orphaned
  `/usr/local/bin/bupt-ec` and metadata, so rollback-by-revert is not sufficient
  without that cleanup policy.

## Risks and Mitigations

| Risk | Mitigation |
| --- | --- |
| old target Installer lacks `--mode` | always bootstrap current/latest Installer |
| old archive lacks CLI | explicit floor matrix and transactional remove action |
| non-root command reads secrets | separate two-field `0644` metadata; never fall back to private env |
| metadata and private config drift | render both from the same validated `CFG_*` staging generation |
| private env mutates CLI or leaks source output | isolated strict NUL loader and generic failures |
| service binary confused with CLI | tarball member remains `bupt-ec-cli`; exact separate extraction/tests |
| CLI and Installer path/policy drift | fixed constants documented in both specs, focused cross-contract tests |
| restart returns false failure during warmup | controls report systemctl result; strict readiness remains in status/health |
| release marker is not injected | exact-one marker assertion and package-content test |
