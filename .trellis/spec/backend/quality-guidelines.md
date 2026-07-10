# Quality Guidelines

## Overview

Backend changes should preserve the current small-service architecture: thin Gin
handlers, injectable service dependencies, no local timetable database, safe JW
error handling, and source-backed API contracts. Keep code readable and verify
with the same commands CI uses.

Primary references:

- `.github/workflows/ci.yml` and `.github/workflows/release.yml` for quality
  gates.
- `service/realtime_data_test.go` for service unit test style.
- `handler_test.go` for HTTP/router test style.
- `docs/development.md`, `docs/release.md`, and `CHANGELOG.md` for workflow and
  release conventions.

## Required Go Patterns

- Format all Go code with `gofmt`. CI expects `gofmt -l .` to print nothing.
- Keep service dependencies injectable. `ClassroomService` accepts a `CacheStore`
  and internally uses a `JWClient`; tests create isolated services with
  `newTestService(t, client)` and a fresh `gocache` instance.
- Use contexts for external work. JW login, API URL fetch, classroom refreshes,
  and HTTP requests all use bounded contexts.
- Keep handlers thin and service logic testable without Gin.
- Keep public JSON structs in `service/model/` with explicit `json` tags.
- Use `errors.As`, `errors.Is`, or `errors.Join` instead of string-matching
  error text.
- Prefer clear exported APIs only at package boundaries; keep helper functions
  unexported unless another package really needs them.

## Testing Requirements

Add or update focused tests when changing behavior, especially for:

- JW response parsing and error classification;
- cache freshness, stale data, and cross-day rejection;
- refresh coordination and concurrency sharing;
- room parsing/building normalization;
- HTTP envelopes, health/readiness behavior, gzip, and SPA fallback.

Local test patterns:

- `service/realtime_data_test.go` defines `mockJWClient` and `newTestService`.
  Follow this pattern for service tests so unit tests do not touch the network.
- Integration tests such as `TestLogin`, `TestQueryOne`, and `TestQueryAll`
  require `JW_TOKEN` or `JW_USERNAME`/`JW_PASSWORD` and must skip cleanly when
  credentials are missing.
- Handler tests should inject deterministic fakes through `NewHTTPServer` and
  use `httptest` plus `gin.New()` or `HTTPServer.RegisterRoutes` when route
  middleware such as `/api` `log_id` correlation matters.

Avoid tests that only restate the implementation. Prefer tests that protect
contract edges, race-prone behavior, security checks, or user-visible output.

## Verification Commands

Use the smallest reliable set while developing, then run broader checks before
finishing substantial backend changes:

```bash
gofmt -l .
go vet ./...
go test ./...
```

For frontend source, API-normalization, selection-state, or package changes,
also run:

```bash
pnpm --dir "frontend" lint
pnpm --dir "frontend" test
pnpm --dir "frontend" build
```

CI and release quality gates also run:

```bash
go test -race ./...
go build -o bupt-ec -v ./
govulncheck ./...
bash scripts/install_test.sh
shellcheck scripts/*.sh
cd frontend && pnpm lint
cd frontend && pnpm build
```

The Go build embeds `frontend/dist/` through `router.go`, so local full builds
need `cd frontend && pnpm build` first if `frontend/dist/` is missing.

## Frontend and Cross-Layer Quality

Backend API changes often require frontend changes because the React app reads
the backend payload directly. Before changing `service/model/realtime_data.go`,
`classroom_builder.go`, or `handler.go`, inspect these frontend files:

- `frontend/src/useTodayClassrooms.js` for fetch and normalization.
- `frontend/src/todayClassroomsResponse.js` for API envelope normalization.
- `frontend/src/components/BuildingPicker.jsx` for building assumptions.
- `frontend/src/components/ClassTimePicker.jsx` for campus node assumptions.
- `frontend/src/components/TodayClassroomTable.jsx` for `free_nodes` filtering.

Frontend code uses ES modules, React hooks, 2-space indentation, PascalCase
component filenames, matching component CSS files, and shared selection state
through `useSelection()` rather than prop drilling.

## Documentation and Release Hygiene

User-visible changes must update documentation and changelog in the same change:

- Update `README.md` or `docs/` when behavior, endpoints, config, deployment,
  operations, or release process changes.
- Add a bullet to `CHANGELOG.md` under `[Unreleased]` using Keep a Changelog
  categories (`Added`, `Changed`, `Fixed`, `Removed`, `Deprecated`, `Security`,
  or `Dependencies`).
