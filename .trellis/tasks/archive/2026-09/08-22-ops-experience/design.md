# 设计：v0.3.0 生产金丝雀发布与 nightly 下线

任务：`08-22-ops-experience`
子任务状态：4/4 已归档
当前状态：本地集成审计与 `main` release dry-run 已完成

## Architecture Overview

父任务不再新增产品功能。用户明确接受无独立测试环境的剩余风险，并选择云主机/VM
快照作为生产升级的外部恢复兜底。剩余发布状态机为：

```text
clean main == origin/main
  │
  ├─ release-critical preflight
  │    ├─ current HEAD dry-run already green
  │    ├─ generated Installer / CLI / layout checks
  │    ├─ release notes review
  │    └─ v0.3.0 absence + nightly presence
  │
  ├─ production recovery preflight
  │    ├─ VM snapshot reaches recoverable/available state
  │    ├─ provider console + SSH verified
  │    └─ secret-free service/Nginx/version baseline captured
  │
  ├─ scripts/release.sh v0.3.0
  │    ├─ release commit + immutable tag
  │    └─ tag workflow publishes exact four assets
  │
  ├─ verify v0.3.0 release/latest/assets
  │
  ├─ production canary upgrade in maintenance window
  │    ├─ direct current/latest Installer → explicit v0.3.0
  │    ├─ transaction result + filesystem/service checks
  │    ├─ CLI/readyz/API/UI version consistency
  │    └─ two observation checkpoints
  │
  └─ production healthy ── delete nightly release/tag ── archive parent
```

The ordering is strict. `nightly` remains available until both the public
v0.3.0 release and production canary are verified. Production is not used for
fault injection, clean-install rehearsal, or deliberate legacy rollback.

## Responsibility Boundary

| Concern | Owner |
| --- | --- |
| source/tests/docs/spec integration | completed parent-task audit |
| release composition and checksums | `.github/workflows/release.yml` + `scripts/compose-release-assets.sh` |
| release commit/tag creation | `scripts/release.sh v0.3.0` |
| stable asset publication | tag-triggered GitHub Release workflow |
| infrastructure snapshot and restore | production cloud/VM provider |
| normal v0.3.0 production transaction | generated current/latest Installer |
| production smoke/observation | operator + read-only CLI/HTTP/systemd/Nginx checks |
| nightly release/tag deletion | explicit post-production-verification cleanup |

No product code changes are planned. A release/preflight finding returns to the
normal fix-forward quality loop and requires a new green `main` dry-run.

## Evidence Model and Accepted Gap

The parent acceptance criteria use three evidence layers:

1. **Repository evidence:** Installer/CLI/release-layout suites prove clean
   first-install, prompt-free update, first-install cleanup, fault rollback,
   legacy CLI/metadata removal/restore, secrecy, and exact assets.
2. **GitHub dry-run evidence:** run `33544029600` proves the pushed HEAD passes
   quality, both architecture builds, composition, checksums, attestation, and
   `main-37926e6` version injection without publication.
3. **Production canary evidence:** the existing production deployment proves the
   real normal update path, host filesystem/systemd/Nginx integration, release
   version consistency, readiness, UI behavior, and sustained operation.

There is deliberately no independent clean-host or real fault-injection E2E.
The user accepts that gap. Reports must not claim otherwise. In particular, the
production host must not be intentionally broken to exercise rollback, and a
pre-v0.3 direct Installer transition is reserved for actual recovery only.

## Completed Integration Contract

The existing audit already established:

- default no-mode Installer still selects interactive `install`;
- update mode is non-TTY, skips apt, and adopts the protected saved config;
- no current workflow, release script, README, config, or promoted docs path
  publishes/recommends nightly;
- tag/main version derivation is shared by Go and packaged CLI, and API/UI read
  the running Go version;
- transaction registry covers binary, env, CLI, metadata, systemd unit/link,
  and Nginx site/link;
- docs and `[Unreleased]` cover all four child deliverables and migration;
- local full gates and final main dry-run pass.

Historical CHANGELOG/Trellis references and active negative `nightly` rejection
contracts remain valid.

## Release-Critical Preflight

Immediately before release:

