# 重构维护性总任务

## Goal

Create a parent Trellis task that coordinates a low-risk, incremental
maintainability refactor for the BUPT_EC project. The work should make the Go
backend, React frontend, tests, configuration, and delivery pipeline easier to
develop and maintain without changing the product behavior: same-day BUPT empty
classroom querying backed by the JW HTTP system, embedded frontend assets, and
process-local stale-while-revalidate cache.

The parent task is not a single large implementation PR. It is the planning and
tracking container for a sequence of smaller child tasks that can be designed,
implemented, validated, and committed one by one.

## Background and Confirmed Facts

- The project is a Go module named `BUPT_EC` with a React/Vite frontend embedded
  by the Go backend.
- Current backend entry points are `main.go`, `router.go`, and `handler.go`.
  `main.go` owns process startup, constructs the single
  `service.ClassroomService` instance, and injects it into `HTTPServer` before
  route registration.
- The previous `07-06-organize-project-structure` task deliberately kept the Go
  main package at the repository root. A `cmd/bupt-ec/` migration is deferred
  because `router.go` embeds `frontend/dist` and `go:embed` cannot reference
  parent paths with `..`.
- `service/` is already split into focused files, but it still combines several
  architectural layers: application use case, JW gateway protocol, token/API URL
  state, refresh coordination, cache policy, runtime readiness status, error
  mapping, and classroom normalization.
- All mutable classroom-query runtime state is intended to live on
  `ClassroomService`; package-level mutable globals inside `service/` are an
  anti-pattern.
- `handler.go` is intentionally thin. The injected HTTP boundary child replaced
  the old package-level handler dependency with deterministic `HTTPServer`
  fakes in handler tests.
- The backend does not maintain a timetable database. It fetches same-day
  classroom availability from the BUPT JW HTTP API and stores one process-local
  cache entry for the current day.
- Existing cache semantics are product-critical: fresh for about five minutes,
  stale usable until local end of day, no cross-day reuse, shared refresh attempt
  for concurrent callers, and failed refresh backoff.
- `JWClient` is an important external-system seam and should be preserved during
  refactors.
- The frontend currently lives under `frontend/src/` with components in
  `frontend/src/components/`, shared selection state in `selectionContext.js` and
  `SelectionProvider.jsx`, and classroom fetching/normalization in
  `useTodayClassrooms.js`.
- Frontend build/lint checks exist, and the behavior-safety child added focused
  Vitest tests for API normalization and selection-state behavior.
- CI/release automation is already mature, but quality-gate logic is duplicated
  across workflows and runtime readiness could expose more non-secret build/cache
  diagnostics.
- User-facing behavior, endpoints, configuration, release process, and docs must
  remain synchronized with `README.md`, `docs/`, `.env.example`, and
  `CHANGELOG.md` when changed.

## Requirements

- Treat this task as a parent roadmap task with explicit child tasks linked in
  Trellis rather than as one large implementation branch.
- Preserve current product behavior and public API compatibility unless a child
  task explicitly proposes and receives approval for a behavior change.
- Preserve the no-local-database architecture and same-day JW-backed query model.
- Preserve the existing stale-while-revalidate cache contract, including
  cross-day cache rejection and shared refresh coordination.
- Preserve `JWClient` or an equivalent mockable upstream seam so service tests do
  not require network access.
- Begin with behavior-safety work before high-churn structural package moves.
- Add or expand tests only when they materially reduce regression risk; avoid
  unnecessary coverage work, low-value smoke tests, and snapshot tests that only
  restate implementation structure.
- Prefer small, reviewable child tasks that can each pass focused validation.
- Keep docs/specs/changelog updated when child tasks alter contributor-visible
  structure, runtime behavior, endpoints, configuration, or release workflow.

## Candidate Child Task Roadmap

1. Behavior safety net (`07-06-behavior-safety-net`, completed):
   - add or strengthen backend characterization tests for cache freshness,
     refresh coordination, token/API URL behavior, JW fixtures, and public
     payload normalization;
   - add a frontend test setup for classroom API normalization and selection
     state transitions.
2. Injected HTTP boundary (`07-06-injected-http-boundary`, completed):
   - replace package-level handler dependency with an injected HTTP server or
     handler struct;
   - keep route registration, gzip, embedded SPA fallback, `/api/get_data`,
     `/healthz`, and `/readyz` behavior unchanged.
3. Configuration and dependency injection cleanup:
   - centralize startup configuration validation;
   - make non-secret runtime constants and dependencies such as clock/HTTP
     client/test doubles more explicit.
4. Backend responsibility split:
   - extract classroom normalization/domain helpers as pure logic;
   - clarify JW gateway/auth/API URL boundaries;
   - model cache freshness as explicit decisions instead of scattered implicit
     checks.
