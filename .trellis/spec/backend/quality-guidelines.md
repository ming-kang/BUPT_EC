# Quality Guidelines

## Overview

Backend changes should preserve the current small-service architecture: thin
`net/http` handlers, injectable service dependencies, no local timetable
database, safe JW error handling, and source-backed API contracts. Keep code
readable and verify with the same commands CI uses.

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
  `ClassroomServiceOptions` and a `JWClient`; tests create isolated services
  with `newTestService` / `newTestServiceWithOptions`
  (`service/testsupport_test.go`), injecting a thread-safe fake `Clock` and
  fixed `BackoffRandom` when asserting time or backoff deadlines, and seed the
  service's internal day cache through the `service/export_test.go` seams
  (`seedCache`) instead of injecting a store.
- Keep runtime environment access in `config.Load` plus the `main.go`
  composition root. Tests pass map-backed lookups and constructor values rather
  than mutating config/cache globals.
- Use contexts for external work. JW login, API URL fetch, classroom refreshes,
  and HTTP requests all use bounded contexts.
- Keep handlers thin and service logic testable without the HTTP layer.
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

- `service/testsupport_test.go` defines `mockJWClient` and `newTestService`.
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
- Real-network tests live in `service/integration_test.go` behind
  `//go:build integration` (`go test -tags integration ./service`). `TestLogin`
  requires `JW_USERNAME`/`JW_PASSWORD`; query integration tests may use that
  pair or `JW_TOKEN`. All must skip cleanly when their required credentials are
  missing.
- Handler tests should inject deterministic fakes through `NewHTTPServer` and
  use `httptest` against `HTTPServer.Routes()` when middleware behavior such
  as `/api` `log_id` correlation (`X-Log-Id`), gzip, caching headers, or the
  fallback matters. Gzip assertions must go through the production
  `newGzipWrapper` configuration (never a test-private wrapper) and use
  bodies ≥1KB, since `MinSize` is 1024.
- Metrics endpoint tests must use a real `promhttp.HandlerFor` over
  `NewPrometheusMetrics()`'s isolated registry (not a fixed fake body), with
  `DisableCompression: true` matching production, and assert identity/gzip
  bodies parse as Prometheus text after at most one decompress.
- Login metric tests use a recording `RuntimeMetrics` (or Gather on an isolated
  registry) and assert one observation per shared network login, correct
  `source` provenance, non-negative duration, and no secret labels.
- Frontend `*.lifecycle.test.tsx` and `components/*.test.tsx` files mount real
  hooks/components under jsdom via `@testing-library/react`, opting in with a
  file-level `@vitest-environment jsdom` directive; pure helper tests remain on
  the default node env configured in `vite.config.js:50`. Shared DOM-only stubs
  (`matchMedia` for antd) live in `frontend/src/test/setup.ts`, which guards on
  `typeof window` so node-environment tests are unaffected.
- Tests that mount the SWR-backed hook must isolate the module-global cache
  with `<SWRConfig value={{ provider: () => new Map() }}>`; see the SWR gotchas
  in api-contract.md's "Frontend Snapshot Validity and Reload Backoff".

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

`Taskfile.yml` provides the local development entry points and must stay in
sync with `.github/workflows/quality.yml` (the sync note is at the top of
both files):

```bash
task test                 # go test -race ./...
task installer:generate   # regenerate tracked scripts/install.sh from fragments
task installer:check      # generator drift + recursive syntax/test/ShellCheck gate
task check                # Go/frontend checks plus installer:check (not bundle size)
task build                # frontend build → copy to web/dist → go build -tags embed_assets
task vuln                 # pinned govulncheck
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
go build ./...
rm -rf web/dist && cp -r frontend/dist web/dist && go build -tags embed_assets ./...
govulncheck ./...
bash scripts/generate-install.sh --check
find scripts -type f -name '*.sh' -exec bash -c 'for script; do bash -n "$script" || exit 1; done' bash {} +
bash scripts/install_test.sh
find scripts -type f -name '*.sh' -exec shellcheck {} +
cd frontend && pnpm lint
cd frontend && pnpm build
cd frontend && node scripts/check-bundle-size.mjs
cd frontend && pnpm audit:prod
cd frontend && pnpm audit:dev
```

