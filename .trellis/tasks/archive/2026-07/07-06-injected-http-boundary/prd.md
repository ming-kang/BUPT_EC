# HTTP 边界注入

## Goal

Replace the HTTP layer's package-level `classroomService` dependency with an
explicitly injected HTTP server/handler boundary, while preserving every public
route, response envelope, cache behavior, logging correlation, gzip behavior,
and embedded SPA fallback.

This task prepares the codebase for later backend responsibility splits by
making handler dependencies testable without mutating package-level state. It is
not a service-package migration, router rewrite, or product behavior change.

## Background and Confirmed Facts

- This child task is linked to parent task `07-06-refactor-maintainability`.
- The previous child task `07-06-behavior-safety-net` completed and added
  focused backend/frontend behavior tests before structural refactors.
- Before this task, `main.go` declared package-level mutable state:
  `var classroomService *service.ClassroomService`.
- Before this task, `Init()` initialized logs, config, cache, and assigned the package-level
  `classroomService` value.
- Before this task, `main()` built a Gin router through `SetRouter(r)`, then used the same global
  `classroomService` for startup warmup and graceful-shutdown draining.
- Before this task, `router.go::SetRouter` registered `Healthz`, `Readyz`, and
  `GetData` package-level handler functions, applies API request `log_id`
  middleware under `/api`, serves embedded `frontend/dist`, and owns the SPA
  fallback plus JSON 404 for unknown `/api/*` paths.
- Before this task, `handler.go::GetData` read request context through
  `logs.GetContextFromGinContext`, logs with `slog.InfoContext`, calls
  `classroomService.GetTodayClassrooms`, and shapes success/error envelopes.
- Before this task, `handler.go::Readyz` called `classroomService.GetRuntimeStatus`,
  `classroomService.HasUsableTodayCache`, and `config.HasJWCredentials`.
- Before this task, `handler_test.go` tests mutated the package-level `classroomService`.
  This works but is the hidden global dependency this task should remove.
- Deterministic `GetData` error-envelope coverage was intentionally deferred
  from the behavior-safety task to this task because handler dependencies become
  injectable here.
- Docs/specs currently describe handler tests assigning package-level
  `classroomService`; they must be updated if this task changes the handler test
  pattern.

## Requirements

- Remove handler and router dependence on the package-level `classroomService`.
- Prefer a small `HTTPServer` or handler struct in the existing `main` package;
  do not move the Go entry point to `cmd/` and do not introduce a new package in
  this child task unless explicitly approved.
- Keep handlers thin: request context/logging, service call, and response shape
  only. Business logic remains in `service/`.
- Preserve all public HTTP behavior:
  - `GET /api/get_data` success envelope: HTTP 200 with `code: 0` and `data`;
  - `GET /api/get_data` failure envelope: HTTP 503 with `code`, safe `msg`,
    `log_id`, and `data: null`;
  - `GET /healthz` remains `200 {"status":"ok"}`;
  - `GET /readyz` keeps credential/cache/runtime readiness semantics;
  - unknown `/api/*` paths still return JSON 404;
  - non-API paths still serve the embedded SPA fallback;
  - gzip still applies to normal responses and still skips `/healthz` and
    `/readyz`.
- Preserve `/api/*` `log_id` middleware behavior, including the `LogID` response
  header and error-body `log_id` correlation.
- Add deterministic handler tests for injected success and error paths without
  touching the real JW network or real credentials.
- Do not modify service cache semantics, refresh coordination, token management,
  JW client behavior, frontend API assumptions, or embedded asset layout.
- Keep tests focused on the dependency-injection seam and public HTTP contract;
  do not add broad smoke tests.
- Update source-backed docs/specs that describe HTTP wiring or handler test
  patterns.

## Out of Scope

- Moving `main.go`, `router.go`, or `handler.go` into `cmd/` or `internal/`.
- Splitting `service/` into `internal/classrooms` or `internal/jw`.
- Changing the `/api/get_data`, `/healthz`, `/readyz`, gzip, or SPA fallback
  public behavior.
- Adding a local database, persistence layer, scheduler, or new runtime config
  format.
- Refactoring frontend components or API client behavior.
- Introducing a large dependency-injection framework.
- Adding test-only seams that are not part of the final HTTP boundary design.

## Acceptance Criteria

- [x] `main.go` no longer needs a package-level `classroomService` variable for
      handlers.
- [x] Route registration receives an injected HTTP server/handler dependency
      instead of closing over global service state.
- [x] `GetData` and `Readyz` can be tested with deterministic fake dependencies
      without real JW credentials or network access.
- [x] `GetData` success and error response envelopes are covered by focused
      handler tests, including safe error message and non-empty `log_id` on the
      error path.
- [x] Existing readiness, gzip, SPA fallback, and unknown `/api/*` behavior stays
      covered and passing.
- [x] Docs/specs that mention package-level handler testing or HTTP wiring are
      updated to the new pattern.
- [x] Validation commands pass or skipped checks are documented:
      `gofmt -l .`, `go test ./...`, `go vet ./...`, and focused frontend
      checks only if a change unexpectedly touches frontend code.

## Implementation Results

- Added `HTTPServer` and the narrow `classroomDataService` interface in the
  `main` package.
- Converted route registration to `HTTPServer.RegisterRoutes(router)` and
  registered handler methods from the injected boundary.
- Changed `Init()` to return the constructed `*service.ClassroomService`; `main()`
  keeps the service as a local dependency for HTTP wiring, warmup, and graceful
  shutdown.
- Replaced handler tests that mutated package-level service state with
  deterministic fake service injection through `NewHTTPServer`.
- Added focused `GetData` error-envelope coverage through the full router so the
  `/api` `log_id` middleware is exercised and the body/header correlation is
  asserted.
- Updated development docs and backend Trellis specs to describe the injected
  HTTP boundary and handler test pattern.

## Validation Results

- `gofmt -l .` passed with no output.
- `go test ./...` passed.
- `go vet ./...` passed.
- Frontend checks were not run because this child task did not modify frontend
  source or package files.

## Approved Decisions

- Keep the HTTP boundary in the existing `main` package for this child task; do
  not create `internal/httpapi` yet.
- Use `HTTPServer.RegisterRoutes(router)` as the route registration API.
- Remove package-level handler functions, package-level route registration, and
  package-level `classroomService` instead of keeping compatibility wrappers.
- Inject `config.HasJWCredentials` as `hasJWCredentials func() bool` for
  readiness behavior and tests.
- Update source-backed docs/specs in the same child task.
- Do not add `CHANGELOG.md` unless implementation unexpectedly creates a
  user-visible behavior or contributor-process change that warrants release
  notes.