5. Frontend classroom feature boundary:
   - separate API client, normalization/model selectors, selection reducer, and
     presentational components;
   - centralize reusable CSS tokens/layout rules without introducing a heavy UI
     framework.
6. Delivery and operations cleanup:
   - deduplicate CI/release quality gates;
   - expose non-secret build/cache/readiness diagnostics;
   - harden installer behavior where useful.

## Out of Scope for the Parent Task Unless Re-approved

- Rewriting the application in a different backend or frontend framework.
- Introducing a local timetable database, ORM, migrations, or persistent cache.
- Moving the Go application to `cmd/bupt-ec/` before an explicit embed-path
  migration design exists.
- Replacing Gin, the current go-cache-backed process cache, or the release asset
  layout without a specific child-task justification.
- Full frontend TypeScript migration as a first step.
- Large mixed PRs that combine behavior change, package movement, UI redesign,
  and delivery workflow changes.

## Approved Decisions

- The first child task will be behavior-safety work rather than the injected HTTP
  boundary. This keeps later structural refactors lower risk by locking down the
  current cache, JW, handler, API-envelope, and frontend selection behavior
  first.

## Linked Child Tasks

- `07-06-behavior-safety-net`: completed. Added the first characterization test
  layer before structural refactors: focused backend cache/normalization/API
  envelope tests plus strict frontend Vitest behavior tests.
- `07-06-injected-http-boundary`: completed. Removed the HTTP layer's hidden
  package-level `classroomService` dependency by introducing `HTTPServer`,
  preserving public route behavior, logging correlation, gzip, readiness, and
  SPA fallback.
- `07-10-runtime-config-composition`: completed. Made `config.Load` the single
  production environment boundary, built dependencies in `main.go`, removed
  downstream environment reads, and injected the service/config snapshot into
  the HTTP boundary.
- `07-10-reliability-audit-hardening`: completed. Coordinated six archived
  children covering refresh full/partial/failure outcomes, frontend business-day
  cache validity, cancellable warmup, concurrent token recovery, installer
  release selection, and transactional installer rollback.
- `07-10-dependency-security-refresh`: completed. Raised the Go security floor,
  patched the reachable quic-go advisory, refreshed the Vite/ESLint toolchain,
  and added production/full dependency audit gates to PR and release workflows.

## Progress Notes

- `07-06-behavior-safety-net` completed validation with `gofmt -l .`,
  `go test ./...`, `go vet ./...`, `pnpm --dir "frontend" lint`,
  `pnpm --dir "frontend" test`, and `pnpm --dir "frontend" build`.
- `07-06-injected-http-boundary` completed validation with `gofmt -l .`,
  `go test ./...`, and `go vet ./...`. It also added deterministic `GetData`
  error-envelope coverage through the injected HTTP boundary.
- `07-10-runtime-config-composition` and `07-10-reliability-audit-hardening`
  completed and were archived on 2026-07-10.
- `07-10-dependency-security-refresh` completed and was archived on 2026-07-10;
  Go 1.25.12 `govulncheck` reports zero reachable vulnerabilities and both pnpm
  audit policies pass with no known vulnerabilities.
- `07-10-installer-provenance-hardening` completed on 2026-07-10: removed
  automatic `gh-v6.com` fallback, GitHub-unreachable fails closed with explicit
  `DOWNLOAD_BASE_URL` guidance, and docs/tests/spec were synchronized.
- A parent-level integration re-review on 2026-07-10 found additional security,
  HTTP-boundary, installer, frontend, and observability work. The parent remains
  open until those findings are handled through reviewable child tasks.

## 2026-07-10 Integration Re-review

### Original Architecture Findings

