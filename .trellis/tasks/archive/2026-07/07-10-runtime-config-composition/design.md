# 运行时配置边界与依赖组装 — Design

## Problem Boundary

当前启动图虽然在 handler 和 service 测试中已有依赖注入，但生产路径仍通过
`config.GlobalConfig`、`cache.GlobalCache`、`utils.defaultHTTPClient` 和多个
`os.Getenv` 隐式取值。目标不是引入 DI 框架，而是让一个小型进程只有一个清晰的
composition root：读取配置一次，构造依赖一次，然后把不可变值传入运行对象。

## Proposed Startup Data Flow

```text
.env (optional) + process environment
  -> config.Load(".env", os.LookupEnv)
  -> config.RuntimeConfig
       ├── JW username/password/token
       ├── APP_ADDR / GIN_MODE / LOG_CALLER
       └── fixed campus list (01, 04)
  -> main.go composition root
       ├── gin.SetMode(config.GinMode)
       ├── logs.Init(main=true, addSource=config.LogCaller)
       ├── cache.New()
       ├── utils.NewHTTPClient()
       ├── service.NewJWClient(username, password, httpClient)
       ├── service.NewClassroomService(options, cache, jwClient)
       └── NewHTTPServer(service, config.HasJWCredentials)
  -> routes + warmup + graceful shutdown
```

No downstream production object reads the environment again.

## Configuration Contract

### Proposed Types and Signatures

```go
type JWCredentials struct {
    Username string
    Password string
    Token    string
}

type RuntimeConfig struct {
    JW        JWCredentials
    AppAddr   string
    GinMode   string
    LogCaller bool
    Campuses  []CampusConfig
}

type LookupEnv func(string) (string, bool)

func Load(dotenvPath string, lookup LookupEnv) (RuntimeConfig, error)
func (c RuntimeConfig) HasJWCredentials() bool
```

`Load` reads `.env` into a map rather than mutating `os.Environ`. Lookup order is:

1. value returned by `lookup` (normally `os.LookupEnv`);
2. value parsed from `.env`;
3. documented default, where one exists.

Tests pass a map-backed lookup and temporary dotenv path, so they do not share global
environment state.

### Defaults and Validation

| Field | Default / rule |
| --- | --- |
| `JW_TOKEN` | optional; when present credentials are considered configured |
| `JW_USERNAME` + `JW_PASSWORD` | both required when token is absent |
| `APP_ADDR` | default `127.0.0.1:8080`; explicit `:8080`, host:port and bracketed IPv6 remain valid |
| `GIN_MODE` | default `debug`; accepted values `debug`, `release`, `test` |
| `LOG_CALLER` | true only for `1` or case-insensitive `true` |
| campuses | fixed `01/西土城`, `04/沙河` |

Missing `.env` is allowed. An existing unreadable or malformed dotenv file returns a safe
error naming the file role, never its contents. Invalid credentials, Gin mode, or listen
address also fail before the server starts.

Explicitly calling `gin.SetMode` after dotenv resolution fixes the current gap where Gin's
package `init` runs before `config.InitConfig` loads `.env`.

## Dependency Construction

### Cache

Replace package globals with:

```go
func cache.New() *gocache.Cache
```

Each call returns an independent process-local cache with the existing default/cleanup
durations. `ClassroomService` still consumes the existing `CacheStore` interface and sets
explicit business TTLs.

### HTTP Transport

`utils` owns the secure concrete transport, but not a singleton:

```go
type HTTPDoer interface {
    Do(*http.Request) (*http.Response, error)
}

func NewHTTPClient() *http.Client
func HttpGet(client HTTPDoer, ctx context.Context, rawURL string) (...)
func HttpPostForm(client HTTPDoer, ctx context.Context, rawURL string, data map[string]string) (...)
func HttpPostWithHeader(client HTTPDoer, ctx context.Context, rawURL string, headers map[string]string) (...)
```

The constructed client preserves proxy behavior, connection pooling, timeouts, the 5 MiB
response limit, and the no-redirect credential protection. Passing the doer explicitly lets
transport tests and the production composition graph prove which client is used.

### JW Client and Classroom Service

Keep the existing `JWClient` interface. The real implementation becomes immutable rather
than reading the environment:

```go
func NewJWClient(username, password string, client utils.HTTPDoer) (JWClient, error)

type ClassroomServiceOptions struct {
    Campuses      []config.CampusConfig
    TokenOverride string
}

func NewClassroomService(
    options ClassroomServiceOptions,
    store CacheStore,
    client JWClient,
) (*ClassroomService, error)
```

`TokenManager` stores the startup `TokenOverride` value. Its existing source tracking and
`overrideInvalidated` state remain unchanged: only rejection of the actual override disables
reuse, and a login-issued token failure cannot restore it.

Constructor inputs are copied where they contain slices so callers cannot mutate runtime
campus state after construction. Nil required dependencies return explicit constructor errors
rather than producing delayed request panics. These errors identify only the missing dependency
category and never format credentials. Empty username/password remain legal constructor values
for token-only startup; if an override is later rejected, `Login` returns the existing safe
configuration error.

## Logging and HTTP Boundary

`logs.Init` changes to accept `addSource bool`; it continues to install the process-global
default `slog` logger because that is the intended logging integration point. It no longer
reads `LOG_CALLER` itself.

`HTTPServer` keeps its current injectable credential predicate for deterministic readiness
tests. Production passes the method value from the immutable runtime config.

## Compatibility and Security

- No environment key is added, removed, or renamed.
- System environment continues to override `.env`.
- `JW_TOKEN`-only startup remains valid; if that token is rejected and no username/password
  exist, the subsequent login fails with the same safe configuration category.
- The two campus IDs/names, Asia/Shanghai business day, cache TTLs, refresh single-flight,
  warmup lifecycle, API envelopes, health/readiness routes, and installer contract do not
  change.
- Secrets remain only in memory and are never formatted into errors, logs, test names, or
  assertion output.
- Integration tests may read real credentials from the environment solely to decide whether
  to skip and to construct the real client; production service code may not.

## Trade-offs

- The change touches several small packages, but keeping config, cache, transport, JW client,
  and service construction in one task avoids a half-explicit startup graph.
- No generic container/options framework is introduced. Constructor parameter growth is
  accepted because the process has only one production composition root.
- Runtime configuration changes require process restart. This matches systemd deployment and
  the documented token-override invalidation lifetime.
- Configurable campuses remain deferred because they would add a new operator-facing contract
  rather than merely improve dependency ownership.

## Rollback Shape

The change has no data migration. A code rollback restores the old global constructors and
environment reads. Keep behavior changes isolated from backend responsibility splitting so
the commit can be reverted without touching cache payloads, API models, frontend code, or
release assets.
