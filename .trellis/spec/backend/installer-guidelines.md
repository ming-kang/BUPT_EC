# Installer Guidelines

These source-backed contracts own installer entry modes, persisted deployment
configuration, generated source/release assets, staging, atomic commit, and
rollback behavior. Read this file before changing `scripts/installer/`,
`scripts/generate-install.sh`, generated `scripts/install.sh`, installer tests,
or installer-related Taskfile/workflow/release commands.

## Scenario: Installer Release Selection

### 1. Scope / Trigger

Apply this contract whenever installer entrypoints, mode selection, release
versions or URLs, persisted deployment metadata, generated installer sources, or
release assets change. Stable `vX.Y.Z` tags are the only release channel. The
repository may maintain modular Bash sources, but operators always receive one
self-contained `scripts/install.sh` asset.

### 2. Signatures

```bash
parse_mode [--mode=<install|update|reconfigure>]
parse_mode [--mode <install|update|reconfigure>]
main [--mode=<install|update|reconfigure>]
capture_invocation_overrides
load_current_config
collect_config_interactive <install|reconfigure>
adopt_current_config
require_update_tools
resolve_release_version <explicit-version> <saved-version>
validate_version <version>
resolve_download_base_url <repo> <version> <override-url>
```

`main` must call `parse_mode "$@"` before the root, TTY, config-load, download,
or transaction paths. The parser accepts no positional arguments and rejects a
missing, duplicate, or unknown `--mode` before any of those side effects.

### 3. Contracts

- Omitting `--mode` means `install`, preserving the historical fully interactive
  installer entrypoint and prompt/default behavior. `--mode=install` is the
  equivalent explicit spelling.
- All modes require root. Only `install` and `reconfigure` require a readable
  TTY; `update` must neither open nor read `/dev/tty` and must work with stdin
  redirected from `/dev/null`.

  | Mode | Configuration and interaction | Release version | Prerequisites |
  | --- | --- | --- | --- |
  | `install` (default) | Full interactive collection. For non-version fields, explicit invocation environment wins over saved values, then hard-coded defaults. | explicit `VERSION` → saved `RELEASE_VERSION` → `latest` | Runs the supported apt package setup, then the common deployment path. |
  | `update` | Existing safely loaded configuration is required; it asks no questions and accepts only `VERSION` as a deployment override. | explicit `VERSION` → saved `RELEASE_VERSION`; no saved metadata is an error | Never calls `install_packages`; before a session, download, or snapshot it checks `curl`, `tar`, `sha256sum`, `install`, `systemctl`, and `nginx`. It still keeps idempotent user creation and the common transaction path. |
  | `reconfigure` | Existing safely loaded installation and an interactive TTY are required. It recollects configuration using invocation value → saved value → default precedence. | exactly saved `RELEASE_VERSION`; ignore `VERSION` | Runs the supported apt package setup, redownloads that version, and uses the common transaction path. |

- `DEPLOYMENT_CONFIG_KEYS` is the twelve-field persisted deployment contract, in
  this order: `RELEASE_REPO`, `RELEASE_VERSION`, `DOMAIN`, `SSL_CERT`,
  `SSL_KEY`, `JW_USERNAME`, `JW_PASSWORD`, `JW_TOKEN`, `APP_ADDR`,
  `DOWNLOAD_BASE_URL`, `LOG_CALLER`, and `READYZ_DIAGNOSTICS`. `CURRENT_*` is
  the safe installed snapshot, `OVERRIDE_*` records invocation presence/value,
  and validated `CFG_*` is the only configuration consumed by rendering and
  staging. `REPO` remains a compatibility alias for an invocation release
  repository; `VERSION` is a one-shot release selector, not a persisted field.
- Capture invocation environment before loading the installed env. For
  `install`/`reconfigure`, an explicitly set empty value still takes precedence
  over a saved/default value. `update` deliberately copies saved deployment
  fields and ignores invocation overrides other than `VERSION`, so it cannot
  become a silent reconfiguration.
