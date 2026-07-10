# Behavior Safety Net Design

## Design Goal

Add a small, behavior-focused safety net before structural refactors begin. The
tests and any helper extractions should protect the next planned refactors while
preserving current runtime behavior, public API shapes, and deployment flow.

This is not a coverage-improvement task. Each new test must have a clear reason:
it protects behavior that is likely to be touched by the upcoming refactor
roadmap.

## Testing Discipline

- Prefer tests that lock down current behavior needed for upcoming refactors.
- Do not add broad smoke tests, snapshot tests, or component-render tests that
  only prove the runner works.
- Prefer small pure helper extractions over heavyweight test harnesses.
- Avoid introducing architectural seams that belong to later child tasks unless
  the seam is necessary to test a specifically approved risk.

## Backend Safety Design

### 1. Fresh cache short-circuit

Add one test in `service/realtime_data_test.go` that proves a same-day fresh
cache entry returns immediately as `stale=false` and does not call the JW client.

Protected future refactor: cache freshness state-machine cleanup.

Implementation notes:

- Use `newTestService(t, client)` with a `mockJWClient` whose `QueryCampus`
  increments a counter or fails if called.
- Seed `TodayCacheKey` with a `model.TodayClassrooms` value whose `Date` is
  today, `ExpiresAt` is in the future, and `StaleUntil` is `endOfDay(now)`.
- Call `GetTodayClassrooms(context.Background())`.
- Assert `Stale == false`, `Error == nil`, the seeded campus payload is returned,
  and the JW client counter remains zero.

### 2. JW fixture to public payload characterization

Add one fixture-driven test in `service/realtime_data_test.go` that runs
representative JW rows through the current service flow and verifies the public
`TodayClassrooms` shape.

Protected future refactor: extracting classroom normalization/domain helpers.

Implementation notes:

- Use `mockJWClient` so the test does not require network or real credentials.
- Provide rows that cover normal rooms, merged room names, duplicate rooms, and
  multiple class nodes.
- Prefer exercising `QueryAll` or `GetTodayClassrooms` rather than directly
  testing many private helpers, so the characterization follows the public
  service behavior.
- Assert the high-value shape only: campus IDs/names, sorted node list, building
  grouping, room display name, capacity, `free_nodes`, and `free_times`.
- Avoid asserting incidental details that would make safe refactors noisy.

### 3. GetData response envelope

Protect the HTTP boundary before the injected-server child task, but do not
preempt that task by adding a large handler abstraction here.

Confirmed constraint:

- `handler.go::GetData` calls the package-level concrete `classroomService`.
- `service.newClassroomService` can inject a `JWClient`, but it is unexported and
  therefore unavailable to `handler_test.go` in the `main` package.
- A deterministic `GetData` error-envelope test is therefore awkward before the
  injected HTTP boundary exists. Using the real default JW client or environment
  credentials would violate the no-network unit-test requirement.

Decision:

- Keep only deterministic success-envelope coverage in this child.
- Defer deterministic error-envelope coverage to the injected HTTP boundary
  child task, where handler dependencies can become injectable without a
  temporary test seam.

## Frontend Safety Design

### Test runner setup

Add a minimal Vitest setup only if paired with the approved behavior tests.

Expected changes:

- Add a `test` script to `frontend/package.json`.
- Add only the dependencies required for Vitest in this Vite/React project.
- Use a browser-like test environment only if needed for `localStorage` behavior.

### API envelope normalization

Extract `normalizeResponse` and related message handling from
`frontend/src/useTodayClassrooms.js` into a small pure module, likely under
`frontend/src/` or a narrowly named model/API helper file.

Protected future refactor: moving classroom API/model logic into a feature
boundary.

Tests should cover:

- successful backend envelope with `campuses` array;
- non-zero backend envelope returning a safe message and `data: null`;
- malformed envelope and malformed data errors.

### Selection reducer behavior

Test current reducer behavior in `frontend/src/selectionContext.js` because it
will likely be moved or split during the frontend feature-boundary task.

Tests should cover:

- changing campus clears dependent building and class-time selections;
- building and class-time actions preserve their current semantics;
- preference actions persist `showClassTime` and `canSelectAllDay` to
  `localStorage`.

Avoid broad component tests in this child task.

## Compatibility Notes

- Public endpoint behavior stays unchanged.
- Runtime configuration stays unchanged.
- No Go package moves are included.
- No full TypeScript migration is included.
- No UI redesign is included.

## Rollback Notes

- Backend tests can be reverted independently from frontend test setup.
- Frontend pure helper extraction should preserve existing exports from
  `useTodayClassrooms.js`; if it causes lint/build issues, revert the extraction
  and keep hook behavior unchanged.
- Dependency additions for Vitest should remain limited and removable if the
  frontend test scope is later split out.
