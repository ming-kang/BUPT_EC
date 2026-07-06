# Runtime State and Cache

## No-Database Runtime Model

The backend does not maintain a timetable database. It fetches same-day
classroom availability from the BUPT JW HTTP API on demand, normalizes the rows,
and stores one process-local cache entry for the current day.

Reference files:

- `service/classroom_service.go`: `ClassroomService` and `CacheStore`.
- `service/realtime_data.go`: `GetTodayClassrooms`, `QueryAll`, cache TTLs, and
  `TodayCacheKey`.
- `cache/cache.go`: `cache.GlobalCache` backed by `github.com/patrickmn/go-cache`.
- `service/runtime_status.go`: readiness and runtime diagnostics.

Do not add ORM, migration, repository, or table abstractions unless a task
explicitly changes the product architecture. If you need durable operational
data, stop and ask for a design decision instead of quietly adding persistence.

## `ClassroomService` Owns Mutable State

All mutable runtime state for classroom queries is on `ClassroomService`:

- token and API URL state through `TokenManager`;
- the `CacheStore` implementation;
- configured campuses from `config.Config`;
- the injected `JWClient`;
- refresh coordination fields guarded by `refreshMu`;
- runtime status fields guarded by `statusMu`.

This shape is deliberate. New state should be added to `ClassroomService` with
the appropriate lock or injected dependency. Avoid package-level mutable globals
inside `service/`, because they make tests leak state and bypass per-service
mocking.

## Cache Semantics

`service/realtime_data.go` defines the cache contract:

- Key: `TODAY_CLASSROOMS_CACHE` (`TodayCacheKey`).
- Value: `*model.TodayClassrooms`.
- Fresh window: about five minutes (`classroomFreshTTL`).
- Stale window: until local end of day (`StaleUntil` from `endOfDay`).
- Cross-day protection: `getCachedTodayClassrooms` rejects entries whose `Date`
  differs from `now().Format("2006-01-02")`.

`GetTodayClassrooms` follows this order:

1. Return fresh same-day cache with `stale=false`.
2. If the entry is expired but still same-day, serve stale data while a
   background refresh continues.
3. If no usable cache exists, start or join a refresh and wait for the result.
4. If refresh cannot start because backoff is active, return the last refresh
   error or `ErrNoTodayCache`.

Do not reuse cache data across days. Do not create multiple cache keys for the
same public payload unless the API contract changes.

## Refresh Coordination

`service/refresh_coordinator.go` makes refreshes single-flight:

- `startClassroomRefresh` shares one `classroomRefreshAttempt` with concurrent
  callers.
- Failed refreshes set `nextRefreshAllowed` using `staleRefreshBackoff` (30s).
- Stale callers wait briefly (`staleRefreshWait`, 300ms) for a refresh to finish
  and otherwise return stale data immediately.
- `WaitWarmup` drains in-flight refresh workers during graceful shutdown.

Tests in `service/realtime_data_test.go` lock this behavior down with
`TestGetTodayClassroomsReturnsStaleWhileRefreshContinues`,
`TestGetTodayClassroomsBroadcastsRefreshResultToConcurrentWaiters`, and
`TestGetTodayClassroomsSharesWarmupRefreshResult`.

When changing refresh behavior, keep the shared-attempt model. Do not start one
JW query per HTTP caller during cache misses.

## JW Token and API URL State

`service/token_manager.go` owns JW token and API URL caching:

- `EnsureToken(ctx, false)` first honors `JW_TOKEN`, then an in-memory token,
  then performs login.
- `EnsureToken(ctx, true)` bypasses the `JW_TOKEN` override and cached token so
  auth retries can force a real login.
- `singleflight.Group` deduplicates concurrent login and API URL resolution.
- Login and API URL fetches use bounded contexts based on `jwRequestTimeout`.
- On auth failure, `queryCampus` clears only the current token and retries once
  with a forced login.

`service/urlutil.go` validates the server-provided API URL. Keep the HTTPS and
`*.bupt.edu.cn` restrictions; never trust or log a raw upstream URL that failed
validation.

## Runtime Status and Readiness

`/readyz` is based on two conditions:

- `config.HasJWCredentials()` is true; and
- `ClassroomService.HasUsableTodayCache()` reports a same-day cache that is
  fresh or stale-but-usable.

`handler.go` returns `RuntimeStatus` in the readiness body so operators can see
refresh/login timestamps and cache diagnostics. Keep status fields diagnostic;
do not add credentials, tokens, or raw upstream payloads.

## Anti-Patterns

- Adding a background scheduler when on-demand refresh plus startup warmup is
  enough for the existing API shape.
- Treating the process-local cache as shared across instances.
- Returning stale data after midnight.
- Clearing a newly refreshed token because an older request failed; use
  `clearTokenIfCurrent` semantics.
- Hiding refresh failures from runtime status or stale response metadata.