- Commit messages in this repository use Conventional Commit prefixes such as
  `feat:`, `fix:`, `chore:`, `ci:`, `docs:`, and `refactor:`.
- Do not mix module renames, dependency updates, and behavior changes in one
  commit.

Release automation depends on exact asset names and layout. If changing release
assets or installer behavior, update `scripts/release.sh`, `scripts/install.sh`,
`.github/workflows/release.yml`, and `docs/release.md` together.

## Scenario: Installer Release Selection

### 1. Scope / Trigger

Apply this contract whenever installer version defaults, release URLs,
deployment commands, or persisted installer metadata change. It prevents a
stable installer URL from silently downloading the rolling nightly package.

### 2. Signatures

```bash
resolve_release_version <explicit-version> <saved-version>
validate_version <version>
resolve_download_base_url <repo> <version> <override-url>
```

### 3. Contracts

- `VERSION`: optional command environment value; highest precedence.
- `RELEASE_VERSION`: saved in `/etc/bupt-ec/bupt-ec.env`; reused when
  `VERSION` is absent.
- First install with neither value uses `nightly`.
- Valid values are `latest`, `nightly`, or `vMAJOR.MINOR.PATCH`.
- `latest` maps to `/releases/latest/download`; other valid values map to
  `/releases/download/<version>`.
- A validated `DOWNLOAD_BASE_URL` overrides the GitHub-derived base URL.

### 4. Validation & Error Matrix

| Input | Result |
| --- | --- |
| `latest` / `nightly` | accepted |
| `v0.1.4` | accepted |
| empty final value, path separators, whitespace, shell punctuation | non-zero validation failure |
| HTTP mirror without explicit insecure opt-in | non-zero validation failure |

### 5. Good/Base/Bad Cases

- Good: latest installer command passes `sudo VERSION=latest bash`.
- Base: no explicit or saved version resolves to `nightly`.
- Bad: download `releases/latest/download/install.sh` and invoke `sudo bash`;
  the script cannot infer which URL supplied its stdin.

### 6. Tests Required

- `bash scripts/install_test.sh` asserts precedence, accepted/rejected values,
  GitHub URL mapping, and custom mirror preservation.
- Both CI quality gates must execute the behavior test before shellcheck.
- Search README and `docs/` for stable/nightly installer commands without a
  matching explicit `VERSION`.

### 7. Wrong vs Correct

#### Wrong

```bash
curl -fsSL https://github.com/org/repo/releases/latest/download/install.sh | sudo bash
```

#### Correct

```bash
curl -fsSL https://github.com/org/repo/releases/latest/download/install.sh | sudo VERSION=latest bash
```

## Scenario: Transactional Installer Commit and Rollback

### 1. Scope / Trigger

Apply this contract whenever `scripts/install.sh` changes release staging,
installed paths, file ownership/modes, systemd or Nginx activation, health
validation, or rollback behavior. The installer runs as root and updates a live
service, so a partially applied change is a production correctness and secret
handling failure.

### 2. Signatures

```bash
configure_installer_test_root <absolute-root>   # sourced tests only
prepare_staging <archive> <work-dir> <staging-dir> <config...>
snapshot_installation <backup-dir>
atomic_install_file <source> <target> <mode> <owner>
atomic_install_symlink <link-target> <target>
commit_installation <staging-dir> <app-addr>
rollback_installation <backup-dir>
perform_install_transaction <staging-dir> <backup-dir> <app-addr>
```

### 3. Contracts

The seven stages are ordered and must not be interleaved:

1. Validate input, certificates, release selection, and platform prerequisites.
2. Download the architecture archive and verify its `checksums.txt` entry.
3. Extract the archive and render every candidate under a mode-`0700` staging
   directory; candidate env is root-owned mode `0600`.
4. Snapshot binary, env, systemd unit + enabled link, and Nginx site + enabled
   link. The manifest records both existing and absent targets under a
   mode-`0700` backup directory; env and manifest are mode `0600`.
5. Copy each candidate to `<target>.new.$$` in the target directory, set
   owner/mode, then use `mv -T` for same-filesystem atomic replacement.
6. Run daemon reload, unit enablement, `nginx -t`, service restart,
   `is-active`, Nginx reload, and loopback `/healthz` retry validation.
7. On success remove the backup and only then print success. On failure restore
   originally present targets, remove originally absent targets, reload the old
   configuration, and attempt to restart the old service. Preserve root-only
   recovery files when rollback itself is incomplete.