```bash
git fetch --force origin main --tags
git status --short --branch
bash scripts/generate-install.sh --check
bash scripts/install_test.sh
bash scripts/cli_test.sh
bash scripts/release_layout_test.sh
scripts/extract-changelog.sh Unreleased
go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.10 \
  .github/workflows/ci.yml .github/workflows/quality.yml .github/workflows/release.yml
git diff --check
```

Required state:

- clean `main == origin/main`;
- latest green `main` dry-run belongs to current HEAD;
- no local/remote/GitHub `v0.3.0` exists;
- `v0.2.0` is Latest and nightly release/tag still exist;
- release notes contain the complete intended public changes;
- generated Installer and release composition do not drift.

Any failure stops before tag creation.

## Production Recovery Preflight

The VM snapshot gate occurs **before any production mutation** and must be
operator/provider-confirmed:

1. create a named snapshot for the exact production VM/system disk;
2. wait until its state is `available`, `completed`, or the provider-equivalent
   recoverable state;
3. record only snapshot identifier/time/state, never credentials;
4. verify out-of-band provider console and SSH access;
5. record a secret-free baseline:
   - current Git commit/release if available;
   - `systemctl is-active/is-enabled bupt-ec`;
   - `nginx -t`;
   - `/healthz` and `/readyz` status/version;
   - owner/mode/hash of installed targets without printing private env values;
   - current public page/API behavior.

The snapshot is retained until production observation and nightly cleanup are
complete. “Snapshot requested” or a failed/in-progress snapshot is not enough.

## Stable Release State Machine

### Tag creation and push

Run `scripts/release.sh v0.3.0`. It creates:

- `chore: release v0.3.0`;
- `CHANGELOG.md` section/date/compare links;
- `frontend/package.json` version `0.3.0`;
- local immutable `v0.3.0` tag;
- after explicit push confirmation, remote `main` and `v0.3.0`.

If commit/tag creation succeeds but push does not occur, confirm no remote ref
before using the documented local rollback. Once pushed, the tag is never moved
or reused.

### Publication verification

Before touching production, verify:

- tag workflow succeeds for the release commit;
- GitHub release `v0.3.0` is neither draft nor prerelease;
- `latest` resolves to `v0.3.0`;
- notes equal the `CHANGELOG.md` `0.3.0` section;
- exact assets are two architecture tarballs, `checksums.txt`, and `install.sh`;
- checksums pass; tar members and modes match the documented layout;
- packaged/top-level/generated Installers match;
- both CLI packages and Go binaries carry `v0.3.0`.

A publication mismatch blocks production. Nightly remains untouched.

## Production Upgrade Contract

### Existing pre-v0.3 host

Because `/usr/local/bin/bupt-ec` does not yet exist, use a downloaded current
Installer rather than a pipe so update stdin can be closed explicitly:

```bash
session="$(mktemp -d /tmp/bupt-ec-release.XXXXXXXX)"
chmod 0700 "${session}"
curl --fail --show-error --silent --location \
  --proto '=https' --proto-redir '=https' \
  --connect-timeout 10 --max-time 60 \
  -o "${session}/install.sh" \
  https://github.com/ming-kang/BUPT_EC/releases/latest/download/install.sh
sudo VERSION=v0.3.0 bash "${session}/install.sh" --mode=update < /dev/null
rm -rf "${session}"
```

The live command must be reviewed against the production host first, including
whether its saved configuration is official GitHub or a custom mirror. If it
uses a custom mirror or saved `nightly`, use the documented matching recovery
path rather than silently changing trust source. Never print the private env.

The Installer's nonzero result is authoritative. On failure:

- stop and preserve complete terminal output without secrets;
- confirm whether `Rollback completed.` was reported;
- check old service/Nginx/health state against the baseline;
- preserve any root-only recovery directory;
- do not retry blindly and do not delete nightly.

### Post-commit smoke checks

On success verify, without changing configuration:

```bash
bupt-ec version
bupt-ec status
bupt-ec health
bupt-ec logs -n 50
sudo systemctl is-active bupt-ec
sudo systemctl is-enabled bupt-ec
sudo nginx -t
curl --fail --show-error --silent http://127.0.0.1:8080/healthz
curl --show-error --silent http://127.0.0.1:8080/readyz
```

Also verify:

