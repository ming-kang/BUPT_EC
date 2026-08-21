# Cold path bounded wait: 503 + Retry-After instead of 30s block

Audit item B-03 (`archive/2026-08/08-07-project-audit-optimize/research/audit-backend.md` §B-03).
Product tradeoff ratified by owner 2026-08-21: during the cold-start window users see a clear
error after ~5s instead of a spinner for up to 30s.

## Goal

Bound how long `/api/get_data` waits for a cold-start refresh, fail fast with `503` +
`Retry-After`, and lower `httpWriteTimeout` accordingly. Background refresh continues
uninterrupted so the next poll succeeds.

## Background (verified 2026-08-21)

- Cold path today (`service/realtime_data.go:77-92`): on cache miss the request that STARTS the
  refresh blocks in `select { <-attempt.done; <-ctx.Done() }` until the full refresh finishes —
  up to `ClassroomRefreshLimit = 30s` (`realtime_data.go:22`). A request that finds a refresh
  already running returns immediately with `ErrNoTodayCache` (`:73-76`).
- The 503 envelope + safe copy already exist: handler maps any service error to
  `503 {code:503, msg:SafeErrorMessage(err), log_id, data:null}` (`handler.go:55-61`);
  `ErrNoTodayCache` renders as 「暂无可用的今日空教室数据，请稍后重试」(`jw_error.go:72-74`).
- Frontend (`useTodayClassrooms.ts`): fetcher throws `ApiError(503)` → SWR error track → retry
  ladder 10/20/30/60s. First rung (10s) already exceeds any sane Retry-After ⇒ **no frontend
  change required**; `Retry-After` is informational for non-browser clients.
- `main.go:84`: `httpWriteTimeout = 45s` exists solely to cover the 30s block;
  `gracefulShutdownTimeout = httpWriteTimeout + 5s` derives from it (`main.go:88`).
- Stale/partial paths never block (serve cached snapshot, refresh in background) — only the cold
  miss blocks.

## Requirements

1. R1 — Bounded wait: on cold miss, `GetTodayClassrooms` waits at most `ColdWaitTimeout`
   (new `ClassroomServiceOptions` field, default **5s**, ≤0 → default) for the in-flight refresh.
   On expiry return a new sentinel `ErrRefreshWaitTimeout` **without cancelling the refresh**
   (singleflight attempt keeps running; the next poll converges).
2. R2 — Retry-After: `GetData` sets `Retry-After: 5` (seconds, constant) on 503 responses when
   the error is `ErrNoTodayCache` or `ErrRefreshWaitTimeout` (errors.Is); other 503 causes
   (config/auth failures) stay header-free.
3. R3 — Safe copy: `SafeErrorMessage(ErrRefreshWaitTimeout)` reuses the existing
   「暂无可用的今日空教室数据，请稍后重试」copy (same user situation).
4. R4 — Metrics: expiry observes a distinct `ObserveCacheServe("wait_timeout")` label so cold-start
   pressure is visible; the immediate-miss path keeps "miss".
5. R5 — Timeout lowering: `httpWriteTimeout` 45s → **15s** (≈3× the bounded wait);
   `gracefulShutdownTimeout` derivation untouched.
6. R6 — No frontend changes; no cancellation of in-flight refreshes; no changes to stale/partial
   paths.

## Acceptance Criteria

- [ ] Service test: gated (slow) refresh on cold miss returns `ErrRefreshWaitTimeout` within the
      injected tiny `ColdWaitTimeout`; after releasing the gate, a subsequent call serves fresh
      data; the refresh was not cancelled (attempt completes).
- [ ] Default-option test: zero-value options yield the 5s default.
- [ ] Handler tests: 503 carries `Retry-After: 5` for both sentinels; success and config-error
      responses carry no `Retry-After`.
- [ ] Existing cold-path tests updated only where they relied on unbounded blocking; all suites
      green: `go test -race ./...`, frontend typecheck/lint/test/build untouched-but-green.
- [ ] `httpWriteTimeout = 15s`; embed build green.
- [ ] Docs synced: api-contract.md (Retry-After on 503), CHANGELOG `[Unreleased]` Changed entry
      (user-visible cold-start behavior), development.md if it documents the 45s value.

## Out of Scope

- Frontend Retry-After-aware scheduling (ladder already ≥ Retry-After).
- Changes to warmup scheduling/jitter, stale-while-revalidate, or partial-refresh semantics.
- The ETag/pre-serialization work (separate deferred item).
