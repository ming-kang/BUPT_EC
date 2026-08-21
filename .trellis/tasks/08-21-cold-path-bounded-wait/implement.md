# Implementation Plan — cold-path-bounded-wait

## Ordered Checklist

1. [ ] `service/classroom_service.go`: add `ColdWaitTimeout time.Duration` to
       `ClassroomServiceOptions`; resolve `<=0 → defaultColdWaitTimeout = 5s` on the struct.
2. [ ] `service/realtime_data.go`: define `ErrRefreshWaitTimeout`; add the `time.After` arm to the
       cold-miss select (observe `"wait_timeout"`, return sentinel, never cancel); comment why the
       refresh is left running (design D3).
3. [ ] `service/jw_error.go`: `SafeErrorMessage` case for `ErrRefreshWaitTimeout` → existing 暂无 copy.
4. [ ] `handler.go`: after the service error, `errors.Is` check against both sentinels → set
       `Retry-After: 5` before writeJSON.
5. [ ] `main.go`: `httpWriteTimeout = 15 * time.Second`.
6. [ ] Tests:
   - [ ] service: gated-refresh cold miss → `ErrRefreshWaitTimeout` under injected 20ms; release
         gate → next call serves fresh data; refresh completion observed (not cancelled).
   - [ ] service: zero-value options resolve 5s default (white-box via export_test seam if needed).
   - [ ] handler: Retry-After present for both sentinels; absent on success and on config-error 503.
   - [ ] Update any existing tests relying on unbounded cold blocking.
7. [ ] Docs: api-contract.md 503 section (+Retry-After), CHANGELOG `[Unreleased]` Changed,
       development.md if it cites 45s.

## Validation Commands

```bash
go test -race ./service/ ./... -count=1
go vet ./... && gofmt -l .
go build -tags embed_assets ./...        # after cp frontend/dist web/dist if needed
pnpm -C frontend typecheck && pnpm -C frontend lint && pnpm -C frontend test
rg -n "45 \* time.Second|httpWriteTimeout" main.go   # confirm single updated definition
```

## Risky Files / Rollback Points

| File | Risk | Mitigation |
|------|------|------------|
| realtime_data.go select | race on attempt state / missed done channel | keep original arms intact, add third arm only; -race run |
| handler.go | header leaks onto non-warming 503s | errors.Is gate + explicit handler tests for both polarities |
| main.go timeout | too-low value truncates slow clients on stale path | stale path serves from memory (no JW wait); 15s ≫ marshal+gzip cost |

Rollback = revert the single commit.

## Pre-start Checks

- implement.jsonl / check.jsonl curated (runtime-state-and-cache spec, api-contract spec, this design).
- Previous task gates green on HEAD.