- `ALLOW_INSECURE_DOWNLOAD_BASE_URL`, `SKIP_CHECKSUM`, and other execution
  switches are never configuration fields and must not be persisted.
  `SKIP_CHECKSUM` remains an exact `=1` break-glass; values such as `true` must
  not widen the checksum bypass.
- First install without a selected or saved version resolves to `latest`. Valid
  versions are `latest` or `vMAJOR.MINOR.PATCH`. `nightly` is rejected rather
  than remapped; a saved `nightly` must fail with the `VERSION=latest` recovery
  instruction so an operator explicitly chooses the stable channel.
- `latest` maps to `/releases/latest/download`; a stable tag maps to
  `/releases/download/<version>`. Official GitHub is the only automatic source.
  A saved or supplied `DOWNLOAD_BASE_URL` must be normalized as an absolute
  HTTPS URL (HTTP only with `ALLOW_INSECURE_DOWNLOAD_BASE_URL=true`), without
  userinfo, query, fragment, whitespace, semicolons, or an empty host. It is an
  operator-trusted mirror, not an inferred fallback; logs must not reveal a raw
  credential-bearing URL. Because that URL carries no independently verifiable
  version mapping, update must reject an explicit `VERSION` different from the
  saved version while a custom base is active; the operator changes both only
  through interactive install with an explicitly supplied matching trusted
  base. Default install also refuses to change VERSION while silently inheriting
  a saved mirror.
- `scripts/installer/` is the maintainable source layout, ordered explicitly by
  `scripts/generate-install.sh`. The generated, tracked `scripts/install.sh`
  is the test and release artifact; do not hand-edit it or make a target host
  source fragments at runtime. `bash scripts/generate-install.sh --check` is a
  required drift gate.

### 4. Validation & Error Matrix

| Condition | Required result |
| --- | --- |
| no `--mode` | select interactive `install` compatibility mode |
| `--mode=<value>` or `--mode <value>` | accept only `install`, `update`, or `reconfigure` |
| missing, duplicate, unknown mode, or positional argument | non-zero usage failure before root/TTY/config/download work |
| `install` or `reconfigure` without a TTY | non-zero interactive-input failure |
| `update` with stdin redirected from `/dev/null` and valid saved config | no prompt/TTY access; proceed through validation and transaction |
| `update` without valid saved release metadata or deployment fields | non-zero before download/snapshot; direct the operator to `--mode=reconfigure` when repairable, or `--mode=install` when metadata must be rebuilt |
| `reconfigure` without an existing saved version | non-zero; do not turn configuration work into an implicit upgrade |
| `VERSION=vX.Y.Z` in `update` with official GitHub source | download that stable version, enabling both explicit update and rollback |
| explicit update version differs from saved version while custom mirror is active | non-zero before download; require interactive install with matching version/base |
| default install changes saved version while only inheriting a custom mirror | non-zero; require explicit matching `DOWNLOAD_BASE_URL` (an explicit empty value selects official GitHub) |
| `VERSION=nightly` or saved `RELEASE_VERSION=nightly` | non-zero validation failure naming `VERSION=latest`; do not suggest reconfigure, which preserves the invalid version |
| update tool missing | non-zero before session/download/snapshot; no apt invocation, with install-mode recovery guidance |
| GitHub unreachable with no configured mirror | non-zero before download/snapshot; never choose a third-party host |
| HTTP mirror without explicit insecure opt-in, non-HTTP(S) URL, userinfo/query/fragment/empty host | non-zero validation failure without leaking a secret URL |
| generated artifact differs from the ordered fragments | `generate-install.sh --check` fails and tells the maintainer to regenerate |

### 5. Good / Base / Bad Cases

- Good: an existing host is updated without prompts with
  `curl -fsSL <latest-install-url> | sudo VERSION=v0.1.6 bash -s -- --mode=update`.
  The explicit stable version is also the supported rollback selector.
- Base: `curl -fsSL <latest-install-url> | sudo bash` retains the original
  interactive `install` behavior; with no version input or saved value it
  selects `latest`.
- Bad: document `--mode=update` as an interactive configuration editor, allow
  it to run apt, or let `VERSION` change `reconfigure`'s saved release.

