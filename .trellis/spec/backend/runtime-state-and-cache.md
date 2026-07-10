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
- warmup/background lifecycle fields guarded by `backgroundMu`;
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
- `startClassroomRefresh` holds `backgroundMu` while calling
  `refreshWorkers.Add`, so `WaitBackground` can stop new workers before waiting.

Tests in `service/realtime_data_test.go` lock this behavior down with
`TestGetTodayClassroomsReturnsStaleWhileRefreshContinues`,
`TestGetTodayClassroomsBroadcastsRefreshResultToConcurrentWaiters`, and
`TestGetTodayClassroomsSharesWarmupRefreshResult`.

When changing refresh behavior, keep the shared-attempt model. Do not start one
JW query per HTTP caller during cache misses.

## Scenario: Warmup Scheduler and Background Shutdown

### 1. Scope / Trigger

Apply this contract whenever startup warmup, Shanghai day-boundary scheduling,
refresh retry timing, process shutdown, or refresh-worker draining changes.
Warmup must recover the cache without user traffic while still sharing the
normal refresh coordinator.

### 2. Signatures

```go
func (s *ClassroomService) StartWarmup(ctx context.Context)
func (s *ClassroomService) WaitBackground(ctx context.Context) error

func nextWarmupDelay(
    now time.Time,
    cacheState warmupCacheState,
    nextAllowed time.Time,
    failures int,
    midnightJitter time.Duration,
) time.Duration
```

### 3. Contracts

- `StartWarmup` starts at most one scheduler per service and attempts refresh
  immediately unless its context is already canceled.
- Every scheduler wait uses `time.NewTimer` plus `select` on `ctx.Done()`; do not
  use an uninterruptible `time.Sleep` loop.
- No usable cache retries after 30s, 1m, 2m, then 5m maximum, with
  `nextRefreshAllowed` taking precedence when later.
- Partial cache is ready but retries no faster than `classroomFreshTTL`; a new
  Shanghai day boundary may wake it earlier.
- Complete cache waits until next Shanghai midnight plus a randomized 1–5s
  jitter. A usable outcome resets the consecutive failure count.
- `main.go` cancels the scheduler before HTTP shutdown, drains handlers, then
  calls `WaitBackground`.
- `WaitBackground` marks background state as stopping before `WaitGroup.Wait`.
  `startClassroomRefresh` checks the same lock, so no later worker can call Add.

### 4. Validation & Error Matrix

| Condition | Required scheduling/lifecycle result |
| --- | --- |
| startup with active context | refresh immediately |
| startup with canceled context | no refresh worker |
| no cache, first/second/third/fourth failure | 30s / 1m / 2m / 5m |
| retry target before `nextRefreshAllowed` | wait until `nextRefreshAllowed` |
| partial same-day cache | wait at least fresh TTL, unless day boundary is earlier |
| full same-day cache | next midnight + injected jitter |
| repeated `StartWarmup` | existing scheduler/channel unchanged |
| shutdown begins | reject new refresh workers, exit scheduler, drain existing workers |

### 5. Good/Base/Bad Cases

- Good: a refresh fails just before midnight and sets backoff past midnight;
  warmup retries after the allowed timestamp and populates the new day's cache.
- Base: startup completes a full refresh, resets failures, and sleeps until the
  next day boundary.
- Bad: midnight refresh is suppressed once, then the process sleeps until the
  following midnight.

### 6. Tests Required

- Pure delay tests cover cross-midnight `nextRefreshAllowed`, capped failure
  delays, partial fresh-TTL policy, and full-cache midnight jitter.
- Lifecycle tests cover immediate start, pre-canceled context, duplicate start,
  scheduler cancellation, and rejection of workers after drain begins.
- Run `go test -race ./...` to protect the `WaitGroup.Add` / `Wait` boundary.
- Main tests assert the shutdown timeout exceeds `ClassroomRefreshLimit`.

### 7. Wrong vs Correct

#### Wrong

