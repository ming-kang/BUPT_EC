# 行为保护网

## Goal

Create a behavior-focused safety net before structural refactors begin. The
task should make the current backend and frontend behavior easier to verify so
later child tasks can remove hidden globals, split service responsibilities, and
reorganize frontend feature boundaries with lower regression risk.

This task should not intentionally change user-visible product behavior. Small
testability-oriented extractions are allowed when they make existing behavior
observable as pure functions, but package moves, API changes, UI redesign, and
runtime configuration changes belong to later child tasks.

## Background and Confirmed Facts

- This child task is linked to parent task `07-06-refactor-maintainability`.
- The parent task approved behavior-safety work as the first refactor step.
- `service/realtime_data_test.go` already contains useful backend testing
  infrastructure: `mockJWClient`, `newTestService`, isolated `go-cache`
  instances, and tests for room parsing, room deduplication, JW API URL
  validation, `JW_TOKEN` override behavior, and cross-day cache rejection.
- The backend runtime cache contract is documented in
  `.trellis/spec/backend/runtime-state-and-cache.md`: one
  `TODAY_CLASSROOMS_CACHE` value, about five minutes fresh, stale usable until
  local end of day, no cross-day reuse, shared refresh attempt for concurrent
  callers, and failed refresh backoff.
- `handler_test.go` already covers readiness requiring usable cache, SPA
  fallback versus unknown `/api/*` JSON 404, and gzip behavior.
- `JWClient` is the existing seam for mocking the BUPT JW upstream. Unit tests
  should continue to avoid network access; integration tests should skip unless
  `JW_TOKEN` or `JW_USERNAME`/`JW_PASSWORD` is provided.
- `frontend/package.json` has `dev`, `build`, `lint`, and `preview` scripts but
  no frontend test script or test framework.
- `frontend/src/useTodayClassrooms.js` mixes fetch orchestration with response
  parsing and normalization through unexported helpers such as
  `normalizeResponse` and `readJson`.
- `frontend/src/selectionContext.js` already has a reducer, but reducer cases
  also write to `localStorage`, and the initializer reads `localStorage`; tests
  will need either a browser-like environment or a small testability seam.

## Requirements

- Add or strengthen characterization tests for behavior that later refactors are
  most likely to break.
- Keep the production API contract for `/api/get_data`, `/healthz`, and
  `/readyz` unchanged.
- Keep the no-local-database architecture and same-day JW-backed query model
  unchanged.
- Preserve the stale-while-revalidate cache contract, including fresh window,
  stale serving, cross-day rejection, shared refresh attempts, and refresh
  backoff.
- Preserve `JWClient` or an equivalent mockable upstream seam for backend tests.
- Backend unit tests must be deterministic and must not require real BUPT JW
  credentials.
- Integration tests that use the real JW system must continue to skip cleanly
  when credentials are absent.
- Frontend tests should start with high-value pure behavior: API envelope
  normalization, malformed response handling, selection-state transitions, and
  persisted preference behavior.
- Add tests only when they materially reduce regression risk for upcoming
  refactors or lock down behavior that is easy to break. Do not add tests merely
  for coverage, broad smoke tests, or snapshots that mostly restate structure.
- Prefer extracting small pure helpers over adding large test harnesses around
  implementation details.
- Avoid low-value snapshot tests or tests that merely restate JSX structure.
- Do not move the Go entry point, split `service/` packages, introduce TypeScript
  migration, or redesign the frontend in this child task.

## Candidate Test Areas

### Backend

- Fresh same-day cache returns `stale=false` without starting unnecessary JW
  queries. This protects a future cache-freshness state-machine refactor.
- JW fixture rows normalize into the expected `TodayClassrooms` campus,
  building, room, `free_nodes`, `free_times`, and node-count shape. This
  protects a future domain-normalization extraction.
- `GetData` preserves the public success response envelope, including
  successful `code` and `data`. This protects the upcoming injected HTTP
  boundary refactor without adding a test seam only for error injection.
- Deterministic `GetData` error-envelope coverage is deferred to the upcoming
  injected HTTP boundary child task, where handler dependencies will become
  injectable without forcing this behavior-safety task to preempt that design.
- Do not add more stale refresh, concurrent refresh, warmup, backoff, token, or
  generic handler tests in this child unless implementation uncovers a specific
  unprotected regression risk.

### Frontend

- Successful backend envelope normalizes to `{ code: 0, msg, data }` with a
  `campuses` array.
- Non-zero backend envelope keeps a safe message and null data.
- Malformed backend payloads throw clear errors.
- HTTP error payloads produce the best available message.
- Selection reducer clears dependent building/time selections when campus
  changes.
- Building, class-time, `showClassTime`, and `canSelectAllDay` actions preserve
  current state semantics, including localStorage persistence where applicable.

## Acceptance Criteria

- [x] The task includes a strictly limited frontend Vitest setup, with tests
      only for behavior that protects upcoming frontend refactors.
- [x] Backend behavior tests added or strengthened for the approved high-risk
      areas without requiring real JW credentials; each new test protects a
      specific behavior or regression risk.
- [x] Frontend behavior tests added for the approved frontend scope, if included.
- [x] Any testability extraction keeps runtime behavior and public contracts
      unchanged.
- [x] New or changed test commands are documented in the child implementation
      plan and package scripts where appropriate.
- [x] `go test ./...` passes or any environment-related skip is documented.
- [x] Frontend validation commands pass for the approved frontend scope, such as
      `pnpm lint`, `pnpm build`, and a new `pnpm test` script if added.
- [x] Parent task progress is updated after this child task is completed.

## Approved Decisions

- Include a minimal frontend Vitest setup in this child task, but only together
  with high-value behavior tests. Do not add smoke tests, snapshots, or broad
  component-render tests just to prove the runner works.
- Keep backend additions narrow: fresh-cache short-circuit, JW fixture to public
  payload characterization, and minimal `GetData` success-envelope coverage.
  Existing stale refresh, concurrency, warmup, backoff, token override, URL
  validation, readiness, gzip, SPA fallback, and API 404 tests are already enough
  for this child task.
- Defer deterministic `GetData` error-envelope coverage to the injected HTTP
  boundary child task instead of adding a temporary test seam in this child.

## Implementation Results

- Added `TestGetTodayClassroomsReturnsFreshCacheWithoutJWQuery` to protect the
  fresh same-day cache short-circuit without touching the JW upstream.
- Added `TestQueryAllBuildsTodayClassroomsFromJWFixture` to characterize the
  public `TodayClassrooms` payload produced from representative JW rows.
- Added `TestGetDataReturnsSuccessEnvelopeFromFreshCache` to protect the
  `/api/get_data` success envelope before the HTTP dependency-injection task.
- Extracted frontend response parsing into `frontend/src/todayClassroomsResponse.js`
  while preserving `useTodayClassrooms` hook behavior.
- Added a strict Vitest setup with seven focused frontend behavior tests across
  response normalization and selection-state persistence/reset semantics.

## Validation Results

- `gofmt -l .` passed with no output.
- `go test ./...` passed.
- `go vet ./...` passed.
- `pnpm --dir "frontend" lint` passed.
- `pnpm --dir "frontend" test` passed: 2 test files, 7 tests.
- `pnpm --dir "frontend" build` passed. Vite emitted an existing third-party
  Rollup annotation warning from `rc-field-form`; build completed successfully.