### 6. Tests Required

Run the generated artifact and every maintained shell source, not only
`scripts/*.sh` at the top level:

```bash
bash scripts/generate-install.sh --check
find scripts -type f -name '*.sh' -exec bash -c 'for script; do bash -n "$script" || exit 1; done' bash {} +
bash scripts/install_test.sh
bash scripts/cli_test.sh
bash scripts/release_layout_test.sh
find scripts -type f -name '*.sh' -exec shellcheck {} +
```

`task installer:check` runs that sequence and `task check` includes it (while
intentionally omitting the fresh-build frontend bundle-size gate). CI runs the
same generator → syntax → behavior → ShellCheck order, and release runs the
generator drift check again before composing assets. Behavior coverage must
include parser failures, default-mode prompt compatibility, TTY/no-TTY behavior,
update package preflight, explicit version update/rollback, reconfigure's fixed
version, and safe mirror/version policy. Search README and `docs/` for commands
whose mode or version claim no longer matches this table.

### 7. Wrong vs Correct

#### Wrong

```bash
curl -fsSL https://github.com/org/repo/releases/latest/download/install.sh | \
  sudo bash --mode=update
```

`bash` consumes `--mode=update` as its own option rather than passing it to a
script supplied on stdin.

#### Correct

```bash
curl -fsSL https://github.com/org/repo/releases/latest/download/install.sh | \
  sudo VERSION=v0.1.6 bash -s -- --mode=update
```

`-s --` makes Bash read the installer from stdin and forwards the mode after
`--`; this update needs no TTY.

## Scenario: Installed Operations CLI

### 1. Scope / Trigger

Apply this contract when changing `scripts/bupt-ec-cli.sh`, public deployment
metadata, CLI packaging, CLI behavior tests, or commands documented for an
installed host. The Shell CLI is an operations dispatcher only; it delegates
archive/checksum/staging/transaction/rollback work to the current generated
Installer.

### 2. Signatures

```bash
cli_main [command [args...]]
configure_cli_test_root <absolute-root>       # sourced tests only
load_private_deployment_config
load_public_deployment_metadata
show_status
show_version
show_health
show_logs [-f] [-n <positive-integer>]
run_update [latest|vX.Y.Z]
run_reconfigure
```

Installed command surface:

```text
bupt-ec update [VERSION]
bupt-ec status | version | health
bupt-ec logs [-f] [-n N]
bupt-ec start | stop | restart
bupt-ec config [show]
bupt-ec -h | --help
```

### 3. Contracts

- The source is `scripts/bupt-ec-cli.sh`; Installer installs it as
  `/usr/local/bin/bupt-ec`, root:root `0755`. It has one `dev` build-version
  marker. Release packaging substitutes the same tag or `main-<short-sha>`
  injected into Go, exactly once, without changing the source checkout.
- `update`, `config`, `config show`, and service controls require root before
  private-file/network/systemd work and print a safe `sudo bupt-ec ...` retry.
  `status`, `version`, `health`, and `logs` do not silently sudo.
  Parser/help failures return 2 before those side effects; no argument/help
  returns 0. `logs` accepts `-f` and `-n <positive integer>` at most once.
- Root-only private config loading repeats only the transport/security boundary
  needed to bootstrap Installer: strict root-owned directory and regular exact
  `0600` env, isolated child evaluation, fixed-NUL framing of the twelve-field
  registry, protected `/tmp` frame cleanup, and no source stdout/stderr/trap or
  secret disclosure. `config show` iterates that fixed registry, always renders
  `JW_PASSWORD` and `JW_TOKEN` as `***`, and shell-escapes other one-line
  output. The CLI parent never sources the env or imports one-shot controls.
- Non-root `status`, `version`, and `health` use only
  `/etc/bupt-ec/deployment.meta`, never the private env or guessed defaults.
  The metadata must be root-owned exact `0644`, regular/non-symlink in the same
  secure directory, and exactly ordered `RELEASE_VERSION`/`APP_ADDR` records.
  Missing, malformed, or untrusted metadata fails explicitly.
