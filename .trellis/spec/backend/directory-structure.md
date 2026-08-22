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
├── router.go                  # HTTPServer.Routes() handler chain, gzhttp config, static/SPA serving
├── handler.go                 # HTTPServer boundary delegating to injected ClassroomService
├── Taskfile.yml               # local dev entry points (task build/test/check/vuln)
├── config/                    # environment loading and campus config
├── logs/                      # slog setup and per-request log_id context
├── service/                   # JW client, outbound HTTP, token, refresh, day-cache, builder logic
│   └── model/                 # JW and public API JSON structs
├── web/                       # frontend asset embedding (build-tag dual implementation)
│   └── dist/                  # build staging area, git-ignored; copied from frontend/dist
├── frontend/                  # React/Vite application (vite builds into frontend/dist)
├── scripts/                   # install/release automation
└── docs/                      # user-facing operation, deploy, release docs
```

Internal imports use the module prefix, for example `"BUPT_EC/service"`,
`"BUPT_EC/logs"`, and `"BUPT_EC/config"`.

## Entry Points and HTTP Layer

- `main.go` owns process lifetime and is the only production composition root.
  Its `Init()` loads one `config.RuntimeConfig`, applies log settings,
  constructs `service.NewJWHTTPClient()`, `service.NewJWClient`, and
  `service.NewClassroomService`, then injects the service plus the immutable
  credential predicate result into `NewHTTPServer`. `main()` runs the service
  lifecycle with `Run(appCtx)` on a goroutine, cancels it before HTTP shutdown,
  and drains work with `ClassroomService.Shutdown` after handlers exit. It sets
  `Handler: app.httpServer.Routes()` (main.go:106) and warns when `web.Dist()`
  reports placeholder assets (main.go:98-100).
- `router.go` owns `func (server *HTTPServer) Routes() http.Handler`: the
  `http.ServeMux`, middleware chain (`gzipSkipProbes` → gzhttp →
  `apiLogContext` → `recovery`), gzhttp configuration, static frontend
  serving, and SPA fallback. `/api` requests receive a `log_id` context from
  the `apiLogContext` middleware. See api-contract.md "Routes" for the
  external contract.
- `handler.go` should stay thin. `HTTPServer` methods take
  `(w http.ResponseWriter, r *http.Request)`, read `r.Context()`, log the
  operation, call the injected `classroomDataService`, and shape HTTP
  responses via `writeJSON`.

When adding an endpoint, register it on the mux inside `HTTPServer.Routes()`,
implement the smallest possible method in `handler.go`, and put data access or
transformation in `service/`. Do not call the JW HTTP API directly from
handlers.

## Frontend Embedding: the `web/` Package

`web/` isolates `go:embed` from the rest of the backend behind a build tag:

- `web/web.go`: public `Dist() (fs.FS, bool)` — the asset tree rooted at
  `dist/` and whether real embedded assets are present — plus the
  placeholder HTML constant.
- `web/embed_enabled.go`: `//go:build embed_assets` + `//go:embed all:dist`.
- `web/embed_disabled.go`: `//go:build !embed_assets`; returns a
  `fstest.MapFS` with a single placeholder `index.html`, so bare
  `go vet` / `go test` / `go build` work on a clean checkout with no
  frontend build.

`web/dist/` is a build **staging area**, not a source directory: `task build`
(and `release.yml`) copy `frontend/dist` into it before building with
`-tags embed_assets`. It is git-ignored (`.gitignore:17` `/web/dist/`). Both
FS variants guarantee `index.html` exists at the root — `Routes()` panics at
assembly time otherwise. The full-binary build chain lives in `Taskfile.yml`
(`task build`: frontend build → copy to `web/dist` → `go build -trimpath
-tags embed_assets -ldflags "-s -w -X main.version=..."`).

## Scenario: Runtime Configuration and Composition Root

### 1. Scope / Trigger

Apply this contract whenever an environment key, dotenv behavior, startup
validation, HTTP/JW constructor, log initialization, or production
dependency wiring changes. The purpose is to keep environment access at one
boundary and make every production dependency traceable from `main.go`.

### 2. Signatures

