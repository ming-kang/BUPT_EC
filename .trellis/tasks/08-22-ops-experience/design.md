# 设计：v0.3.0 最终集成、发布与 nightly 下线

任务：`08-22-ops-experience`
子任务状态：4/4 已归档

## Architecture Overview

父任务不再新增产品功能。它把四个已归档交付整合为一个受控发布状态机：

```text
local main (18 commits ahead)
  │
  ├─ local full-scope integration audit
  │    ├─ Go/frontend/shell/security gates
  │    ├─ exact release-layout simulation
  │    ├─ changelog/nightly production-path audit
  │    └─ cross-child contract review
  │
  ├─ push main ── GitHub release.yml dry-run
  │                    ├─ reusable quality gate
  │                    ├─ amd64/arm64 embedded builds
  │                    └─ pack/checksum/attest artifacts, no publication
  │
  ├─ real Linux E2E gate [currently deferred / blocking]
  │    ├─ clean install
  │    ├─ bupt-ec update
  │    ├─ version/status/UI consistency
  │    └─ pre-v0.3 no-curl rejection + current Installer fallback
  │
  ├─ scripts/release.sh v0.3.0
  │    ├─ release commit
  │    ├─ immutable v0.3.0 tag
  │    └─ tag workflow publishes four assets
  │
  └─ verify v0.3.0 latest ── delete nightly release/tag
```

The safe ordering is strict. `nightly` remains available until v0.3.0 is
published and verified. The stable tag is not pushed while the real-host E2E
gate is deferred unless the user later makes an explicit risk-acceptance
decision.

## Responsibility Boundary

| Concern | Owner |
| --- | --- |
| local source/tests/docs/spec integration | parent task execution |
| release asset composition | `.github/workflows/release.yml` + `scripts/compose-release-assets.sh` |
| release commit/tag creation | `scripts/release.sh v0.3.0` |
| stable asset publication | GitHub tag-triggered Release workflow |
| real host systemd/Nginx/filesystem validation | disposable or designated Linux environment |
| nightly release/tag deletion | explicit post-release GitHub cleanup |

No product code should change during the parent integration pass unless a check
finds a genuine defect. Any defect returns the task to planning/execution,
updates the owning child contract or shared spec, and reruns the complete gate.

## Integration Contract

The eight parent acceptance criteria are reviewed as one release contract:

1. historical default `curl | sudo VERSION=latest bash` remains compatible;
2. installed updates require no configuration prompts;
3. nightly has no production code, workflow, installer, or promoted-doc path;
4. release tag, `/readyz` version, CLI build marker, and UI version agree;
5. service binary, CLI, metadata, env, systemd, and Nginx remain one rollback unit;
6. operator/deployment/release docs match final behavior;
7. all local and GitHub quality gates pass;
8. `[Unreleased]` is complete release-note material including nightly migration.

Historical CHANGELOG entries may mention nightly. Searches must classify each
match rather than mechanically require zero repository matches.

## Local Validation Layer

Before any remote mutation:

```bash
task check
task test
task installer:check
pnpm -C frontend build
pnpm -C frontend size
go build ./...
rm -rf web/dist && cp -r frontend/dist web/dist
go build -tags embed_assets ./...
GOTOOLCHAIN=go1.25.13 go run golang.org/x/vuln/cmd/govulncheck@v1.5.0 ./...
actionlint .github/workflows/ci.yml .github/workflows/quality.yml .github/workflows/release.yml
git diff --check
```

The release-layout suite remains the local packaging simulation for stable and
`main-<sha>` versions. Additional read-only audits verify exact release assets,
Installer/CLI version injection, changelog extraction, archived child state,
remote tags/releases, and the absence of production nightly paths.

## Main Push and Dry-Run Gate

The current local branch is ahead of `origin/main`, while `scripts/release.sh`
requires equality with `origin/main`. The first remote mutation is therefore a
normal `main` push after local checks and a clean working tree.

That push triggers `release.yml` in dry-run mode. Completion requires:

- the workflow run is associated with the pushed HEAD;
- all jobs succeed;
- the workflow uploads composed artifacts but creates no GitHub release;
- no `v0.3.0` tag/release exists afterward.

