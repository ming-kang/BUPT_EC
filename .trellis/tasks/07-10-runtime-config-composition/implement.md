# 运行时配置边界与依赖组装 — Implementation Plan

## Preconditions

- Preserve the current public HTTP/cache/auth behavior protected by existing tests.
- Do not add campus configuration, runtime reload, a DI framework, or unrelated package moves.
- Keep `.agents/` and `.codex/` untracked; `.trellis/.template-hashes.json` remains local-only.

## Ordered Implementation

1. **Add immutable runtime configuration loading**
   - replace `GlobalConfig`, `initOnce`, `InitConfig`, `GetConfig`, and direct credential
     helpers with `RuntimeConfig`, `JWCredentials`, map-backed dotenv/environment resolution,
     defaults, and validation;
   - distinguish missing dotenv from present-but-invalid dotenv;
   - validate credentials, Gin mode, and listen address without exposing values;
   - add table-driven `config/config_test.go` coverage.

2. **Remove the global cache instance**
   - replace `GlobalCache` / `InitCache` and unused package wrappers with `cache.New()`;
   - add a focused test proving calls return isolated cache instances and preserve expected
     default behavior.

3. **Make the outbound HTTP client explicit**
   - expose the small `HTTPDoer` seam and `utils.NewHTTPClient()`;
   - pass the doer into request helpers instead of using `defaultHTTPClient`;
   - update redirect/body-limit tests and add an assertion that the supplied doer is used;
   - preserve all transport security and timeout settings byte-for-behavior.

4. **Inject JW credentials and token override**
   - give the real JW client immutable username/password plus the explicit HTTP doer;
   - return a constructor error when the required HTTP doer is nil, without including
     credential values;
   - remove production `os.Getenv` from `jw_client.go`;
   - store the startup override token on `TokenManager` and remove its environment read;
   - update override invalidation/concurrency tests to inject values directly;
   - adapt real integration tests to construct credentials explicitly after their skip check.

5. **Make ClassroomService construction explicit**
   - introduce bounded service options for campuses and token override;
   - require the cache store and JW client at the exported production constructor and return
     explicit errors for missing dependencies;
   - clone campus input and preserve existing clock/jitter test seams;
   - update `newTestService` and all focused tests without introducing shared globals.

6. **Rebuild the main composition root**
   - load/validate runtime config before starting the server;
   - call `gin.SetMode`, pass `LogCaller` to logging, and construct cache, HTTP client,
     JW client, service, HTTP server, warmup, and shutdown dependencies in visible order;
   - replace `listenAddr` tests with config-level default/explicit-address tests where
     appropriate;
   - retain server timeouts and graceful-shutdown ordering unchanged.

7. **Synchronize contracts and documentation**
   - update `AGENTS.md`, `docs/development.md`, and relevant backend specs for the new
     composition/config signatures;
   - add an `[Unreleased]` changelog bullet for stricter malformed-dotenv handling and
     effective `.env` Gin mode;
   - search for stale `GlobalConfig`, `GlobalCache`, `InitConfig`, `InitCache`,
     `defaultHTTPClient`, and old constructor references.

## Focused Test Matrix

- dotenv missing / valid / malformed / unreadable;
- system environment wins over dotenv;
- token-only, username+password, incomplete credentials, and empty credentials;
- APP_ADDR default, loopback, wildcard, hostname and bracketed IPv6;
- Gin debug/release/test plus invalid value;
- LOG_CALLER `1`, `true`, mixed case and false values;
- independent cache instances;
- explicit HTTP doer, redirect refusal and response size limit;
- login uses injected username/password without reading changed process env;
- token override is injected once, rejected override stays invalidated, login token failures do
  not alter override invalidation;
- existing concurrent auth recovery, partial refresh, cache day boundary, warmup shutdown,
  handler readiness/error envelope, and integration-skip behavior.

## Validation

```powershell
$bad = gofmt -l .
if ($bad) { $bad; exit 1 }
go vet ./...
go test ./...
go test -race ./...
go build -o bupt-ec -v ./
git diff --check
```

Frontend commands are not required unless implementation unexpectedly changes frontend source
or its API contract. Integration tests must continue to skip cleanly without real credentials.

## Validation Results

- `gofmt -l .` passed with no output.
- `go vet ./...`, `go test ./...`, and `go test -race ./...` passed.
- A full Go binary build to a temporary output path passed.
- `git diff --check` passed apart from the existing CRLF conversion warning on the parent
  Trellis task JSON.
- `govulncheck` was not installed locally, so it was not run; dependencies were unchanged.
- Frontend checks were not run because no frontend source/package/API contract changed.

## Review Gates

- Confirm `rg "os\.Getenv|LookupEnv" service logs main.go` finds no production runtime config
  reads outside the config composition boundary (test-only environment checks are allowed).
- Confirm `rg "GlobalConfig|GlobalCache|InitConfig|InitCache|defaultHTTPClient"` finds no stale
  production references.
- Trace credentials from `config.Load` to `NewJWClient` / `TokenManager` and verify no log or
  error formatting includes their values.
- Verify `utils.NewHTTPClient` still rejects every redirect and retains all current transport
  timeout/pooling settings.
- Verify `.agents/`, `.codex/`, and local template hashes are absent from staged changes.

## Rollback Points

- Configuration loader/tests can be reverted independently before constructor migration.
- Cache constructor cleanup is mechanically reversible and carries no persisted state.
- HTTP helper signature changes and JW client injection should land in the same work commit so
  no intermediate revision has an implicit client.
- If the complete constructor graph cannot be proven by tests, restore the prior production
  constructors rather than leaving compatibility wrappers that reintroduce environment reads.
