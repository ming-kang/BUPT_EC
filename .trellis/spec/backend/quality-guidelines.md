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
- Keep service dependencies injectable. `NewClassroomService` accepts explicit
  `ClassroomServiceOptions`, a `TodayClassroomCache`, and a `JWClient`; tests
  create isolated services with `newTestService` / `newTestServiceWithOptions`,
  injecting a thread-safe fake `Clock` and fixed `BackoffRandom` when asserting
  time or backoff deadlines, plus a fresh `cache.TodayClassroomsStore`.
- Keep runtime environment access in `config.Load` plus the `main.go`
  composition root. Tests pass map-backed lookups and constructor values rather
  than mutating config/cache globals.
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
- refresh coordination, adaptive total-failure backoff/jitter, and concurrency;
- room parsing/building normalization;
- HTTP envelopes, health/readiness behavior, gzip, and SPA fallback;
- Prometheus `/metrics` encoding and low-cardinality collector labels;
- TokenManager login observations (`ObserveLogin` source/outcome/singleflight).

Local test patterns:

- `service/realtime_data_test.go` defines `mockJWClient` and `newTestService`.
  Follow this pattern for service tests so unit tests do not touch the network.
- Backoff/jitter tests live in `service/refresh_backoff_test.go` and must use
  `options.Clock` + fixed unit samples (no `sleep` for core deadline state, no
  assigning a production `now` field).
- `service/jw_protocol_test.go` uses an injected `HTTPDoer` for offline JW
  Login/FetchAPIURL/QueryCampus protocol fixtures (method, path, token header,
  auth/parse classification). Never require network or credentials there.
- `service/crypto_test.go` pins AES known vectors with independently generated
  expected ciphertexts; do not derive expected values by calling
  `encryptJWPassword` in the test.
- `TestLogin` requires `JW_USERNAME`/`JW_PASSWORD`; query integration tests may
  use that pair or `JW_TOKEN`. All must skip cleanly when their required
  credentials are missing.
- Handler tests should inject deterministic fakes through `NewHTTPServer` and
  use `httptest` plus `gin.New()` or `HTTPServer.RegisterRoutes` when route
  middleware such as `/api` `log_id` correlation matters.
- Metrics endpoint tests must use a real `promhttp.HandlerFor` over
  `NewPrometheusMetrics()`'s isolated registry (not a fixed fake body), with
  `DisableCompression: true` matching production, and assert identity/gzip
  bodies parse as Prometheus text after at most one decompress.
- Login metric tests use a recording `RuntimeMetrics` (or Gather on an isolated
  registry) and assert one observation per shared network login, correct
  `source` provenance, non-negative duration, and no secret labels.
