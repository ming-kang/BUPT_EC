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

## Progress Notes

- `07-06-behavior-safety-net` completed validation with `gofmt -l .`,
  `go test ./...`, `go vet ./...`, `pnpm --dir "frontend" lint`,
  `pnpm --dir "frontend" test`, and `pnpm --dir "frontend" build`.
- `07-06-injected-http-boundary` completed validation with `gofmt -l .`,
  `go test ./...`, and `go vet ./...`. It also added deterministic `GetData`
  error-envelope coverage through the injected HTTP boundary.
- The next recommended child task is configuration and dependency injection
  cleanup.

## Acceptance Criteria

- [ ] The parent task has a reviewed child-task decomposition with clear order,
      dependencies, and scope boundaries.
- [ ] Each implementation child task has its own PRD and, when complex, design
      and implementation plan before code changes begin.
- [ ] Behavior-safety child work precedes structural package moves.
- [ ] Each completed child task documents validation commands and any skipped
      checks.
- [ ] Public behavior remains compatible unless an approved child task documents
      the behavior change and updates docs/changelog accordingly.
- [ ] Trellis parent/child links are maintained so progress can be inspected
      from this parent task.

## Notes

- The current recommended order is behavior safety net first, then injected HTTP
  boundary, then configuration/dependency cleanup, then deeper backend/frontend
  responsibility splits, then delivery cleanup.