A failed dry-run blocks release. Fixes are committed normally to `main`, all
local gates rerun, and a new dry-run must pass.

## Real Linux E2E Gate

The release-grade scenario needs a disposable or explicitly designated Linux
host with root, systemd, Nginx, curl, tar, and network access. It verifies real
paths and ownership in addition to command behavior.

Required scenario shape:

1. install from current/latest self-contained Installer into a clean host;
2. verify `/opt/bupt-ec/bupt-ec`, `/usr/local/bin/bupt-ec`, private env, public
   metadata, systemd unit/link, and Nginx site/link ownership/modes;
3. verify `bupt-ec version/status/health/config show` secrecy and exit behavior;
4. perform a zero-prompt CLI-bearing update and confirm configuration retention;
5. verify CLI rejects a pre-v0.3 target before curl;
6. use the current/latest Installer directly for the legacy target and confirm
   transactional CLI/metadata removal or rollback behavior as selected by the
   test fixture/release availability.

The user has deferred this environment. The gate remains blocking; no host is
provisioned and no real-host commands run during the first execution tranche.

## Stable Release State Machine

### Pre-tag

- working tree clean;
- local `main == origin/main`;
- latest main dry-run green;
- real Linux E2E green, unless the user explicitly waives it later;
- `v0.3.0` absent locally/remotely;
- `[Unreleased]` extraction reviewed.

### Tag creation and push

Run `scripts/release.sh v0.3.0`. It creates only:

- `chore: release v0.3.0`;
- annotated/lightweight repository tag according to the existing script;
- `CHANGELOG.md` release section and compare links;
- `frontend/package.json` version `0.3.0`.

The script's push prompt is a point-of-no-return gate. If the local commit/tag
exists but was not pushed, follow the documented local rollback command or push
only after revalidation.

### Publication verification

The tag workflow must succeed before cleanup. Verify:

- GitHub release `v0.3.0` exists and is not a prerelease/draft;
- `latest` resolves to `v0.3.0`;
- release notes equal the `CHANGELOG.md` `0.3.0` section;
- exact top-level assets are two architecture tarballs, `checksums.txt`, and
  `install.sh`;
- checksums verify and each tarball has the exact documented member list;
- both CLI copies and Go binaries report/inject `v0.3.0` consistently.

### Nightly cleanup

Only after publication verification:

1. delete the GitHub `nightly` prerelease;
2. delete remote `refs/tags/nightly`;
3. delete the local `nightly` tag;
4. verify `gh release list` and `git ls-remote` show no nightly release/tag;
5. verify `v0.3.0` remains Latest and its assets still resolve.

Cleanup never rewrites history or moves `v0.3.0`.

## Failure and Rollback Strategy

| Failure point | Required response |
| --- | --- |
| local integration failure | no remote mutation; fix and rerun full gate |
| main push dry-run failure | release blocked; fix forward on main and rerun |
| E2E unavailable/deferred | stop before stable tag push |
| release script fails before commit/tag | restore working tree if needed; inspect partial changelog/package edits |
| local release commit/tag created but not pushed | use documented local tag delete + reset only after confirming no remote ref |
| tag workflow fails after push | do not delete nightly; prefer workflow rerun for transient failures or a documented fix-forward release decision |
| v0.3.0 verification mismatch | do not delete nightly; preserve evidence and investigate |
| partial nightly cleanup | retry only the missing deletion; never touch v0.3.0 |

## Security and Operational Notes

- Planning and verification commands may query GitHub read-only. Remote pushes,
  release publication, and deletion are explicit mutation stages.
- No credentials, GitHub tokens, JW secrets, deployment env contents, or host
  logs are recorded in task artifacts.
- The pinned Go 1.25.13 `govulncheck` result is authoritative for the repository
  toolchain; host Go 1.26 findings must be interpreted against the documented
  patched floor.
- Release artifacts are downloaded into protected temporary directories and
  removed after verification.

## Deferred Item

A real Linux E2E environment is intentionally deferred by user decision. The
parent task may advance through planning, local integration, and main dry-run,
but it cannot satisfy the final acceptance criteria or archive until that gate
is completed or explicitly waived in a future planning update.