- Frontend `*.lifecycle.test.jsx` files mount real hooks under jsdom via
  `@testing-library/react`; pure helper tests remain on the default node env.

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
pnpm --dir "frontend" audit:prod
pnpm --dir "frontend" audit:dev
```

CI and release quality gates also run (via `.github/workflows/quality.yml`):

```bash
go mod tidy -diff
go mod verify
go test -race ./...
go build -o bupt-ec -v ./
govulncheck ./...
bash scripts/install_test.sh
shellcheck scripts/*.sh
cd frontend && pnpm lint
cd frontend && pnpm build
cd frontend && pnpm audit:prod
cd frontend && pnpm audit:dev
```

Frontend audit policy is executable through `frontend/package.json` so local,
PR, and release checks share the same thresholds: production dependencies fail
at moderate or above; the full development toolchain fails at high or above.
Generate and verify `frontend/pnpm-lock.yaml` with pnpm 9.15.x.

## Scenario: Dependency Security Baseline

### 1. Scope / Trigger

Apply this contract whenever Go or frontend dependencies, toolchain versions,
lockfiles, lint configuration, or CI/release quality gates change.

### 2. Signatures

```bash
GOTOOLCHAIN=go1.25.12 go run golang.org/x/vuln/cmd/govulncheck@v1.5.0 ./...
pnpm --dir frontend audit:prod
pnpm --dir frontend audit:dev
```

### 3. Contracts

- `go.mod` and every `actions/setup-go` step use Go `1.25.12`; Go 1.26 users
  need `1.26.5` or newer.
- `frontend/package.json` owns the audit thresholds: `audit:prod` checks
  production dependencies at `moderate`, while `audit:dev` checks the complete
  toolchain at `high`.
- PR and release workflows run both scripts after
  `pnpm install --frozen-lockfile`.
- Generate `frontend/pnpm-lock.yaml` with pnpm 9.15.x and keep the manifest's
  `packageManager` field aligned with that line.
- Upgrade the smallest compatible dependency set. Do not use an unrelated
  framework or application rewrite to clear a transitive advisory.

### 4. Validation & Error Matrix

| Condition | Required result |
| --- | --- |
| Reachable Go vulnerability | `govulncheck` fails the change |
| Production moderate/high/critical advisory | `audit:prod` fails |
| Full-toolchain high/critical advisory | `audit:dev` fails |
| Low/moderate development-only advisory | document the finding or patch it; `audit:dev` may pass |
| Lockfile differs under frozen install | CI/release fails before audit |
| Toolchain version is below the documented security floor | update is incomplete |

### 5. Good/Base/Bad Cases

- Good: patch the vulnerable dependency, regenerate with pnpm 9.15.x, then run
  frozen install, lint, tests, build, both audits, and `govulncheck`.
- Base: an unreachable module advisory remains visible in verbose
  `govulncheck`, but the symbol scan reports zero reachable vulnerabilities.
- Bad: widen `go-version` to `1.25`, suppress audit errors, or upgrade React/
  Ant Design solely to replace one compatible transitive dependency.

### 6. Tests Required

- Run `go mod tidy -diff`, `go vet ./...`, `go test -race ./...`, a full Go
  build, and pinned `govulncheck` with the safe Go toolchain.
- Run pnpm 9.15.x frozen install, lint, behavior tests, production build,
  `audit:prod`, and `audit:dev`.
- Run `actionlint` after editing either workflow and `git diff --check` before
  commit.

### 7. Wrong vs Correct

#### Wrong

```yaml
with:
  go-version: "1.25"
```

#### Correct

```yaml
with:
  go-version: "1.25.12"
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
- Default download base is always official GitHub (`github.com`). Reachability
  probes may only produce clearer errors; they must never select a third-party
  host.
- A validated `DOWNLOAD_BASE_URL` is the only non-GitHub source path and means
  the operator already trusts that mirror. Same-origin `checksums.txt` proves
  integrity, not independent publisher identity. Saved `DOWNLOAD_BASE_URL`
  values come from prior explicit configuration only and are re-validated
  before download (normalized absolute HTTPS, or HTTP only with
  `ALLOW_INSECURE_DOWNLOAD_BASE_URL=true`). Reject userinfo, query, fragment,
  empty host, whitespace/semicolons, and non-HTTP(S) schemes even when the
  insecure opt-in is set. Logs must never echo raw URLs containing credentials
  or tokens; curl uses explicit `--proto` / `--proto-redir` allow-lists shared
  by package and checksum downloads.

### 4. Validation & Error Matrix

| Input | Result |
| --- | --- |
| `latest` / `nightly` | accepted |
| `v0.1.4` | accepted |
| empty final value, path separators, whitespace, shell punctuation | non-zero validation failure |
| GitHub unreachable and no `DOWNLOAD_BASE_URL` | non-zero failure before download/snapshot |
| HTTP mirror without explicit insecure opt-in | non-zero validation failure |
| `file://` / `ftp://` / other non-HTTP(S) with insecure opt-in | non-zero validation failure before download |
| userinfo, query, fragment, or empty host | non-zero validation failure; error omits raw secret |

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
   link, plus a runtime state file recording prior service present/enabled/active
   and Nginx site/enablement. The manifest records both existing and absent
   targets under a mode-`0700` backup directory; env, manifest, and runtime
   state are mode `0600`.
5. Copy each candidate to `<target>.new.$$` in the target directory, set
   owner/mode, then use `mv -T` for same-filesystem atomic replacement.
6. Run daemon reload, unit enablement, `nginx -t`, service restart,
   `is-active`, Nginx reload, and loopback `/healthz` retry validation.
7. On success remove the backup and only then print success. On failure: stop any
   currently active unit, restore originally present targets / remove originally
   absent targets, `daemon-reload`, reconcile prior enabled/disabled and
   active/inactive service state (do not start a previously inactive unit), run
   `nginx -t` and reload Nginx even for first-install site removal. Preserve
   root-only recovery files when rollback itself is incomplete.
8. Generated Nginx `/api/` `proxy_read_timeout` must be 60s (SPA `/` may stay
   30s) so it exceeds the 30s classroom refresh and 45s Go write budgets.

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
| restart, `is-active`, reload, or loopback health failure | restore files; stop current unit; restore prior active/enabled state |
| first-install commit failure | remove newly created transaction targets; stop new unit; reload Nginx |
| previously inactive/disabled upgrade failure | restore files; leave service inactive/disabled |
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
- Can every production runtime value be traced from `config.Load` through
  `main.go` constructors without downstream environment reads or globals?

## Forbidden Patterns

- Network-dependent unit tests that do not skip without credentials.
- Global mutable state in `service/` that bypasses injected test instances.
- Package-level config/cache/HTTP singletons or hot-path runtime `os.Getenv`
  calls outside the startup boundary.
- Raw `err.Error()` in API responses.
- Logging secrets or raw upstream payloads.
- Reintroducing local timetable persistence as an incidental implementation
  detail.