Production paths are fixed constants. Environment variables must not redirect
them. Tests opt into a temporary root only by sourcing the script and calling
`configure_installer_test_root` explicitly. Release archives and top-level
assets remain self-contained; no runtime helper beside `install.sh` is allowed.

### 4. Validation & Error Matrix

| Condition | Required result |
| --- | --- |
| checksum download/entry/hash failure | non-zero; installed targets byte-identical |
| archive missing `bupt-ec` or candidate render failure | non-zero; snapshot/commit not entered |
| snapshot copy/manifest failure | non-zero; transaction inactive; installed targets unchanged |
| atomic write, daemon reload, enable, or `nginx -t` failure | restore every recorded target/existence state |
| restart, `is-active`, reload, or loopback health failure | restore files and attempt old-service restart |
| first-install commit failure | remove newly created transaction targets |
| rollback command failure | non-zero; preserve and print root-only recovery directory |
| all validations pass | remove backup; clear transaction state; print success |
| non-loopback `APP_ADDR` | explicitly log that direct health probing is skipped |

### 5. Good/Base/Bad Cases

- Good: an upgrade stages and snapshots everything, atomically installs all
  candidates, passes validation, removes the backup, then reports success.
- Base: a first install records every target as absent; a failed validation
  removes all newly created transaction targets and does not restart a nonexistent
  old service.
- Bad: write `/etc/bupt-ec/bupt-ec.env` before checksum verification, or keep a
  new binary after `nginx -t`/health validation fails.

### 6. Tests Required

`bash scripts/install_test.sh` must use an explicit temporary root plus mocked
`curl`, `chown`, `systemctl`, and `nginx`, and assert:

- missing/invalid checksums, missing binary, render failure, and snapshot copy
  failure leave old targets unchanged;
- Nginx, restart, and health failures restore file content, modes, symlink
  targets, and attempt the old-service restart where one existed;
- first-install rollback removes all transaction targets;
- incomplete rollback preserves a mode-`0700` recovery directory and mode-`0600`
  env backup;
- successful upgrade replaces every target, enables both services/sites,
  installs env mode `0600`, clears transaction state, and removes the backup;
- `bash -n scripts/install.sh scripts/install_test.sh`, `shellcheck scripts/*.sh`,
  and release asset layout checks remain green.

### 7. Wrong vs Correct

#### Wrong

```bash
snapshot_installation() {
  cp -a "${ENV_FILE}" "${backup_dir}/env"
  printf 'env\t1\t%s\n' "${ENV_FILE}" >> "${backup_dir}/manifest"
}

# Bash suppresses errexit inside a function used in an OR-list. A failed cp can
# be masked by the later successful printf.
snapshot_installation "${backup_dir}" || return
```

#### Correct

```bash
snapshot_installation() {
  if ! cp -a "${ENV_FILE}" "${backup_dir}/env"; then
    echo "Failed to snapshot env." >&2
    return 1
  fi
  printf 'env\t1\t%s\n' "${ENV_FILE}" >> "${backup_dir}/manifest" || return
}

snapshot_installation "${backup_dir}" || return
```

Critical installer helpers must explicitly propagate each filesystem failure;
do not rely only on `set -e` when callers use `if`, `!`, `&&`, or `||`.

## Security Checklist

- Never commit real `.env` files, `JW_USERNAME`, `JW_PASSWORD`, `JW_TOKEN`, or
  generated `run_log/` files.
- Keep JW API URL validation restricted to HTTPS BUPT hosts.
- Keep the AES key in `service/crypto.go` aligned with the JW protocol; do not
  change it casually.
- Keep `/etc/bupt-ec/bupt-ec.env` documented as root-owned mode `0600` for
  deployed servers.
- Keep production `APP_ADDR` behind Nginx as `127.0.0.1:8080` unless the deploy
  design changes.

## Review Checklist

- Does the change preserve `ClassroomService` as the owner of mutable runtime
  state?
- Are errors classified internally and converted to safe client messages only at
  the HTTP boundary?
- Are logs structured and correlated with request context when applicable?
- Does any public JSON change update frontend consumers and tests?
- Do same-day cache and stale behavior still reject cross-day reuse?
- Are docs and changelog updated for user-visible behavior?
- Did the author run the relevant Go/frontend/script checks?

## Forbidden Patterns

- Network-dependent unit tests that do not skip without credentials.
- Global mutable state in `service/` that bypasses injected test instances.
- Raw `err.Error()` in API responses.
- Logging secrets or raw upstream payloads.
- Reintroducing local timetable persistence as an incidental implementation
  detail.