- Local probes use metadata `APP_ADDR`, bounded curl, no proxy, and no redirect.
  `health` succeeds only when `/healthz` and `/readyz` are 2xx; `status` also
  requires an active unit. A readiness 503 is printed as degraded/not-ready and
  is nonzero for both. `version` may report a valid build version from a 503
  readyz response, but unreachable/malformed output is unavailable/nonzero.
- `update` and `config` fetch a **current/latest** Installer implementation:
  official URL is `https://github.com/${RELEASE_REPO}/releases/latest/download/install.sh`;
  a saved validated mirror is `${DOWNLOAD_BASE_URL}/install.sh` and fails closed
  if absent. The requested `VERSION` selects its archive, never an old
  Installer. `config` validates TTY before bootstrap and invokes
  `--mode=reconfigure`; update invokes `VERSION=<target> --mode=update` without
  prompts.
- CLI-selected `latest` and stable `>=v0.3.0` are supported. Any stable target
  below `v0.3.0` is rejected before curl with current/latest Installer fallback
  guidance. Direct invocation of that current Installer still supports legacy
  targets and transactionally removes CLI/metadata through the Installer action
  contract.

### 4. Validation & Error Matrix

| Condition | Required result |
| --- | --- |
| no command, `-h`, or `--help` | print help and return 0 |
| unknown command, extra argument, duplicate log flag, or invalid `-n`/VERSION | return 2 before root/file/network/systemd work |
| non-root mutation/private-config command | return nonzero with exact safe `sudo bupt-ec ...` retry before side effects |
| private env has unsafe parent/type/owner/mode, source output, or malformed frames | clear values, remove protected frame, return generic error without secrets |
| public metadata is missing, unsafe, reordered, duplicated, or has extra keys | fail explicitly; never read private env or guess `APP_ADDR` |
| `/healthz` or `/readyz` is non-2xx/unreachable | `health` nonzero; `status` nonzero, with degraded/unavailable output |
| readyz returns 503 with a valid version body | `version` may report the build; strict `health`/`status` still fail |
| explicit/saved stable target is below `v0.3.0` | reject before curl and show current/latest Installer fallback |
| saved mirror Installer is missing or its URL is invalid | fail closed without GitHub/third-party fallback or raw URL disclosure |
| `config` has no interactive TTY | fail before bootstrap network work |
| valid CLI-bearing target | bootstrap current/latest Installer and pass target through `VERSION` to `--mode=update` |

### 5. Good / Base / Bad Cases

- Good: a root operator runs `bupt-ec update v0.3.1`; the CLI safely loads only
  registered private fields, fetches the current Installer from the saved trust
  source, and delegates a zero-prompt update while the Installer verifies and
  transactionally commits the archive.
- Base: a non-root operator runs `bupt-ec version`; strict two-field metadata and
  a 503 readyz body distinguish saved `latest`, running `v0.3.0`, and the CLI
  build without exposing the private env.
- Bad: source `/etc/bupt-ec/bupt-ec.env` in the CLI parent, silently probe
  `127.0.0.1:8080` after metadata failure, download the target-old Installer,
  or allow `bupt-ec update v0.2.0` to leave a newer CLI beside an older service.

### 6. Tests Required

`scripts/cli_test.sh` must use the sourced-only path seam and mocks to cover
parsing/root ordering, secret-safe loading/redaction, strict metadata rejection,
strict readiness exits, logs/control status propagation, official/mirror
bootstrap URLs and protocol policy, no-TTY update/reconfigure behavior, and the
pre-v0.3 no-curl boundary. `scripts/release_layout_test.sh` must verify stable
and `main-<sha>` version injection, architecture package parity, exact tar
members, generated Installer parity, and the unchanged top-level asset set.

### 7. Wrong vs Correct

#### Wrong

```bash
# Target v0.2.0's Installer predates --mode=update.
curl -fsSL "https://github.com/${repo}/releases/download/v0.2.0/install.sh" | \
  VERSION=v0.2.0 bash -s -- --mode=update
```

#### Correct