```go
for {
    time.Sleep(time.Until(endOfDay(time.Now())))
    s.runWarmupOnce()
}
```

#### Correct

```go
delay := nextWarmupDelay(now, state, nextAllowed, failures, jitter)
timer := time.NewTimer(delay)
select {
case <-timer.C:
case <-ctx.Done():
    return
}
```

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

## Scenario: Token Auth Recovery and Shared Cancellation

### 1. Scope / Trigger

Apply this contract whenever token caching, `JW_TOKEN`, auth retry, login/API URL
singleflight, or caller cancellation changes. It prevents delayed auth failures
from deleting a newer token or starting mutually invalidating logins.

### 2. Signatures

```go
type tokenSource int // none, override, login

func (m *TokenManager) EnsureToken(ctx context.Context, forceRefresh bool) (string, error)
func (m *TokenManager) RefreshAfterAuthFailure(ctx context.Context, failedToken string) (string, error)
func (m *TokenManager) APIURL(ctx context.Context) (string, error)
```

### 3. Contracts

- Cached token state stores both value and source. `JW_TOKEN` is installed as
  `tokenSourceOverride`; successful login uses `tokenSourceLogin`.
- `RefreshAfterAuthFailure` receives the exact rejected token and rechecks state
  inside the token singleflight closure.
- If current token differs from `failedToken`, return it without login. If it
  matches, clear it; invalidate the environment override only when its source
  was `tokenSourceOverride`.
- Auth recovery performs at most one login and `queryCampus` retries its JW query
  exactly once.
- Token and API URL groups use `DoChan`. Shared work runs under
  `context.WithTimeout(context.WithoutCancel(caller), jwRequestTimeout)` so one
  waiter cannot cancel the operation for others.
- Each caller selects the shared result against its own `ctx.Done()` and may
  return early.
- `EnsureToken(ctx, true)` still guarantees a real login even if it first joins
  an operation that only reused or installed a token.
- Never log token values, credentials, request headers, or upstream bodies.

### 4. Validation & Error Matrix

| Condition | Required result |
| --- | --- |
| simultaneous failures for same token | one login, all waiters receive replacement |
| old failure arrives after replacement installed | reuse replacement, no login |
| rejected source is override | set `overrideInvalidated=true` |
| rejected source is login | preserve existing override-invalidated state |
| one waiter cancels | that waiter gets context error; shared operation continues |
| shared login/API URL exceeds timeout | bounded operation returns timeout error |
| retry query fails | return joined original-auth and retry errors; do not loop |

### 5. Good/Base/Bad Cases

- Good: both campus queries fail with token A; one login installs token B, and a
  delayed second failure for A observes/reuses B.
- Base: cached login token succeeds and no singleflight operation runs.
- Bad: delayed failure for A blindly clears current token B and forces login C,
  making the first retry fail because B was invalidated upstream.

### 6. Tests Required

- Concurrent and deliberately delayed auth failures assert exactly one login.
- The delayed test rejects any second generated token to model upstream token
  invalidation between logins.
- Override-source and login-source tests assert `overrideInvalidated` behavior.
- Token and API URL cancellation tests assert canceled waiter + surviving waiter
  outcomes and one underlying operation.
- Run `go test -race ./service`; integration tests must still skip without
  credentials.

### 7. Wrong vs Correct

#### Wrong

```go
m.clearTokenIfCurrent(failedToken)
token, err := m.EnsureToken(ctx, true)
```

#### Correct

```go
token, err := m.RefreshAfterAuthFailure(ctx, failedToken)
// The singleflight closure checks whether another goroutine already replaced it.
```

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

- Adding an independent scheduler that bypasses refresh single-flight/backoff or
  cannot be canceled and drained during shutdown.
- Treating the process-local cache as shared across instances.
- Returning stale data after midnight.
- Clearing a newly refreshed token because an older request failed; pass the
  failed token to `RefreshAfterAuthFailure` and recheck inside singleflight.
- Hiding refresh failures from runtime status or stale response metadata.
