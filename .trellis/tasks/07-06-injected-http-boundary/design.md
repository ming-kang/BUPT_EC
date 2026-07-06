# Injected HTTP Boundary Design

## Design Goal

Make the HTTP boundary explicit and injectable without changing runtime behavior.
The end state should let tests construct handlers with deterministic fake
dependencies instead of mutating a package-level `classroomService` variable.

This task is intentionally narrow: keep the current root `main` package, keep
Gin, keep the embedded frontend and gzip behavior in `router.go`, and keep all
business logic in `service/`.

## Pre-Refactor Data Flow

```text
main.Init()
  -> service.NewClassroomService(config.GetConfig(), cache.GlobalCache)
  -> package-level classroomService

main.main()
  -> gin.New()
  -> SetRouter(router)
  -> package-level handlers in handler.go
  -> classroomService.GetTodayClassrooms / RuntimeStatus / HasUsableTodayCache
```

The hidden dependency is the `main` package global. It makes handler tests mutate
shared state and prevented deterministic `GetData` error-envelope coverage in
the previous behavior-safety task.

## Target Shape

Keep the new HTTP boundary in the existing `main` package for this child task.

```go
type classroomDataService interface {
    GetTodayClassrooms(ctx context.Context) (*model.TodayClassrooms, error)
    GetRuntimeStatus() service.RuntimeStatus
    HasUsableTodayCache() bool
}

type HTTPServer struct {
    classroomService   classroomDataService
    hasJWCredentials   func() bool
}

func NewHTTPServer(classroomService classroomDataService, hasJWCredentials func() bool) *HTTPServer
func (server *HTTPServer) RegisterRoutes(router *gin.Engine)
func (server *HTTPServer) GetData(context *gin.Context)
func (server *HTTPServer) Healthz(context *gin.Context)
func (server *HTTPServer) Readyz(context *gin.Context)
```

Recommended details:

- `classroomDataService` stays unexported because it is an internal HTTP-layer
  seam, not a public service API.
- `*service.ClassroomService` satisfies the interface without changes.
- `hasJWCredentials` is injected so readiness tests can avoid process
  environment coupling. Production passes `config.HasJWCredentials`.
- `HTTPServer.RegisterRoutes` replaces the package-level `SetRouter`. The
  method keeps route registration next to the injected handlers, making hidden
  dependencies obvious.
- Package-level `GetData`, `Healthz`, and `Readyz` functions are removed rather
  than kept as compatibility wrappers. This is safe because the package is
  `main` and not imported by other Go packages.
- `gzipMiddleware`, `EmbedFolder`, and embedded SPA filesystem helpers remain in
  `router.go`; they do not depend on service state and are still tested.

## Startup Flow After Refactor

```text
main.Init()
  -> returns *service.ClassroomService

main.main()
  -> classroomService := Init()
  -> httpServer := NewHTTPServer(classroomService, config.HasJWCredentials)
  -> router := gin.New()
  -> httpServer.RegisterRoutes(router)
  -> classroomService.StartWarmup()
  -> graceful shutdown waits on classroomService.WaitWarmup(ctx)
```

`Init()` should keep existing validation behavior and still call
`logs.Init(true)`, `config.InitConfig()`, `config.ValidateRuntimeConfig()`, and
`cache.InitCache()`.

## HTTP Contract Preservation

The handler method bodies should remain equivalent to the current functions:

- `GetData`:
  - reads the request context from `logs.GetContextFromGinContext`;
  - logs `GetData` with `slog.InfoContext`;
  - calls `server.classroomService.GetTodayClassrooms(ctx)`;
  - on error returns HTTP 503 with `service.SafeErrorMessage(err)`, `log_id`,
    and `data: nil`;
  - on success returns HTTP 200 with `code: 0` and `data`.
- `Readyz`:
  - uses injected credentials function and injected service cache status;
  - returns existing `RuntimeStatus` body shape;
  - does not expose secrets.
- `Healthz`:
  - remains independent from config and JW state.
- `RegisterRoutes`:
  - keeps gzip middleware behavior;
  - keeps `/api` log-id middleware;
  - keeps static serving and SPA fallback;
  - keeps JSON 404 for unknown `/api/*` routes.

## Test Design

Add a focused fake service in `handler_test.go`:

```go
type fakeClassroomService struct {
    todayClassrooms *model.TodayClassrooms
    todayError      error
    runtimeStatus   service.RuntimeStatus
    usableCache     bool
}
```

Focused test changes:

1. Replace package-level `classroomService` mutation in existing handler tests
   with `NewHTTPServer(fakeService, fakeCredentialsFunc)`.
2. Keep existing readiness behavior coverage, but make it deterministic through
   fake `usableCache` and fake credential function.
3. Keep the existing `GetData` success-envelope test, but route through the
   injected server.
4. Add the deferred deterministic `GetData` error-envelope test:
   - fake service returns an error such as `context.DeadlineExceeded`;
   - request goes through `server.RegisterRoutes` so the `/api` log-id middleware
     is active;
   - assert HTTP 503, `code: 503`, safe `msg`, `data: null`, non-empty `log_id`,
     and `LogID` response header.
5. Update SPA fallback and gzip tests to use the injected router method where
   route registration is part of the behavior under test.

Do not add broad smoke tests. The behavior-safety task already protects cache
and payload normalization; this task protects the HTTP dependency seam and the
error envelope that was intentionally deferred.

## Documentation and Spec Sync

Update docs/specs that currently mention package-level handler wiring:

- `docs/development.md`: main package tour and test description.
- `AGENTS.md`: public endpoint flow and handler testing guidance.
- `.trellis/spec/backend/directory-structure.md`: entry-point and handler-test
  ownership.
- `.trellis/spec/backend/quality-guidelines.md`: local handler-test pattern.

No `CHANGELOG.md` entry is recommended because runtime behavior and user-facing
API behavior do not change. If implementation changes contributor-facing docs
in a way the project considers user-visible, add a small `[Unreleased]` Changed
bullet.

## Compatibility and Rollback

- Rollback is small: restore package-level handlers and `SetRouter`, then revert
  tests/docs.
- No frontend files should change in this task.
- No service behavior should change; if a service test fails, treat it as a
  regression in the HTTP refactor, not as a reason to alter cache/JW logic.

## Recommended Decisions

1. Keep the HTTP boundary in the `main` package for this child task.
2. Use `HTTPServer.RegisterRoutes(router)` rather than `SetRouter(router,
   server)`.
3. Remove package-level `GetData`, `Healthz`, `Readyz`, `SetRouter`, and
   `classroomService` instead of keeping temporary compatibility wrappers.
4. Inject `config.HasJWCredentials` as a function for readiness tests.
5. Keep docs/spec updates in this same child task.