```bash
# The current control plane selects the older archive. The installed CLI itself
# refuses pre-v0.3; this direct Installer form is the documented legacy fallback.
curl -fsSL "https://github.com/${repo}/releases/latest/download/install.sh" | \
  sudo VERSION=v0.2.0 bash -s -- --mode=update
```

## Scenario: Transactional Installer Commit and Rollback

### 1. Scope / Trigger

Apply this contract whenever `scripts/installer/`,
`scripts/generate-install.sh`, generated `scripts/install.sh`, release staging,
installed paths, persisted configuration, systemd/Nginx rendering, health
validation, or rollback behavior changes. The installer runs as root and
updates a live service, so generated-source drift, secret handling, and a
partially applied transaction are production correctness failures.

### 2. Signatures

```bash
configure_installer_test_root <absolute-root>   # sourced tests only
load_current_config
render_env_file <destination>                   # reads validated CFG_*
render_systemd_service <destination>
render_nginx_site <destination>                 # reads validated CFG_*
prepare_staging <archive> <work-dir> <staging-dir> # reads validated CFG_*
snapshot_installation <backup-dir>
atomic_install_file <source> <target> <mode> <owner>
atomic_install_symlink <link-target> <target>
commit_installation <staging-dir> <app-addr>
rollback_installation <backup-dir>
perform_install_transaction <staging-dir> <backup-dir> <app-addr>
```

The stable path-only APIs are `render_env_file(destination)`,
`render_nginx_site(destination)`, and
`prepare_staging(archive, work, staging)`. Configuration is not fanned out
through positional arguments; those helpers read the validated `CFG_*` contract.

### 3. Contracts

- `scripts/generate-install.sh` uses its explicit ordered fragment list to
  write executable, tracked `scripts/install.sh` atomically. The generated file
  remains the sole self-contained installer in release tarballs and as the
  top-level release asset. Fragments and `scripts/installer_test/` modules are
  repository-only inputs and must never be copied as runtime companions. The
  independently packaged `bupt-ec-cli` tar member is an installed product, not
  an Installer runtime companion: generated `install.sh` must never source or
  execute it.
- Rendering writes the complete twelve-field `CFG_*` contract in the registry
  order: `RELEASE_REPO`, `RELEASE_VERSION`, `DOMAIN`, `SSL_CERT`, `SSL_KEY`,
  `JW_USERNAME`, `JW_PASSWORD`, `JW_TOKEN`, `APP_ADDR`,
  `DOWNLOAD_BASE_URL`, `LOG_CALLER`, and `READYZ_DIAGNOSTICS`. Values are
  shell-quoted; the env candidate and installed env are root-owned mode `0600`.
  One-shot `ALLOW_INSECURE_DOWNLOAD_BASE_URL` and `SKIP_CHECKSUM` values must
  never be rendered or reloaded as deployment configuration.
- `load_current_config` treats installed env as root-controlled input, not as a
  generic caller environment file. It accepts only a regular, non-symlink env
  with the expected root owner and exact `0600` mode, in a root-owned config
  directory that is not group/other writable or a symlink. It evaluates that
  trusted file in an isolated child, frames only registered values into a
  protected temporary file, returns them into `CURRENT_*`, and clears/fails
  without echoing secrets if ownership, mode, syntax, or framing is unsafe. This isolation prevents a
  trusted env's contents from changing parent-process one-shot controls; it is
  not permission to trust an operator-uncontrolled file. The same directory
  check runs even when no env exists, and the transaction temporarily uses
  umask `022` (restoring the caller value afterward) so a newly created fixed
  config directory is not group/other writable.
- `install` and `reconfigure` retain package installation. `update` skips apt
  entirely and runs its required-tool preflight before session creation,
  download, or snapshot; all modes retain idempotent service-user creation and
  the same download → staging → transaction path.
