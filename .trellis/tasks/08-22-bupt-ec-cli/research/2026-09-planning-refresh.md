# bupt-ec CLI planning refresh — 2026-09-01

## Why this refresh is required

The original CLI plan predates the completed `08-22-installer-modes` work. That
dependency is now archived (`5eb45d6`, implementation `1c461aa`), but several
old assumptions no longer match the generated installer, its security model, or
the actual v0.2.0 release boundary.

## Confirmed current repository facts

### Installer and transaction

- The maintained installer is split under `scripts/installer/` and deterministically
  generates tracked `scripts/install.sh`; production changes must edit fragments
  and regenerate the artifact.
- `--mode=update` is noninteractive, skips apt, safely loads the installed twelve-field
  configuration, and accepts only `VERSION` as an update override.
- `--mode=reconfigure` is interactive and keeps the saved release selector.
- `transaction_targets` currently snapshots six roles: service binary, private env,
  systemd unit/link, and Nginx site/link. Snapshot and rollback iterate this registry;
  commit still installs regular-file candidates explicitly.
- The private env is `/etc/bupt-ec/bupt-ec.env`, root-owned mode `0600`, inside a
  root-controlled non-writable-by-others directory. Non-root read-only CLI commands
  therefore cannot obtain `RELEASE_VERSION` or a custom `APP_ADDR` from that file.
- The installer safe loader evaluates the trusted env only in an isolated child and
  accepts strict NUL-framed registered fields. A CLI root path must not source the env
  into its own long-lived process or allow the env to activate one-shot switches.

### Release layout

- Current architecture tarballs contain exactly `bupt-ec`, `.env.example`, `README.md`,
  and generated `install.sh`; `.github/workflows/release.yml` asserts this exact list
  and packaged/generated installer byte parity.
- `scripts/bupt-ec-cli.sh` can be copied as tarball member `bupt-ec-cli`; it must not be
  named `bupt-ec`, because `stage_release` currently locates the service executable by
  `find ... -name bupt-ec -print -quit`.
- `checksums.txt` covers each whole tarball, so a CLI inside the tarball is covered by
  the existing archive checksum without a new top-level checksum mechanism.
- The authoritative self-contained-asset and transaction contracts now live in
  `.trellis/spec/backend/installer-guidelines.md`, not the old quality-guidelines
  section named by the initial CLI PRD.

### The v0.3 compatibility boundary

- Existing stable tags include `v0.2.0` and earlier.
- `v0.2.0`'s published-source installer has no `--mode=update` or
  `--mode=reconfigure` parser.
- `v0.2.0` tarballs predate this task and contain no `bupt-ec-cli` member.
- Therefore the original flow "download the target version's install.sh, then invoke
  `--mode=update`" cannot implement the explicit acceptance example
  `bupt-ec update v0.2.0`.
- Current operator documentation already defines rollback as "run the current/latest
  installer and pass an older target `VERSION`". The CLI should preserve that control
  plane/data plane split: fetch the newest compatible installer from the official
  release `latest` URL (or the saved fixed mirror base), then pass the requested target
  version to it.

## Required planning changes

1. Replace the target-version-installer bootstrap design with current/latest-installer
   bootstrap. The target `VERSION` selects the archive, not the installer implementation.
2. Define an explicit migration rule for a pre-v0.3 target archive that has no CLI.
   The three viable product choices are:
   - atomically remove the installed CLI because that target release did not ship one;
   - retain the newer CLI beside the older service (convenient but version-skewed);
   - reject rollback below the first CLI-bearing release.
3. Preserve rootless `status` / `version` / `health` only by staging a root-owned,
   world-readable, non-secret metadata file containing at least `RELEASE_VERSION` and
   `APP_ADDR`; otherwise those commands must become root-only or explicitly degrade.
   If added, this metadata file must be rendered from validated `CFG_*`, included in
   `transaction_targets`, and removed together with CLI when selecting a legacy release.
4. Add a focused CLI behavior suite rather than testing only transaction integration.
   It must cover parsing, permissions, output/exit status, probe degradation, log flags,
   safe config display, installer bootstrap URLs, and secret non-disclosure. The existing
   installer suite remains responsible for archive extraction and transaction rollback.
5. Update the exact release-layout assertions, release docs, recursive shell gate, and
   installer spec together. Generated `scripts/install.sh` remains self-contained and
   must never source or call the installed CLI.

## Technical recommendation pending product confirmation

For rollback across the introduction boundary, use the latest compatible installer,
allow a pre-v0.3 archive to omit `bupt-ec-cli`, and atomically remove both the installed
CLI and its public metadata in the same transaction. A failed transaction restores both.
This keeps each installed release internally consistent and makes
`bupt-ec update v0.2.0` work, with the documented consequence that subsequent upgrades
from v0.2.x use the retained `curl | bash` fallback because the CLI no longer exists.

## User decisions after evidence review

- The CLI itself **rejects pre-v0.3 targets** before network access. It supports
  `latest` and stable tags at or above the first CLI-bearing release (`v0.3.0`).
  Direct current/latest Installer invocation remains the fallback for v0.2.x and
  transactionally removes CLI/metadata to preserve target-release consistency.
- Non-root `status` / `version` / `health` remain complete through a root-owned
  `0644` metadata file containing only `RELEASE_VERSION` and `APP_ADDR`; they do
  not degrade to guessed defaults or read the private env.
- `status` and `health` use strict readiness exit status: readiness 503 is shown
  as degraded/not-ready and returns nonzero even when liveness still passes.