```go
type LookupEnv func(string) (string, bool)

func config.Load(dotenvPath string, lookup config.LookupEnv) (config.RuntimeConfig, error)
func (c config.RuntimeConfig) HasJWCredentials() bool

func service.NewJWHTTPClient() *http.Client
func service.NewJWClient(username, password string, client service.HTTPDoer) (service.JWClient, error)
func service.NewClassroomService(
    options service.ClassroomServiceOptions,
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
  `LOG_CALLER`, and fixed campuses `01/西土城`, `04/沙河`. (`GIN_MODE` was
  removed with the Gin dependency; a leftover `GIN_MODE=` line in an old
  deployment env file is silently ignored by `config.Load`.)
- `main.go` applies `logs.Init` after loading the snapshot,
  then constructs HTTP client → JW client → classroom service → HTTP boundary
  in visible order.
- `JWClient`, `TokenManager`, `ClassroomService`, HTTP helpers, and logs do not
  read runtime environment values after construction.
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
| nil HTTP doer or JW client | constructor error |

### 5. Good/Base/Bad Cases

- Good: systemd supplies credentials and `APP_ADDR`; no repository
  `.env` exists, and `main.go` constructs the full graph from that snapshot.
- Base: local `.env` supplies username/password and defaults the listen address
  to loopback; tests pass a map lookup without mutating process environment.
- Bad: `TokenManager.EnsureToken` or `logs.Init` calls
  `os.Getenv`, so changing global environment during a test silently changes a
  previously constructed application's behavior.

### 6. Tests Required

- Config table tests cover missing/valid/malformed/unreadable dotenv, process
  precedence, credential combinations, address/log parsing, and secret-safe
  errors.
- Cache policy tests (`service/cache_policy_test.go`) prove the service's
  internal day-cache slot rejects cross-day reuse and stamps entries at refresh
  completion.
- HTTP/JW tests prove the supplied doer and injected credentials are used,
  redirect/body limits remain intact, and nil dependencies fail at
  construction.
- Service tests inject token overrides directly and assert rejected overrides
  remain invalidated until process reconstruction.
- Run `rg "os\\.(Getenv|LookupEnv)" service logs main.go config` and verify the
  only production lookup is `main.go` passing `os.LookupEnv` to `config.Load`.

### 7. Wrong vs Correct

#### Wrong

```go
config.InitConfig()
client := &defaultJWClient{} // reads credentials and a package HTTP client later
service := service.NewClassroomService(config.GetConfig(), client)
```

#### Correct

```go
cfg, err := config.Load(".env", os.LookupEnv)
httpClient := service.NewJWHTTPClient()
jwClient, err := service.NewJWClient(cfg.JW.Username, cfg.JW.Password, httpClient)
classroomService, err := service.NewClassroomService(service.ClassroomServiceOptions{
    Campuses: cfg.Campuses, TokenOverride: cfg.JW.Token,
}, jwClient)
```

## Service Package Ownership

`service/` is split by runtime responsibility:

- `classroom_service.go` defines `ClassroomService` (including the
  `todayCache atomic.Pointer` day store), optional `Clock` / `BackoffRandom` /
  `WarmupJitter`, constructor options, and service construction. All mutable classroom-query runtime state belongs on this struct.
- `realtime_data.go` exposes the public classroom query methods and owns the
  same-day cache read/write flow.
- `refresh_coordinator.go` owns single-flight refresh state, backoff, and
  stale-while-revalidate behavior.
- `warmup.go` owns the `Run`/`Shutdown` service lifecycle, the startup/midnight
  scheduler, retry-delay state machine, scheduler cancellation, and
  background-worker draining.
- `token_manager.go` owns token/API URL caching and `singleflight` login/API URL
  deduplication.
- `jw_client.go`, `jw_http.go`, `crypto.go`, and `urlutil.go` own the JW HTTP protocol,
  outbound HTTP transport (redirect rejection, body limits),
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

- JW upstream structs: `ServerConfigResponse`, `LoginResponse`, and
  `JWClassInfo`.
- Public API structs: `TodayClassrooms`, `CampusInfo`, `BuildingInfo`,
  `RoomInfo`, `NodeInfo`, `FreeTime`, and `APIError`.

When changing a public JSON tag or field, update the backend builder/handler,
the frontend consumer in `frontend/src/useTodayClassrooms.ts`, the envelope
normalization helpers in `frontend/src/todayClassroomsResponse.ts`, any affected
components, tests, docs, and `CHANGELOG.md` if the behavior is user-visible.

## Naming Conventions

- Keep Go package names short and lowercase (`service`, `config`, `logs`,
  `model`).
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