- CLI-bearing releases (`latest` or stable `>=v0.3.0`) contain a separate
  regular `bupt-ec-cli` archive member. Staging must fail before snapshot if it
  is absent, render `/etc/bupt-ec/deployment.meta` from validated
  `CFG_RELEASE_VERSION` and `CFG_APP_ADDR`, and write an exact `install` action.
  Metadata is root-owned `0644`, regular/non-symlink, and exactly two ordered
  lines: `RELEASE_VERSION=<latest|vX.Y.Z>` and `APP_ADDR=<validated-host:port>`.
  It contains no private env fields. Stable legacy targets below `v0.3.0` are
  still accepted by direct current Installer invocation; their archives omit
  the CLI and staging writes a strict `remove` action instead.
- `transaction_targets` has exactly eight roles: binary, private env, CLI,
  public metadata, systemd unit/link, and Nginx site/link. Commit validates the
  protected `install|remove` action. `install` atomically writes
  `/usr/local/bin/bupt-ec` root:root `0755` and metadata root:root `0644`;
  `remove` deletes both. Snapshot and rollback iterate the same registry, so
  first-install cleanup and late legacy rollback restore need no CLI-specific
  snapshot/restore algorithm.
- Transaction ordering is fixed and must not be interleaved:

  1. Parse mode and validate root/TTY/configuration, certificates, release
     selection, and mode-specific prerequisites.
  2. Download the architecture archive and verify its `checksums.txt` entry.
  3. Extract and render every candidate under a mode-`0700` staging directory;
     env is root-owned mode `0600`, CLI action is protected mode `0600`, and
     CLI-bearing metadata is root-owned mode `0644`. Generated Nginx `/api/`
     `proxy_read_timeout` remains 60s (SPA `/` may remain 30s).
  4. Snapshot binary, env, CLI, metadata, systemd unit + enabled link, and
     Nginx site + enabled link, plus runtime state for prior service/site presence,
     enablement, and activity. The mode-`0700` backup records absent and
     present targets; env, manifest, and runtime state are mode `0600`.
  5. Copy each candidate to `<target>.new.$$` in the target directory, set
     owner/mode, then use `mv -T` for same-filesystem atomic replacement.
  6. Run daemon reload, unit enablement, `nginx -t`, service restart,
     `is-active`, Nginx reload, and loopback `/healthz` retry validation.
  7. On success remove the backup before reporting success. On failure stop any
     currently active unit, restore present targets/remove originally absent
     targets, reload daemons, reconcile prior enabled/disabled and
     active/inactive state (never start a previously inactive unit), validate
     and reload Nginx even after first-install site removal, and preserve
     root-only recovery files if rollback is incomplete.

- Production paths remain fixed constants. Tests may redirect them only by
  sourcing the generated script and calling `configure_installer_test_root`.
  The transaction core does not branch on mode. Architecture tarballs contain
  exactly `bupt-ec`, `bupt-ec-cli`, `.env.example`, `README.md`, and generated
  `install.sh`; top-level published assets remain the two tarballs,
  `checksums.txt`, and self-contained `install.sh` only.

### 4. Validation & Error Matrix

| Condition | Required result |
| --- | --- |
| source fragments and generated `scripts/install.sh` differ | drift check fails before tests, CI assets, or release composition |
| installed env is a symlink, has unsafe parent ownership/mode, wrong owner/mode, malformed output, or unsafe syntax | reject it before deployment work; clear current state and do not print credentials/tokens |
| update lacks a required tool | fail before session/download/snapshot, never call apt, and give recovery guidance |
| checksum download/entry/hash failure | non-zero; installed targets byte-identical |
| archive missing `bupt-ec`, CLI-bearing archive missing `bupt-ec-cli`, malformed action, or candidate render failure | non-zero; snapshot/commit not entered or rollback leaves targets unchanged |
| snapshot copy/manifest failure | non-zero; transaction inactive; installed targets unchanged |
| atomic write, daemon reload, enable, or `nginx -t` failure | restore every recorded target/existence state |
| restart, `is-active`, reload, or loopback health failure | restore files; stop current unit; restore prior active/enabled state |
| first-install commit failure | remove newly created transaction targets; stop new unit; reload Nginx |
| previously inactive/disabled upgrade failure | restore files; leave service inactive/disabled |
| rollback command failure | non-zero; preserve and print root-only recovery directory |
| all validations pass | remove backup; clear transaction state; print success |
| non-loopback `APP_ADDR` | explicitly log that direct health probing is skipped |

