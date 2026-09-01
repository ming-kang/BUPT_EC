# v0.3.0 final integration baseline — 2026-09-01

## Local repository state

- Branch: `main`.
- Working tree was clean at planning start.
- Local `main` is 18 commits ahead of `origin/main`; the v0.3.0 work is not yet remote.
- No local or remote `v0.3.0` tag exists.
- The four child tasks linked from `08-22-ops-experience` are archived; the parent remains `planning`.

## Remote release state

Read-only checks used `git ls-remote` and `gh release list`:

- `origin/main` is `8cd4594f08c3461b0320ecb7ca0ad5badda68728`.
- The remote `nightly` tag points to the same commit.
- GitHub currently marks `v0.2.0` as Latest.
- A `nightly` prerelease and remote `nightly` tag both still exist.
- GitHub CLI authentication is available with repository/workflow permissions; no remote mutation was performed during planning.

## Release control-plane constraints

`scripts/release.sh v0.3.0` requires:

1. branch `main`;
2. a clean working tree;
3. local `HEAD == origin/main` after fetch;
4. no existing `v0.3.0` tag.

It rolls `CHANGELOG.md`, bumps `frontend/package.json`, creates
`chore: release v0.3.0`, tags `v0.3.0`, and asks before pushing `main` and the
tag. Therefore the existing 18 local commits must first be pushed and the
resulting `main` release dry-run workflow must pass before the stable release
script can run safely.

The tag workflow publishes the four exact assets:

- `bupt-ec-linux-amd64.tar.gz`
- `bupt-ec-linux-arm64.tar.gz`
- `checksums.txt`
- `install.sh`

The `nightly` release/tag deletion must happen only after v0.3.0 is published,
its assets and release notes are verified, and GitHub `latest` resolves to
v0.3.0.

## End-to-end environment decision

The parent PRD requires a clean-environment rehearsal of initial installation,
`bupt-ec update`, CLI-bearing version selection, and the pre-v0.3 rejection plus
current/latest Installer fallback. Repository mocks and release-layout tests
cover the contracts but cannot prove real systemd/Nginx/root filesystem
integration. A disposable Linux VM is the recommended final gate; using only
local mocks must be an explicit reduction in release assurance.
