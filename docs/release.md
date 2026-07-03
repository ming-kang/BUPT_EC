# Releases

How versioning, the changelog, and the release pipeline work, and how to cut a release.

## Release flavors

| Flavor | Trigger | Audience |
|---|---|---|
| `nightly` prerelease | every push to `main` (automatic) | freshest build; the installer default |
| `vX.Y.Z` stable release | pushing a `v*` tag via `scripts/release.sh` | immutable, reproducible deployments |

Both flavors publish the same four assets, which the installer depends on by exact name:

- `bupt-ec-linux-amd64.tar.gz`
- `bupt-ec-linux-arm64.tar.gz`
- `checksums.txt`
- `install.sh`

Versioning follows [Semantic Versioning](https://semver.org/). While the project is pre-1.0, minor bumps may contain breaking changes.

## Changelog conventions

[CHANGELOG.md](../CHANGELOG.md) follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/):

- **Update it as you go.** A commit with a user-visible change adds a bullet to the `[Unreleased]` section in the same commit. Internal-only changes (CI tweaks, refactors with no visible effect) may skip it.
- Use the standard categories: `Added`, `Changed`, `Fixed`, `Removed`, `Deprecated`, `Security` — plus `Dependencies` for notable upgrades.
- Write bullets for operators and users, not for reviewers: what changed and why it matters, not how it was implemented.
- Don't edit released sections; corrections go in a new release.

The `[Unreleased]` section becomes the release notes verbatim, so keeping it clean is what makes releases cheap.

## Cutting a stable release

One command from a clean, up-to-date `main`:

```bash
scripts/release.sh v0.1.4
```

The script:

1. Validates the version format, that you are on `main`, the working tree is clean, local `main` matches `origin/main`, and the tag doesn't exist.
2. Shows the `[Unreleased]` changelog content that will become the notes.
3. Rolls `CHANGELOG.md`: renames `[Unreleased]` to `[0.1.4] - <today>`, starts a fresh empty `[Unreleased]`, and updates the compare links.
4. Bumps `version` in `frontend/package.json`.
5. Commits `chore: release v0.1.4`, tags `v0.1.4`, and (after confirmation) pushes `main` and the tag.

The tag push triggers the Release workflow, which publishes the GitHub release with the changelog section as its body (plus an auto-generated compare link).

If something fails after the commit/tag but before the push, undo locally with `git tag -d v0.1.4 && git reset --hard HEAD~1`.

## CI/CD pipeline

Two workflows, no overlap:

### `ci.yml` — pull requests

Runs the full quality gate on every PR to `main`: frontend lint + build, `gofmt` check, `go vet`, `go test -race`, `go build`, `govulncheck`, and `shellcheck` on all scripts.

### `release.yml` — pushes to `main` and `v*` tags

Four jobs in sequence:

1. **quality-gate** — same checks as CI (this is what validates direct pushes to `main`).
2. **build-frontend** — builds the React app, uploads `frontend/dist` as an artifact.
3. **build-go** — matrix over `amd64`/`arm64`; embeds the frontend artifact and compiles static Linux binaries (`CGO_ENABLED=0`).
4. **release** — packs each binary with `.env.example`, `README.md`, and `install.sh` into a tarball, generates `checksums.txt`, attests build provenance, then publishes:
   - **tag push**: a stable release whose body is extracted from `CHANGELOG.md` by `scripts/extract-changelog.sh`.
   - **main push**: deletes and re-creates the rolling `nightly` prerelease.
   - **manual dispatch**: a dry-run — assets are uploaded as workflow artifacts, nothing is published.

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

- Go 1.25 (`go.mod`, `actions/setup-go`)
- Node 22 LTS (`actions/setup-node`)
- pnpm 9.15.x (`corepack prepare`, lockfile v9)

All third-party actions are pinned to 40-character commit SHAs for supply-chain safety. To bump a pin, resolve the new SHA and update both the `uses:` ref and the version comment:

```bash
git ls-remote https://github.com/actions/checkout.git refs/tags/v4
```