- `/usr/local/bin/bupt-ec` is root:root `0755`;
- `/etc/bupt-ec/deployment.meta` is root:root `0644` and has only the two public
  fields;
- private env remains root:root `0600` without displaying its values;
- configured selector, running `/readyz` version, CLI version, API envelope,
  and UI settings all identify `v0.3.0` as appropriate;
- public page loads current data and browser HTML is not stuck on old assets;
- logs contain no new persistent error loop or secret disclosure.

`/readyz` may transiently return 503 during warmup. Treat it as failure only if
it does not become ready within the agreed observation or logs show a persistent
configuration/upstream error.

### Observation and success threshold

Use two successful checkpoints at least five minutes apart (one frontend cache
TTL), with the second no earlier than ten minutes after upgrade. Each checkpoint
requires active/enabled service, valid Nginx config, 2xx health/readiness,
`v0.3.0` version consistency, successful API/UI behavior, and no repeating new
error pattern. Until then, retain the VM snapshot and nightly release/tag.

## Production Rollback Strategy

1. **Installer exits nonzero:** rely first on its automatic transaction rollback
   and compare the restored host with the baseline. Do not restore the VM while
   rollback is still running.
2. **Rollback incomplete:** preserve the root-only recovery directory and use
   provider console/SSH plus the VM snapshot recovery plan.
3. **Installer succeeds but post-check/observation fails:** stop further changes,
   capture safe evidence, and restore the verified VM snapshot. This service has
   no durable classroom/user database, so snapshot rollback primarily loses
   transient cache/log interval, but provider behavior must still be reviewed.
4. **Published v0.3.0 defect:** do not move/delete/reuse the tag. Keep nightly,
   restore production, then fix forward and publish a new immutable version.
5. **Direct v0.2.x Installer rollback:** reserved as a secondary recovery option
   only; do not run it merely to test legacy removal, and require fresh explicit
   authorization before use.

## Nightly Cleanup

Only after release verification and both production checkpoints:

1. delete GitHub `nightly` prerelease;
2. delete remote `refs/tags/nightly`;
3. delete local `nightly` tag;
4. verify no nightly release/tag remains;
5. verify v0.3.0 remains Latest and latest/stable assets resolve;
6. retain the production VM snapshot until the cleanup verification is recorded,
   then hand snapshot retention/deletion back to the operator/provider policy.

Partial cleanup retries only the missing action and never touches v0.3.0.

Cleanup completed after the operator confirmed production success: GitHub nightly prerelease, remote tag, and local tag were deleted; v0.3.0 remained Latest.

## v0.3.1 Inline Health-Retry Output Fix

The production transaction showed the intended startup race behavior: the first loopback `/healthz` request reached the host before the restarted service had bound port 8080, then a later retry succeeded. `wait_for_health` currently redirects curl stdout only, so `curl -fsS` exposes each intermediate failure on stderr even though the retry state machine is still healthy.

The smallest correction is owned by `scripts/installer/40-transaction.sh::wait_for_health`: redirect both stdout and stderr for each bounded retry. Do not change the ten-attempt loop, two-second curl bounds, one-second delay, success predicate, or the final `Service health check failed: <url>` diagnostic. Therefore a transient failure becomes silent, while exhaustion still returns nonzero through the existing high-level message and transaction rollback path.

The generated `scripts/install.sh` must be regenerated rather than edited. The mock curl emits a recognizable stderr line on health failures; transaction tests must prove both a fail-then-success path suppresses it and an exhausted path suppresses raw curl output while retaining the Installer-owned final diagnostic. `CHANGELOG.md` carries the user-visible fix. Release `v0.3.1` uses the normal immutable release script/tag workflow and does not imply a production update.

## Security and Operational Notes

- No task artifact or chat output records GitHub tokens, SSH keys, JW secrets,
  private env content, raw credentials, or full sensitive logs.
- Production commands are shown for operator review before execution. Access
  method, hostname, maintenance timing, and snapshot identifier are runtime
  inputs, not committed repository configuration.
- Remote mutations are ordered: release tag/publication → production normal
  update → observation → nightly cleanup. No force push is permitted.
- This plan records explicit risk acceptance; it does not redefine mocks as a
  real-host fault-injection test.