| # | Original finding | Status | Re-review result |
| --- | --- | --- | --- |
| 1 | One campus failure discarded the whole refresh | Resolved | Refresh outcomes explicitly distinguish full, partial, and total failure; successful campuses are cached and failed campuses are identified. |
| 2 | Runtime config and credentials leaked into service hot paths | Resolved | `config.Load` snapshots the environment once and `main.go` injects credentials, campuses, cache, HTTP client, JW client, and service. The two-campus list remains an intentional fixed product value. |
| 3 | Business-day behavior depended on host timezone | Resolved | Cache date and expiry use the Asia/Shanghai business calendar with next-midnight expiry and cross-day rejection. |
| 4 | Logs/readiness only; no trend metrics | Open | Runtime status is richer, but there are still no counters, latency histograms, cache-hit trends, or alertable upstream metrics. |
| 5 | Fixed retry behavior and no JW circuit breaker | Partial | Warmup now has bounded 30s/1m/2m/5m retry and midnight jitter, but request-triggered refresh still uses a fixed 30-second backoff and has no adaptive breaker. |
| 6 | Generic `CacheStore` leaks go-cache details | Open | `CacheStore` still exposes `Get/Set/Delete`, callers still type-assert the cached model, `Delete` is unused, and the cache default TTL remains misleading because service writes use explicit expiry. |
| 7 | Inconsistent time source | Partial | Cache policy uses `s.now`, but refresh/login status and elapsed timestamps still call `time.Now`/`time.Since` directly; the clock is not a public constructor option. |
| 8 | Hand-written frontend request/reload state machine | Partial | Cross-day validity, stale preservation, and bounded retry are tested, but the hook still owns raw `fetch`, `AbortController`, and timers with no explicit request timeout or visibility pause. |
| 9 | Reducer performs `localStorage` side effects | Open | Storage errors are handled and tested, but the reducer is still impure; persistence should move to provider effects or a persisted-state adapter. |
| 10 | Security/protocol and frontend test blind spots | Partial | URL validation and many cache/token/frontend pure behaviors are covered. A JW AES known vector, the actual hook/effect lifecycle, and a non-skipped upstream contract path remain uncovered. |
| 11 | Process-local single-instance assumption | Partial | Operations/development docs now state that instances do not share cache, but do not yet declare the recommended topology or a Redis/leader/fetcher scaling route. |
| 12 | Repository/tooling hygiene and embedded dist workflow | Accepted decision | Trellis and supported editor integration are intentionally versioned; generated Python caches are absent and `.template-hashes.json` is ignored. Local Go builds intentionally require a prior frontend build and CI supplies the artifact. |

### New Confirmed Findings

#### P0 — release/security blockers

1. `govulncheck ./...` fails on reachable `GO-2026-5676` through
   `github.com/quic-go/quic-go v0.59.0`; the fixed version is `v0.59.1`. The
   current CI/release quality gate will fail until this dependency is updated.
2. The local Go `1.26.4` toolchain is affected by standard-library TLS advisory
   `GO-2026-5856`; fixed releases are Go `1.25.12` and `1.26.5`. CI currently
   requests broad `1.25`, so the repository should set an explicit safe minimum
   patch and local development must upgrade.
3. `pnpm audit --prod` reports one moderate runtime dependency advisory in
   `@babel/runtime 7.23.7`. Full audit reports 11 high, 20 moderate, and 4 low
   findings, including multiple Vite 5.0.10/Rollup/esbuild development-server
   and build-tool advisories. CI currently has no pnpm audit gate.

#### P1 — correctness and security

1. The installer automatically trusts `gh-v6.com` when GitHub is unreachable and
   downloads both the archive and checksum from that same third-party origin.
   A co-fetched checksum does not independently authenticate a root-installed
   binary; fallback must become explicit or verify GitHub provenance/signatures.
2. Late first-install rollback is incomplete. If service restart/Nginx reload has
   succeeded and a later active/health check fails, rollback removes files but
   does not stop the newly started service and does not reload Nginx when no old
   service/site existed. Tests cover only first-install failure before restart.
3. Generated Nginx uses `proxy_read_timeout 30s`, equal to
   `ClassroomRefreshLimit`; a cold refresh near the backend limit can be cut off
   with a proxy timeout before the 45-second Go write budget can return JSON.
4. Unknown `/api/*` requests do not traverse the `/api` group middleware, so
   their JSON 404 responses lack the promised `LogID` header/body correlation.
   Cold refresh work also starts from `context.Background`, so its failure log
   cannot be correlated with the request `log_id` returned to the client.
5. `safeRemoteMessage` only trims upstream `Msg` values before they are included
   in internal errors and warning logs. It needs bounded length and explicit
   secret/account-detail sanitization to satisfy the logging contract.
6. On a cold partial response where the preferred Shahe campus failed, the
   frontend still auto-selects its empty placeholder instead of the campus that
   has usable data, making a successful partial result appear empty by default.

#### P2 — robustness and maintainability

1. Gzip negotiation uses `strings.Contains`; it compresses
   `Accept-Encoding: gzip;q=0`, misses case/weight semantics, and has no
   regression test for refusal.
2. Frontend fetches have no explicit timeout and polling is not paused while the
   page is hidden. The 5-second stale poll can also conflict with the installed
   Nginx limit of 30 API requests/minute for several users/tabs sharing one IP.
3. The production CSP disallows inline scripts, so it blocks the inline dark-mode
   bootstrap in `frontend/index.html`; the app recovers after React mounts but
   the intended no-flash behavior does not work in production.
4. `CampusSettingsModal` labels `updated_at` as the last refresh attempt even
   though a total failed attempt does not update that value.