### 5. Good / Base / Bad Cases

- Good: a noninteractive update first validates its saved configuration and
  tools, stages a complete twelve-field env plus candidates, snapshots every
  target, atomically commits, passes validation, removes the backup, then
  reports success.
- Base: a first install records every target as absent; a failed validation
  removes all newly created transaction targets and does not restart a
  nonexistent old service.
- Bad: add a renderer/stager configuration positional argument, write
  `/etc/bupt-ec/bupt-ec.env` before checksum verification, publish an installer
  that needs a fragment beside it, or leave a new binary after `nginx -t` or
  health validation fails.

### 6. Tests Required

Run the generated asset, all fragments, and all test modules:

```bash
bash scripts/generate-install.sh --check
find scripts -type f -name '*.sh' -exec bash -c 'for script; do bash -n "$script" || exit 1; done' bash {} +
bash scripts/install_test.sh
bash scripts/cli_test.sh
bash scripts/release_layout_test.sh
find scripts -type f -name '*.sh' -exec shellcheck {} +
```

The behavior suite must source generated `scripts/install.sh` into an explicit
temporary root with mocked `curl`, `chown`, `systemctl`, and `nginx`, and assert:

- generated-artifact drift, parser/mode behavior, no-TTY update, zero prompt/
  apt update, tool preflight, and reconfigure version preservation;
- all twelve configuration fields round-trip, `LOG_CALLER` and
  `READYZ_DIAGNOSTICS` persist, one-shot flags do not, and unsafe env loading
  fails without secret disclosure;
- missing/invalid checksums, missing binary, render failure, and snapshot copy
  failure leave old targets unchanged;
- Nginx, restart, and health failures restore file content, modes, symlink
  targets, and attempt the old-service restart where one existed;
- first-install rollback removes all transaction targets; incomplete rollback
  preserves a mode-`0700` recovery directory and mode-`0600` env backup; a
  successful upgrade replaces every target and clears the backup; and legacy
  removal late failure restores the prior CLI and metadata.
- `scripts/cli_test.sh` sources the CLI through its explicit test seam and
  covers parser/root gates, safe private loading/redaction, strict public
  metadata, probes/readiness exits, journal/service dispatch, latest Installer
  bootstrap, mirror policy, reconfigure TTY, and pre-v0.3 no-curl rejection.

CI must run the same recursive syntax/ShellCheck gate. Release must run
`generate-install.sh --check` immediately before packaging, inject the same
stable tag or `main-<short-sha>` value used by Go `-ldflags` into the one CLI
build-version marker, assert each tarball's exact documented file list and
Installer byte parity, and verify both architecture CLI bytes match. The local
release-layout suite exercises stable and `main` injection plus checksum/layout
assertions.

### 7. Wrong vs Correct

#### Wrong

```bash
prepare_staging "$archive" "$work_dir" "$staging_dir" "$repo" "$version"
```

Configuration fan-out makes a field addition or reordered call silently change
what later parameters mean.

#### Correct

```bash
# CFG_* has already been selected and validated.
prepare_staging "$archive" "$work_dir" "$staging_dir"
```

The staging helpers consume the single validated `CFG_*` contract, leaving the
transaction's path-only signatures stable.

Critical filesystem helpers must also propagate every failure explicitly.
Bash suppresses `errexit` inside a function invoked by `if`, `!`, `&&`, or
`||`; a failed copy followed by a successful manifest write must never appear
successful.

#### Wrong

```bash
snapshot_installation() {
  cp -a "${ENV_FILE}" "${backup_dir}/env"
  printf 'env\t1\t%s\n' "${ENV_FILE}" >> "${backup_dir}/manifest"
}
snapshot_installation "${backup_dir}" || return
```

#### Correct

```bash
snapshot_installation() {
  cp -a "${ENV_FILE}" "${backup_dir}/env" || return 1
  printf 'env\t1\t%s\n' "${ENV_FILE}" >> "${backup_dir}/manifest" || return 1
}
snapshot_installation "${backup_dir}" || return
```
