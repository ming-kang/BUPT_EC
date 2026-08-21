# Design — cold-path-bounded-wait

## Change Shape

Three seams, no architectural change:

```text
service/realtime_data.go   GetTodayClassrooms: select gains a time.After(coldWaitTimeout) arm
service/classroom_service.go (options)  ColdWaitTimeout field + default resolution
service/jw_error.go        ErrRefreshWaitTimeout sentinel + SafeErrorMessage case
handler.go                 Retry-After header on 503 when errors.Is matches either sentinel
main.go                    httpWriteTimeout 45s → 15s
```

## Key Decisions

### D1. Timeout lives in the service, not the handler
The blocking `select` is inside `GetTodayClassrooms`; bounding it there keeps the handler thin
(spec: handler stays smallest-possible method) and lets unit tests exercise the wait with injected
short timeouts. The handler only translates the resulting sentinel into a header.

### D2. New sentinel `ErrRefreshWaitTimeout`, distinct from `ErrNoTodayCache`
Both render the same safe copy (R3) but mean different things operationally:
`ErrNoTodayCache` = "a refresh is running, you just missed it" (immediate return);
`ErrRefreshWaitTimeout` = "we waited Ns for the refresh we started and gave up".
Distinct sentinels let the handler set `Retry-After` precisely (R2) and give metrics a clean
label without string-matching wrapped text.

### D3. Never cancel the refresh on expiry
The goroutine-side attempt belongs to the singleflight coordinator; abandoning the wait must not
abort the work — the whole point is that poll #2 (10s later, ladder rung 1) finds fresh data.
Implementation-wise this falls out naturally: we simply stop selecting on `attempt.done`; nothing
cancels the refresh context. Documented in code comment to prevent a future "cleanup" from adding
cancellation.

### D4. `Retry-After: 5` as a fixed constant in the handler
Computing "seconds until ready" would require exposing coordinator internals for marginal value —
the frontend ladder's first rung (10s) dominates anyway. A constant honors the RFC semantics
("delay the client SHOULD wait") without new coupling. Header-only; body log_id correlation is
unaffected (X-Log-Id middleware runs before handlers).

### D5. Default 5s via options normalization
`ColdWaitTimeout <= 0 → defaultColdWaitTimeout(5s)` resolved at construction, so zero-value
`ClassroomServiceOptions` keeps working and tests inject e.g. 20ms. Not configurable via
environment — operators have no reason to tune it, and an env knob would need docs/spec churn
(composition-root contract).

### D6. WriteTimeout 15s
Worst-case handler time after this change ≈ ColdWaitTimeout (5s) + marshal/IO margin. 15s keeps
~3× headroom while cutting the worst-case socket hold from 45s. `gracefulShutdownTimeout`
(= WriteTimeout + 5s → 20s) still comfortably exceeds any in-flight write.

## Compatibility / Migration

- Wire contract ADDITION only (`Retry-After` response header on some 503s); existing clients that
  ignore it behave exactly as before except failing faster during cold start.
- Existing service tests that rely on unbounded cold blocking are expected to be few: mocked JW
  clients complete quickly (< default 5s), so they pass unchanged; only tests with gated/hanging
  refreshes need the injected short timeout or explicit `ErrRefreshWaitTimeout` expectation.
- Metrics label `wait_timeout` is additive; dashboards unaffected.

## Rollback

Single-commit revert restores unbounded waiting + 45s timeout. No data/state migration involved.
