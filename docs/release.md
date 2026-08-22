# Releases

How versioning, the changelog, and the release pipeline work, and how to cut a release.

## Release channel and assets

Stable `vX.Y.Z` tags are the only published release channel. A tag pushed by
`scripts/release.sh` produces an immutable, reproducible release; pushes to
`main` build and pack the same assets as a dry-run but publish nothing.

A release publishes four assets, which the installer depends on by exact name:

- `bupt-ec-linux-amd64.tar.gz`
- `bupt-ec-linux-arm64.tar.gz`
- `checksums.txt`
- `install.sh`

Installer commands select the release explicitly: `VERSION=latest` tracks the
newest stable release, and immutable deployments use a matching
`VERSION=vX.Y.Z`. The installer persists that choice as `RELEASE_VERSION`; a
first-time install without either value falls back to `latest`.

Versioning follows [Semantic Versioning](https://semver.org/). While the project is pre-1.0, minor bumps may contain breaking changes.

`install.sh` is intentionally a self-contained release asset. Its transaction flow is preflight → snapshot → atomic commit → validation/rollback; do not add runtime helper files to the published layout. `scripts/install_test.sh` uses a temporary root and mocked network/system commands to cover checksum and archive failures, upgrade rollback, first-install cleanup, permissions, and successful commit behavior.

## Changelog conventions

[CHANGELOG.md](../CHANGELOG.md) follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/):

- **Update it as you go.** A commit with a user-visible change adds a bullet to the `[Unreleased]` section in the same commit. Internal-only changes (CI tweaks, refactors with no visible effect) may skip it.
- Use the standard categories: `Added`, `Changed`, `Fixed`, `Removed`, `Deprecated`, `Security` — plus `Dependencies` for notable upgrades.
- Write bullets for operators and users, not for reviewers: what changed and why it matters, not how it was implemented.
- Don't edit released sections; corrections go in a new release.

The `[Unreleased]` section becomes the release notes verbatim, so keeping it clean is what makes releases cheap. Stable `v*` tag releases publish that extracted section only (`body_path`); they do not append GitHub auto-generated notes.

PR CI and the release workflow both call the reusable quality gate in
`.github/workflows/quality.yml` so the frontend, Go, audit, installer, and
ShellCheck checks stay identical.

## Cutting a stable release

One command from a clean, up-to-date `main`:

```bash
scripts/release.sh v0.1.6
```

The script:

1. Validates the version format, that you are on `main`, the working tree is clean, local `main` matches `origin/main`, and the tag doesn't exist.
2. Shows the `[Unreleased]` changelog content that will become the notes.
3. Rolls `CHANGELOG.md`: renames `[Unreleased]` to `[0.1.6] - <today>`, starts a fresh empty `[Unreleased]`, and updates the compare links.
4. Bumps `version` in `frontend/package.json`.
5. Commits `chore: release v0.1.6`, tags `v0.1.6`, and (after confirmation) pushes `main` and the tag.

The tag push triggers the Release workflow, which publishes the GitHub release with the changelog section as its body (plus an auto-generated compare link).

If something fails after the commit/tag but before the push, undo locally with `git tag -d v0.1.6 && git reset --hard HEAD~1`.

## CI/CD pipeline

Three workflows: `ci.yml` and `release.yml` both call `quality.yml` (reusable gate), so the checks stay identical with no overlap:

### `ci.yml` — pull requests

Runs the full quality gate on every PR to `main`: frontend production/toolchain audits + lint + test + build, the bundle size budget check (`node scripts/check-bundle-size.mjs` in `frontend/`, over the freshly built `dist/`), `go mod tidy -diff`, `go mod verify`, `gofmt` check, `go vet`, `go test -race`, `go build` (tag-less, so a clean checkout stays buildable), an embedded-assets build (`go build -tags embed_assets` with the freshly built frontend copied to `web/dist`), `govulncheck` (pinned version), transactional installer behavior tests, and `shellcheck` on all scripts.

### `release.yml` — pushes to `main` and `v*` tags

Three jobs in sequence:

1. **quality-gate** — same checks as CI (this is what validates direct pushes to `main`); the frontend it builds is uploaded as the `frontend-dist` artifact.
2. **build-go** — matrix over `amd64`/`arm64`; downloads the frontend artifact, embeds it, and compiles static Linux binaries (`CGO_ENABLED=0`, `-trimpath`, version injected via `-ldflags "-X main.version=..."` — the tag name, or `main-<commit>` on `main` pushes).
3. **release** — packs each binary with `.env.example`, `README.md`, and `install.sh` into a tarball, generates `checksums.txt`, attests build provenance, then publishes:
   - **tag push**: a stable release whose body is extracted from `CHANGELOG.md` by `scripts/extract-changelog.sh`.
   - **main push** and **manual dispatch**: a dry-run — the identical pack /
     checksum / attest path runs and the assets are uploaded as workflow
     artifacts, but nothing is published. This is deliberate: the packaging
     path is exercised on every merge, so a broken tarball or checksum step
     surfaces then instead of on release day.

Release assets keep this layout (the installer depends on it):

```text
bupt-ec-linux-${arch}/
  bupt-ec
  .env.example
  README.md
  install.sh
```

## Toolchain versions

Pinned via the workflows and `go.mod`; no Dependabot — bump by hand:

- Go 1.25.13 (`go.mod`, `actions/setup-go`; Go 1.26 users need a current patch release)
- Node 22 LTS (`actions/setup-node`)
- pnpm 9.15.x (`corepack prepare`, lockfile v9)

All third-party actions are pinned to 40-character commit SHAs for supply-chain safety. To bump a pin, resolve the new SHA and update both the `uses:` ref and the version comment:

```bash
git ls-remote https://github.com/actions/checkout.git refs/tags/v4
```
