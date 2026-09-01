# v0.3.0 local integration audit — 2026-09-02

## Archived child evidence

All four children are archived with `status: completed`:

| Child | Work commit | Integration evidence |
| --- | --- | --- |
| `08-22-api-version-field` | `e536d0b feat(api): add version to the get_data envelope and show it in settings` | Typed `/api/get_data` success/error envelopes carry `main.version`; frontend normalization/error flow preserves the optional value; `CampusSettingsModal` renders it immediately above the repository link; Go and Vitest coverage includes success, 503, absent version, and hard-empty preservation. |
| `08-22-drop-nightly` | `37ad2e1 chore(release): drop the nightly channel and default installs to latest` | `main` is release dry-run only; tags publish. Installer resolves a first install to `latest` and rejects `nightly` with `VERSION=latest` recovery; current docs/spec/CHANGELOG were revised. |
| `08-22-installer-modes` | `1c461aa feat(installer): add generated deployment modes` | Generated self-contained Installer has default interactive install, non-TTY prompt-free update, interactive fixed-version reconfigure, the 12-field configuration registry, generator drift protection, and mode/transaction tests. |
| `08-22-bupt-ec-cli` | `d4e45b2 feat(cli): add transactional operations command` | CLI/metadata are separate archive members but transaction targets; CLI bootstraps the current Installer, rejects pre-v0.3 before curl, and release-layout tests cover stable and `main-<sha>` CLI injection. |

## Parent acceptance mapping

| Parent criterion | Local evidence | State after local audit |
| --- | --- | --- |
| Compatible default Installer | `parse_mode` defaults to `install`; `resolve_release_version` defaults to `latest`; installer modes/policy tests passed. | Source/mock proven; real clean-host install still required. |
| Update has no questions | Update mode bypasses TTY/prompts/apt and adopts saved configuration; Installer and CLI suites passed. | Source/mock proven; real host update still required. |
| One stable release channel | `release.yml` has no `nightly` reference and only tag pushes publish; current product paths reject rather than promote `nightly`. Remote nightly release/tag deliberately remain until post-v0.3 verification. | Local source proven; end-state blocked on stable release then authorized cleanup. |
| Release/readyz/CLI/UI versions agree | `release.yml` derives tag or `main-<short-sha>` for Go; composition substitutes that exact value in the one CLI marker; API/UI carry `main.version`; layout simulation passed. | Packaging/data-flow proven; deployed tag/runtime consistency needs E2E/release. |
| One rollback transaction | The eight-role registry includes binary, env, CLI, metadata, unit/link, and Nginx site/link; snapshot/rollback iterate it; CLI/legacy removal tests passed. | Mock transaction proven; real systemd/Nginx/filesystem validation deferred. |
| Documentation synchronization | `README.md`, `docs/deployment.md`, `docs/upgrading.md`, `docs/operations.md`, and `docs/release.md` were read against the final modes, CLI floor/fallback, metadata, release layout, and `main-<short-sha>` wording. No current guide promotes nightly. | Proven locally. |
| Quality gates | Full local preflight in `review/2026-09-02-release-preflight.md` passed after the audit remediation. | Local proven; pushed GitHub dry-run pending. |
| Complete Unreleased notes | `[Unreleased]` has Added entries for CLI, modes, and UI/API version; Removed includes nightly removal and saved-nightly `VERSION=latest` migration; Changed includes generated Installer; Fixed includes retained settings and this audit remediation. | Proven locally. |

## `nightly` reference classification

`git grep -il nightly` found 34 tracked paths (2026-09-02):

- **1 `CHANGELOG.md` path:** `[Unreleased]` contains the required removal/migration note; released historical sections retain historical release facts. Allowed release-note/history references, not a promotion path.
- **1 current migration guide:** `docs/upgrading.md` only states that a saved `RELEASE_VERSION=nightly` fails and must be changed with `VERSION=latest`. This is a required migration warning, not channel promotion.
- **4 active script/test paths:** `scripts/installer/10-config.sh` and generated `scripts/install.sh` reject it with recovery guidance; `scripts/installer_test/{policy,modes}.sh` assert that rejection. These are guard/test references, not a release path.
- **1 current spec path:** `.trellis/spec/backend/installer-guidelines.md` specifies rejection and recovery. This is a negative contract, not promotion.
- **3 active parent-task paths:** task title/PRD/context describe the removal and deferred post-release cleanup. Planning evidence only.
- **23 archived task paths and 1 workspace journal:** historical planning/research records. Non-production historical records.

There are **no** `nightly` matches in `.github/workflows/`, `README.md`, `docs/deployment.md`, `docs/operations.md`, `docs/release.md`, config examples, or release-composition scripts. No production workflow/script/current documentation/config path selects, builds, publishes, or recommends nightly.

## Release interface and version-flow audit

- `.github/workflows/release.yml` builds Linux binaries with `-X main.version=${version}`, where a tag uses its tag name and a branch uses `main-$(git rev-parse --short HEAD)`; tag pushes alone execute `Publish release`.
- `scripts/compose-release-assets.sh` permits only stable tags or `main-<sha>`, requires exactly one source CLI marker, injects it once per architecture package, enforces executable modes, verifies packaged Installer parity, and rejects unexpected top-level files.
- `scripts/release_layout_test.sh` exercised both `v0.3.0` and `main-deadbeef`, checks two architecture tar member lists, checksum entries, byte-identical CLI scripts, the top-level generated Installer, and no standalone CLI asset.
- The exact top-level interface is `bupt-ec-linux-amd64.tar.gz`, `bupt-ec-linux-arm64.tar.gz`, `checksums.txt`, and `install.sh`. Each tarball has only its root directory plus `bupt-ec`, `bupt-ec-cli`, `.env.example`, `README.md`, and generated `install.sh`.
- `main.go` owns the default `version = "dev"`; `handler.go` emits it from `/readyz` and `/api/get_data`; frontend normalization/error handling carries a safe optional version; `CampusSettingsModal` renders it when present. This gives the UI the running backend value rather than a frontend build-time value.

## Scope and remaining blocker

No product-contract defect was found in the child integration audit. A separate development-only dependency-security defect found during preflight is recorded in the release-preflight review and fixed in this parent pass. Real Linux E2E remains intentionally deferred and blocks the v0.3.0 tag/release and nightly cleanup.
