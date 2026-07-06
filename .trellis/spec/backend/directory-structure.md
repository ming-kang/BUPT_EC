# Directory Structure

## Overview

This repository is a Go module named `BUPT_EC` with a React/Vite frontend
embedded into the Go binary. Keep backend entry points thin, put business logic
under `service/`, put JSON contract types under `service/model/`, and keep
deployment/release tooling in `scripts/` and `docs/`.

## Repository Layout

```text
.
├── main.go                    # process startup, service init, HTTPServer wiring, graceful shutdown
├── router.go                  # HTTPServer route registration, gzip middleware, embedded SPA
├── handler.go                 # HTTPServer boundary delegating to injected ClassroomService
├── config/                    # environment loading and campus config
├── logs/                      # slog setup and per-request log_id context
├── cache/                     # process-local go-cache adapter
├── service/                   # JW client, token, refresh, cache, builder logic
│   └── model/                 # JW and public API JSON structs
├── utils/                     # shared small helpers
├── frontend/                  # React/Vite application and built dist assets
├── scripts/                   # install/release automation
└── docs/                      # user-facing operation, deploy, release docs
```

Internal imports use the module prefix, for example `"BUPT_EC/service"`,
`"BUPT_EC/logs"`, and `"BUPT_EC/config"`.

## Entry Points and HTTP Layer

- `main.go` owns process lifetime and startup initialization. Its `Init()`
  function configures logs/config/cache and returns the single application
  `service.ClassroomService` built from `config.GetConfig()` and
  `cache.GlobalCache`. `main()` injects that service plus
  `config.HasJWCredentials` into `NewHTTPServer`, starts background warmup, and
  drains warmup work during graceful shutdown with `ClassroomService.WaitWarmup`.
- `router.go` owns `HTTPServer.RegisterRoutes`, gzip handling, static frontend
  serving, and SPA fallback. API routes live under `/api` and receive
  `logs.SetNewContextForGinContext`.
- `handler.go` should stay thin. `HTTPServer` methods read the request context,
  log the operation, call the injected `classroomDataService`, and shape HTTP
  responses.

When adding an endpoint, register it in `HTTPServer.RegisterRoutes`, implement
the smallest possible method in `handler.go`, and put data access or
transformation in `service/`. Do not call the JW HTTP API directly from
handlers.

## Service Package Ownership

`service/` is split by runtime responsibility:

- `classroom_service.go` defines `ClassroomService`, `CacheStore`, and service
  construction. All mutable classroom-query runtime state belongs on this
  struct.
- `realtime_data.go` exposes the public classroom query methods and owns the
  same-day cache read/write flow.
- `refresh_coordinator.go` owns single-flight refresh state, backoff,
  stale-while-revalidate behavior, and warmup draining.
- `token_manager.go` owns token/API URL caching and `singleflight` login/API URL
  deduplication.
- `jw_client.go`, `crypto.go`, and `urlutil.go` own the JW HTTP protocol,
  password encryption, response parsing, and API URL validation.
- `classroom_builder.go` converts JW rows into campuses, buildings, rooms,
  `free_nodes`, and `free_times`.
- `runtime_status.go` exposes readiness diagnostics without leaking secrets.
- `jw_error.go` classifies JW failures and maps them to safe user-facing
  messages.

Add new service behavior next to the responsibility it extends. If new behavior
needs external I/O, keep the dependency injectable like `JWClient` so tests can
use mocks instead of network calls.

## JSON Model Boundary

`service/model/realtime_data.go` is the source of truth for serialized JW and
API shapes:

- JW upstream structs: `ServerConfigResponse`, `LoginResponse`, `JWClassInfo`,
  and `QueryResponse`.
- Public API structs: `TodayClassrooms`, `CampusInfo`, `BuildingInfo`,
  `RoomInfo`, `NodeInfo`, `FreeTime`, and `APIError`.

When changing a public JSON tag or field, update the backend builder/handler,
the frontend consumer in `frontend/src/useTodayClassrooms.js`, the envelope
normalization helpers in `frontend/src/todayClassroomsResponse.js`, any affected
components, tests, docs, and `CHANGELOG.md` if the behavior is user-visible.

## Naming Conventions

- Keep Go package names short and lowercase (`service`, `config`, `logs`,
  `cache`, `model`).
- Use exported names only for cross-package APIs such as
  `service.NewClassroomService`, `service.SafeErrorMessage`, and
  `config.GetConfig`.
- Prefer focused files named after their runtime role (`token_manager.go`,
  `refresh_coordinator.go`, `classroom_builder.go`) rather than broad utility
  files.
- Tests live beside the code they verify as `*_test.go`. Service tests use the
  `service` package; handler tests use `main` and inject fake dependencies via
  `NewHTTPServer`.

## Anti-Patterns

- Do not add package-level mutable globals inside `service/`; extend
  `ClassroomService` instead.
- Do not introduce a timetable database or persistence layer for classroom data
  unless the task explicitly asks for an architecture change.
- Do not put JW protocol parsing in handlers or frontend code. Keep it behind
  `JWClient` and `service/model`.
- Do not add route-specific static file behavior outside `router.go`; the
  embedded frontend and SPA fallback are centralized there.
- Do not commit generated runtime logs (`run_log/`) or real `.env` credentials.
