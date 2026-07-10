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

- `main.go` owns process lifetime and is the only production composition root.
  Its `Init()` loads one `config.RuntimeConfig`, applies Gin/log settings,
  constructs `cache.New()`, `utils.NewHTTPClient()`, `service.NewJWClient`, and
  `service.NewClassroomService`, then injects the service plus the immutable
  credential predicate into `NewHTTPServer`. `main()` starts background warmup
  with an application context, cancels it before HTTP shutdown, and drains work
  with `ClassroomService.WaitBackground` after handlers exit.
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

## Scenario: Runtime Configuration and Composition Root

### 1. Scope / Trigger

Apply this contract whenever an environment key, dotenv behavior, startup
validation, cache/HTTP/JW constructor, Gin/log initialization, or production
dependency wiring changes. The purpose is to keep environment access at one
boundary and make every production dependency traceable from `main.go`.

### 2. Signatures

```go
type LookupEnv func(string) (string, bool)

func config.Load(dotenvPath string, lookup config.LookupEnv) (config.RuntimeConfig, error)
func (c config.RuntimeConfig) HasJWCredentials() bool

func cache.New() *gocache.Cache
func utils.NewHTTPClient() *http.Client
func service.NewJWClient(username, password string, client utils.HTTPDoer) (service.JWClient, error)
func service.NewClassroomService(
    options service.ClassroomServiceOptions,
    store service.CacheStore,
    client service.JWClient,
) (*service.ClassroomService, error)
```

### 3. Contracts

- `config.Load(".env", os.LookupEnv)` is the only production environment read.
- Resolution order is process environment, dotenv, then documented default.
  A process value that is explicitly present but empty still overrides dotenv.
- Missing dotenv is allowed. A present malformed/unreadable file returns a
  generic safe error without file contents or credential values.
- The snapshot owns `JW_USERNAME`, `JW_PASSWORD`, `JW_TOKEN`, `APP_ADDR`,
  `GIN_MODE`, `LOG_CALLER`, and fixed campuses `01/西土城`, `04/沙河`.
- `main.go` applies `gin.SetMode` and `logs.Init` after loading the snapshot,
  then constructs cache → HTTP client → JW client → classroom service → HTTP
  boundary in visible order.
- `JWClient`, `TokenManager`, `ClassroomService`, cache, HTTP helpers, and logs
  do not read runtime environment values after construction.
- Slice inputs such as campuses are copied by the receiving constructor.
  Missing required dependencies return constructor errors before any request;
  errors identify only the dependency category and never format secrets.
- Configuration is not hot-reloaded. Operators restart the process to apply a
  new snapshot.

### 4. Validation & Error Matrix

| Condition | Required result |
| --- | --- |
| dotenv absent | continue with process environment/defaults |
| dotenv valid, process key absent | use dotenv value |
| same key in process and dotenv | process value wins |
| dotenv malformed/unreadable | startup error with no dotenv contents |
| token present | credentials valid, even without login pair |
| token absent, username + password present | credentials valid |
| incomplete/no credentials | startup error with no credential values |
| empty `APP_ADDR` | `127.0.0.1:8080` |
| malformed address or port outside 1–65535 | startup error |
| `GIN_MODE` not `debug`, `release`, or `test` | startup error |
| nil/typed-nil HTTP doer, cache store, or JW client | constructor error |

### 5. Good/Base/Bad Cases

- Good: systemd supplies credentials and `GIN_MODE=release`; no repository
  `.env` exists, and `main.go` constructs the full graph from that snapshot.
- Base: local `.env` supplies username/password and defaults the listen address
  to loopback; tests pass a map lookup without mutating process environment.
- Bad: `service.Login`, `TokenManager.EnsureToken`, or `logs.Init` calls
  `os.Getenv`, so changing global environment during a test silently changes a
  previously constructed application's behavior.

### 6. Tests Required

- Config table tests cover missing/valid/malformed/unreadable dotenv, process
  precedence, credential combinations, address/Gin/log parsing, and secret-safe
  errors.
- Cache tests prove `cache.New()` returns isolated instances with the expected
  default expiration.
- HTTP/JW tests prove the supplied doer and injected credentials are used,
  redirect/body limits remain intact, and nil/typed-nil dependencies fail at
  construction.
- Service tests inject token overrides directly and assert rejected overrides
  remain invalidated until process reconstruction.
- Run `rg "os\\.(Getenv|LookupEnv)" service logs main.go config` and verify the
  only production lookup is `main.go` passing `os.LookupEnv` to `config.Load`.

### 7. Wrong vs Correct

#### Wrong

```go
config.InitConfig()
cache.InitCache()
client := &defaultJWClient{} // reads credentials and a package HTTP client later
service := service.NewClassroomService(config.GetConfig(), cache.GlobalCache)
```

#### Correct

```go
cfg, err := config.Load(".env", os.LookupEnv)
store := cache.New()
httpClient := utils.NewHTTPClient()
jwClient, err := service.NewJWClient(cfg.JW.Username, cfg.JW.Password, httpClient)
classroomService, err := service.NewClassroomService(service.ClassroomServiceOptions{
    Campuses: cfg.Campuses, TokenOverride: cfg.JW.Token,
}, store, jwClient)
```

## Service Package Ownership

`service/` is split by runtime responsibility:

- `classroom_service.go` defines `ClassroomService`, `CacheStore`, constructor
  options, and service construction. All mutable classroom-query runtime state
  belongs on this struct.
- `realtime_data.go` exposes the public classroom query methods and owns the
  same-day cache read/write flow.
- `refresh_coordinator.go` owns single-flight refresh state, backoff, and
  stale-while-revalidate behavior.
- `warmup.go` owns the startup/midnight scheduler, retry-delay state machine,
  scheduler cancellation, and background-worker draining.
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
  `service.NewClassroomService`, `service.SafeErrorMessage`, and `config.Load`.
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
- Do not add package-level config/cache/HTTP singletons or downstream
  `os.Getenv` calls; extend the startup snapshot and constructor graph instead.