The bundle size budget step ("Check bundle size budget", right after "Build
frontend" in `quality.yml`) is the one gate `task check` deliberately omits,
because it needs a fresh production build and that build dominates local check
latency. The omission is intentional and documented in place — the comment on
`Taskfile.yml`'s `check` task names the missing step and spells out the manual
command; keep both sides in sync if either changes. Run
`pnpm -C frontend build && pnpm -C frontend size` (or
`node frontend/scripts/check-bundle-size.mjs`) by hand when touching frontend
dependencies, chunk splitting, or anything that lands in `dist/`.

`frontend/scripts/check-bundle-size.mjs` is dependency-free (`node:fs` +
`node:zlib` only) and measures **gzip level 9 over every `js`/`css`/`html`/`svg`
file under `frontend/dist`**. That metric is not Vite's console gzip column
(zlib default level) — never mix the two when arguing about a regression. The
budget is `BUDGET_BYTES = 230_888` (check-bundle-size.mjs:28), derived as
`ceil(measured total × 1.10)` from the 209,898 B measured on 2026-07-27, with
the 10% headroom absorbing dependency patch churn. Raising it requires editing
that constant together with a recorded justification in the comment block above
it; the file also records the pre-modernization baseline (293,407 B) and the
accepted trade-off that this is a *total* budget, so grouping antd into one
eagerly preloaded vendor chunk raised first load while cutting the total ~28%.

The tag split in that list is deliberate: `go vet` / `go test -race` /
`go build ./...` stay tag-less as the bare-clone gate (they must pass on a
checkout with no frontend build, via the `web` placeholder branch), while
the separate `-tags embed_assets` build step catches embed pattern breakage
before release.

Frontend audit policy is executable through `frontend/package.json` so local,
PR, and release checks share the same thresholds: production dependencies fail
at moderate or above; the full development toolchain fails at high or above.
Generate and verify `frontend/pnpm-lock.yaml` with pnpm 9.15.x.

### Convention: CI Workflow Editing Rules

**What**: Constraints that hold across `.github/workflows/*.yml` edits.

- Every third-party action is pinned to a 40-char commit SHA with a trailing
  version comment. Never introduce an action you cannot pin (prefer reusing an
  action+SHA already present in the repo, or a `go run <module>@<version>`
  equivalent, as done for `govulncheck`).
- Every job carries `timeout-minutes` — except jobs that call a reusable
  workflow (`uses:` form), where GitHub Actions rejects the key; their
  effective timeout is the inner job's `timeout-minutes` inside
  `quality.yml`.
- `frontend/dist` is built once in the `quality.yml` reusable gate and shared
  as the `frontend-dist` artifact (retention 3 days). `release.yml`'s
  `build-go` downloads it to `path: web/dist` (release.yml:45-49) before
  `go build -tags embed_assets` so `//go:embed all:dist` in the `web`
  package resolves; keep the upload/download artifact names and the
  download path in sync when renaming.
- `quality.yml`'s `go vet` / `go test -race` / `go build ./...` steps run
  **without** `-tags embed_assets` on purpose: they gate the bare-clone
  (placeholder) build. The dedicated "Build with embedded assets" step
  (copy `frontend/dist` → `web/dist`, then `go build -tags embed_assets
  ./...`) is the embed-mode gate; do not "fix" the tag-less steps by adding
  the tag.
- Release binaries build with
  `go build -trimpath -tags embed_assets -ldflags "-s -w -X main.version=<value>"`;
  the version value is the tag name for tag builds and `main-<short-sha>`
  otherwise. Keep the `-X` target in sync with the `version` variable in
  `main.go` (see api-contract.md "Health and Readiness" for the injection
  gotchas) and keep `task build` in `Taskfile.yml` aligned with the same
  flag set.
- `release.yml` publishes on `v*` tag pushes only. Pushes to `main` and manual
  dispatches run the *identical* pack / checksum / attest path and upload the
  result as workflow artifacts without publishing. Do not "optimize" the
  dry-run by skipping those steps: exercising the packaging path on every
  merge is the only thing that catches a broken tarball or checksum step
  before release day.

**Why**: The pinning rule is a supply-chain gate; the artifact contract spans
two workflow files and breaks silently when only one side is renamed; the
`timeout-minutes` exception avoids a recurring "add timeout to every job"
false fix that GitHub rejects at parse time.

### Convention: Frontend Dependency Lines and Build Target

**What**: Ranges in `frontend/package.json` are deliberate *line* locks, not
staleness. Upgrade within a line freely (patch/minor); crossing a line is its
own evaluated change with its own visual/behavior regression pass.

- `antd` stays on `^5` (currently `^5.29.3`). antd 6 is a separate migration,
  not a dependency bump.
- `@ant-design/v5-patch-for-react-19` is required for the antd v5 React 19
  bridge: React 19 removed the legacy dynamic-render path used by antd v5
  Button waves and static overlays. Import the patch before rendering in
  `frontend/src/main.tsx`. Direct component tests bypass that entrypoint, so
  `frontend/src/test/setup.ts` must await the same patch when `window` exists;
  do not statically load the antd graph into pure node tests. The Button/Modal
  component suites are the compatibility regression gate.
- `react` / `react-dom` / `@types/react*` stay on the `^19.2` line.
- `vite` stays on `^7`, and `@vitejs/plugin-react` must stay on `^5`: its 6.x
  peer range is `vite ^8` only, so bumping the plugin alone silently drags the
  bundler major or breaks install resolution.
- `vitest` stays on `^3` (natively compatible with Vite 7) and `eslint` /
  `@eslint/js` on the `^9` maintenance line.
- **Never run `pnpm update --latest`** (or `pnpm up --latest`) in `frontend/`.
  Every one of the packages above has a `latest` dist-tag beyond its pinned
  line, so one command crosses several majors at once and destroys regression
  attribution. Bump the specific package, then run lint/test/build/size/audits.
- `pnpm.overrides` is intentionally absent. It previously held four CVE
  pins that became dead weight after a fresh resolve. If an audit goes red
  again, add a single entry naming the advisory in a comment — never a blanket
  override block.
- `build.target: ['es2020', 'safari14']` in `vite.config.js:33` is an explicit
  rejection of Vite 7's new default baseline (Safari 16 / Chrome 107). The
  default would silently drop iOS < 16 devices, which are a real share of the
  campus user base, and nothing in the test suite would fail. Keep the explicit
  target until raising the baseline is evaluated on its own.

**Why**: Each of these has a failure mode that no gate catches: a plugin peer
range that quietly forces a bundler major, a browser baseline change that only
shows up on other people's phones, and a single `--latest` invocation that
makes a regression impossible to bisect.

## Scenario: Dependency Security Baseline

### 1. Scope / Trigger

Apply this contract whenever Go or frontend dependencies, toolchain versions,
lockfiles, lint configuration, or CI/release quality gates change.

### 2. Signatures

```bash
GOTOOLCHAIN=go1.25.13 go run golang.org/x/vuln/cmd/govulncheck@v1.5.0 ./...
pnpm --dir frontend audit:prod
pnpm --dir frontend audit:dev
```

### 3. Contracts

- `go.mod` and every `actions/setup-go` step use Go `1.25.13`; Go 1.26 users
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
  go-version: "1.25.13"
```

The Go build embeds the frontend only when built with `-tags embed_assets`,
which expects `frontend/dist` copied to `web/dist` (see the `web` package in
directory-structure.md). `task build` runs the whole chain; default builds
compile without any frontend output and serve a placeholder page.

## Frontend and Cross-Layer Quality

Backend API changes often require frontend changes because the React app reads
the backend payload directly. Before changing `service/model/realtime_data.go`,
`classroom_builder.go`, or `handler.go`, inspect these frontend files:

- `frontend/src/useTodayClassrooms.ts` for the SWR call, the fetch boundary and
  the render-time envelope derivation.
- `frontend/src/todayClassroomsResponse.ts` for API envelope normalization.
- `frontend/src/apiError.ts` for the structured fetch-boundary error
  (`status` / `code` / `logId`).
- `frontend/src/components/BuildingPicker.tsx` for building assumptions.
- `frontend/src/components/ClassTimePicker.tsx` for campus node assumptions.
- `frontend/src/components/TodayClassroomTable.tsx` for `free_nodes` filtering.

Read api-contract.md's "Scenario: Frontend Snapshot Validity and Reload
Backoff" **before** touching the data layer: SWR configuration, the retry
ladder, visibility handling and the never-falsy poll interval are contract
surface, not implementation detail.

Frontend code uses ES modules, React hooks, 2-space indentation, PascalCase
component filenames, matching component CSS files, and shared selection state
through `useSelection()` rather than prop drilling. Prefer derived render-time
values over `useEffect`-synced state; keep effects for genuine external
synchronization (store convergence, timers, DOM listeners). The main classroom
table is native `<table>` markup styled through the shared `.ec-table` system,
not `antd`'s `Table` — do not reintroduce it, it is the single largest bundle
line item.

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
assets or installer behavior, update the `scripts/installer/` sources,
`scripts/generate-install.sh`, generated `scripts/install.sh`,
`scripts/release.sh`, `.github/workflows/release.yml`, and `docs/release.md`
together.

## Installer Guidance

Installer mode, configuration, generated-artifact, staging, release-layout,
and transaction/rollback contracts live in
[Installer Guidelines](./installer-guidelines.md). Read that file before
changing any installer source, test, quality gate, or release asset.

## Security Checklist

- Never commit real `.env` files, `JW_USERNAME`, `JW_PASSWORD`, `JW_TOKEN`, or
  generated `run_log/` files.
- Keep JW API URL validation restricted to HTTPS BUPT hosts.
- Keep the AES key in `service/crypto.go` aligned with the JW protocol; do not
  change it casually.
- Keep `/etc/bupt-ec/bupt-ec.env` documented as a regular root-owned mode
  `0600` file in a root-controlled non-writable-by-others directory; the
  installer safe-loader rejects symlink or unsafe ownership/mode layouts.
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
