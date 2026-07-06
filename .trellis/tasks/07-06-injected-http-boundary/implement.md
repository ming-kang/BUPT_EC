# Injected HTTP Boundary Implementation Plan

## Pre-Implementation Gate

- Confirm the recommended design decisions in `design.md` before editing source
  files.
- Do not start a `cmd/` or `internal/httpapi` migration in this child unless the
  user explicitly changes the approved scope.
- Do not change service cache/JW/token behavior or frontend files.
- Do not add broad smoke tests; only update tests that protect the HTTP
  injection seam and public HTTP contracts.

## Implementation Checklist

### 1. Introduce the injectable HTTP boundary

1. In `handler.go`, add an unexported `classroomDataService` interface covering
   only the handler methods needed from `*service.ClassroomService`:
   - `GetTodayClassrooms(context.Context)`;
   - `GetRuntimeStatus()`;
   - `HasUsableTodayCache()`.
2. Add `HTTPServer` with fields for the classroom service and credential-check
   function.
3. Add `NewHTTPServer(classroomService classroomDataService, hasJWCredentials func() bool) *HTTPServer`.
4. Convert `GetData`, `Healthz`, and `Readyz` into methods on `*HTTPServer`.
5. Keep error response construction safe through `service.SafeErrorMessage` and
   `logs.GetLogIDFromContext`.

### 2. Move route registration onto the injected boundary

6. In `router.go`, replace package-level `SetRouter(r *gin.Engine)` with
   `func (server *HTTPServer) RegisterRoutes(r *gin.Engine)`.
7. Register `server.Healthz`, `server.Readyz`, and `server.GetData`.
8. Preserve gzip middleware, `/api` log-id middleware, embedded static serving,
   SPA fallback, and JSON API 404 behavior exactly.
9. Keep `gzipMiddleware`, `gzipResponseWriter`, `EmbedFolder`, and embedded FS
   helpers as package-level helpers because they do not depend on service state.

### 3. Remove package-level service state from startup

10. In `main.go`, remove `var classroomService *service.ClassroomService`.
11. Change `Init()` to return `*service.ClassroomService` after performing the
    same logging/config/cache initialization and runtime config validation.
12. In `main()`, store the returned service in a local variable, construct
    `httpServer := NewHTTPServer(classroomService, config.HasJWCredentials)`,
    and call `httpServer.RegisterRoutes(r)`.
13. Use the local service variable for warmup and graceful shutdown.

### 4. Update focused handler/router tests

14. Add `fakeClassroomService` in `handler_test.go` implementing
    `classroomDataService`.
15. Update readiness tests to use `NewHTTPServer(fakeService, fakeCredentials)`
    instead of environment variables and package-level service mutation.
16. Update `GetData` success-envelope test to use the injected server.
17. Add deterministic `GetData` error-envelope test that goes through
    `RegisterRoutes` so `LogID` middleware runs. Assert:
    - HTTP 503;
    - JSON `code` is 503;
    - `msg` equals the safe generic message for the chosen fake error;
    - `data` is null;
    - `log_id` is non-empty;
    - `LogID` response header is non-empty and matches the body value.
18. Update SPA fallback and gzip tests to call the new registration method or
    handler method shape.

### 5. Update docs and specs

19. Update `docs/development.md` so the architecture tour says `main.go`
    constructs the service and injects it into `HTTPServer` / route methods.
20. Update `AGENTS.md` to remove the old `handler.go::GetData` → package-level
    `classroomService` wording and the old handler-test mutation pattern.
21. Update `.trellis/spec/backend/directory-structure.md` for new HTTP-layer
    ownership and handler test pattern.
22. Update `.trellis/spec/backend/quality-guidelines.md` for the new local
    handler-test pattern.
23. Add a `CHANGELOG.md` entry only if the implementation makes a
    contributor-visible documentation/process change that should appear in user
    release notes. Do not add one for purely internal test refactoring unless
    required by final scope.

### 6. Task artifact updates

24. Update this task's PRD/design/implementation plan if implementation evidence
    changes the approved scope.
25. After validation, record validation results in this task and update the
    parent roadmap progress.

## Validation Plan

Run after implementation:

```bash
gofmt -l .
go test ./...
go vet ./...
```

If docs/spec-only files are the only non-Go changes beyond tests, frontend
checks are not required. If frontend files are unexpectedly touched, also run:

```bash
pnpm --dir "frontend" lint
pnpm --dir "frontend" test
pnpm --dir "frontend" build
```

## Validation Results

- `gofmt -l .` passed with no output.
- `go test ./...` passed.
- `go vet ./...` passed.
- Frontend checks were not run because this implementation did not touch
  frontend source or package files.

## Risk and Rollback Points

- Main risk: accidentally bypassing `/api` log-id middleware when converting
  route registration. Use the error-envelope test to prove the middleware still
  runs.
- Main risk: readiness tests no longer exercising credential/cache combination.
  Keep explicit fake credential and fake cache states.
- Main risk: docs/spec drift. Search for old `classroomService` and `SetRouter`
  wording after edits.
- Rollback: restore package-level `classroomService`, package-level handlers,
  and `SetRouter`, then revert docs/tests.

## Expected Changed Files

- `main.go`
- `router.go`
- `handler.go`
- `handler_test.go`
- `docs/development.md`
- `AGENTS.md`
- `.trellis/spec/backend/directory-structure.md`
- `.trellis/spec/backend/quality-guidelines.md`
- `.trellis/tasks/07-06-injected-http-boundary/*`
- `.trellis/tasks/07-06-refactor-maintainability/prd.md`