5. `logs.Init` exits the process internally on directory creation failure and
   `NewHTTPServer` accepts a nil service, weakening otherwise explicit
   constructor/startup error handling.
6. CI and release duplicate nearly the same quality-gate steps, while dependency
   audit policy is absent from both.
7. Stable release publishing supplies both `body_path` and
   `generate_release_notes: true`; GitHub can append generated notes, which
   conflicts with the documented contract that the matching changelog section
   is used verbatim.

### Re-review Validation

- Passed: `gofmt -l .`, `go vet ./...`, `go test ./...`,
  `go test -race ./...`, temporary-path `go build -v ./`, Staticcheck,
  frontend lint, 44 Vitest tests, frontend build, `bash -n` for all four shell
  scripts, `bash scripts/install_test.sh`, and `git diff --check`.
- Blocked by findings: `govulncheck ./...` and both production/full pnpm audits.
- Not rerun locally: ShellCheck is not installed in this Windows environment;
  CI installs and runs it.

## Remaining Child-task Roadmap

1. **Dependency security refresh (P0, completed 2026-07-10)**: updated quic-go
   and the safe Go patch floor, refreshed Vite/ESLint/Babel dependencies, and
   added production/dev audit policy in separate Go and frontend commits.
2. **`07-10-installer-provenance-hardening` (P1, completed)**: remove implicit
   third-party proxy trust and require an explicit operator-selected mirror.
3. **`07-10-installer-late-rollback-timeouts` (P1, planning)**: after item 2,
   reconcile first-install service/Nginx state on late failure and align the
   `/api/` proxy timeout with backend/server budgets.
4. **`07-10-protocol-lifecycle-test-coverage` (P1, planning)**: add independent
   AES vectors, deterministic JW protocol fixtures, and a real React hook
   lifecycle harness before higher-churn protocol/frontend work.
5. **`07-10-http-protocol-correlation` (P1, planning)**: correct gzip
   negotiation, cover unknown API responses with log IDs, and preserve request
   values in detached shared refresh work.
6. **`07-10-upstream-error-startup-safety` (P1, planning)**: after item 4,
   bound/redact upstream messages and make logging/HTTP constructor failures
   propagate through the composition root.
7. **`07-10-frontend-partial-data-semantics` (P1, planning)**: choose a campus
   with usable data on cold partial success and correct the `updated_at` label.
8. **`07-10-frontend-lifecycle-state-hardening` (P1, planning)**: after items
   3, 4, and 7, add fetch timeout/visibility/rate-aware scheduling, pure
   preference persistence, and a CSP-compliant dark bootstrap.
9. **`07-10-typed-cache-clock` (P2, planning)**: replace the generic cache seam
   and inject one clock across cache, refresh, login, and runtime status.
10. **`07-10-runtime-observability-protection` (P2, planning)**: after item 9,
    add low-cardinality metrics and deterministic adaptive JW backoff.
11. **`07-10-delivery-workflow-release-contract` (P3, planning)**: after the
    quality commands stabilize, reuse one PR/main gate and keep stable release
    notes changelog-only.
12. **`07-10-deployment-topology-guidance` (P3, planning)**: last, document the
    supported single-instance topology and future shared-cache/leader options
    without claiming they are implemented.

Ten remaining child directories contain converged `prd.md`, `design.md`, and
`implement.md` artifacts. They remain in `planning` until the user reviews that
child's final scope (user authorized sequential completion on 2026-07-10).

## Acceptance Criteria

- [x] The parent task has a reviewed child-task decomposition with clear order,
      dependencies, and scope boundaries.
- [x] Each completed implementation child task has its own PRD and, when complex, design
      and implementation plan before code changes begin.
- [x] Behavior-safety child work precedes structural package moves.
- [x] Each completed child task documents validation commands and any skipped
      checks.
- [x] Public behavior remains compatible unless an approved child task documents
      the behavior change and updates docs/changelog accordingly.
- [x] Trellis parent/child links are maintained so completed progress can be inspected
      from this parent task.
- [x] P0 dependency/toolchain findings pass `govulncheck` and the agreed pnpm
      audit policy.
- [x] Each of the eleven planned remaining children is reviewed before activation,
      and implementation follows the dependency order above.
- [x] P1 installer, HTTP correlation/error-sanitization, and partial-campus UX
      findings are completed through linked child tasks.
- [x] Remaining P2/P3 work is either completed or explicitly accepted/deferred
      with rationale before the parent is archived.

## Notes

- The original first three maintainability children, the six-child reliability
  audit, the P0 dependency security refresh, and installer provenance hardening
  are complete. The next planned task is
  `07-10-installer-late-rollback-timeouts`.
