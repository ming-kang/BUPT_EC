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

## Scenario: Full, Partial, and Failed Refresh Outcomes

### 1. Scope / Trigger

Apply this contract whenever campus query aggregation, refresh backoff, stale
response selection, runtime diagnostics, or the public classroom payload
changes. The outcome must not be inferred only from `TodayClassrooms.error`.

### 2. Signatures

Internal coordination uses:

```go
type refreshKind int // refreshFull, refreshPartial, refreshFailed

type classroomRefreshResult struct {
    value    *model.TodayClassrooms
    kind     refreshKind
    failures []campusRefreshFailure
    err      error // total failure only
}
```

Public methods remain compatible:

```go
QueryAll(context.Context) (*model.TodayClassrooms, error)
GetTodayClassrooms(context.Context) (*model.TodayClassrooms, error)
```

### 3. Contracts

| Outcome | Cache | Backoff | Runtime/log |
| --- | --- | --- | --- |
| full | replace with complete payload | clear | success, no warning/error |
| partial | cache usable payload and `partial_campuses` | 30 seconds | warning with failed campus IDs |
| failed | preserve existing cache | 30 seconds | total error |

`cache_fresh` describes cache age. `cache_partial` independently describes
completeness. A fresh partial cache can be ready.

### 4. Validation & Error Matrix

| Condition | Required response |
| --- | --- |
| all campuses succeed | HTTP 200 payload without `error` |
| at least one campus succeeds | HTTP 200 payload with safe partial `error` |
| all fail, no cache | service error -> HTTP 503 at handler |
| stale partial cache, latest total failure | HTTP 200 stale payload with latest total-failure warning |
| refresh still in flight after stale wait | cached warning remains until an outcome exists |

### 5. Good/Base/Bad Cases

- Good: Shahe fails, Xitucheng succeeds; cache identifies `04` and remains ready.
- Base: both campuses succeed; partial diagnostics are empty.
- Bad: a later total outage still shows only the old “部分校区” warning.

### 6. Tests Required

- Full, partial, and failed outcomes have separate unit coverage.
- Partial cache followed by total failure asserts latest-error precedence both
  immediately and during backoff.
- Runtime/readiness tests assert `cache_partial` and `partial_campuses`.
- Race tests preserve the single shared refresh attempt.

### 7. Wrong vs Correct

#### Wrong

```go
if result.value.Error != nil { /* infer partial */ }
```

#### Correct

```go
switch result.kind {
case refreshFull:
case refreshPartial:
case refreshFailed:
}
```

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
