# Implementation Plan: Performance Optimization

## Phase 1: Backend Typed Structs + /healthz Constant (R1, R3)

### Steps

1. [x] Define typed response structs in `handler.go` (getDataSuccessResponse, getDataErrorResponse, readyzResponse, readyzDiagnosticsResponse, notFoundResponse).
2. [x] Replace `map[string]any` in `GetData`, `Readyz`, and the 404 fallback with the typed structs.
3. [x] Replace `/healthz` with `w.Write(healthzBody)` (pre-computed `[]byte`).
4. [x] Run `go test -race ./...` — all existing handler_test.go assertions must pass unchanged (byte-compatible JSON).
5. [x] Add `BenchmarkGetDataAllocations` in `handler_test.go` to baseline allocs/op.

### Validation

```bash
go test -race ./...
go test -run BenchmarkGetDataAllocations -bench . -benchmem
```

### Rollback

Pure refactor — revert the commit if tests fail.

---

## Phase 2: writeJSON Buffer Pool (R4)

### Steps

1. [x] Add `var jsonBufPool = sync.Pool{...}` in `router.go`.
2. [x] Rewrite `writeJSON` to use pooled buffer + `json.NewEncoder`.
3. [x] Trim trailing newline from Encoder output (Encode adds `\n`; Marshal does not).
4. [x] Run handler tests — verify byte-identical output.

### Validation

```bash
go test -race ./...
```

---

## Phase 3: Pre-serialized JSON Cache (R2)

### Steps

1. [x] Define `todayCacheEntry` in `service/classroom_service.go`, carrying both `*model.TodayClassrooms` and an immutable fresh JSON string.
2. [x] Replace the separate model and JSON atomic pointers with one `atomic.Pointer[todayCacheEntry]`.
3. [x] Publish refresh results through `publishTodayCache`, which marshals first and makes one atomic store; the test cache seeding seam uses the same helper.
4. [x] Keep `GetCachedDataJSON() (string, bool)` as the handler boundary, but load, same-day validate, check fresh/complete/error-free status, and return the immutable representation from one entry.
5. [x] In `handler.go GetData`, use the pre-serialized fresh path when available and retain typed runtime serialization for stale, partial, cold-start, and error responses.
6. [x] Keep `writePreserializedGetData(w, status, logID, version string, dataJSON string)` in `router.go` using the buffer pool.
7. [x] Measure the allocation-reduction target with the optimization parent (`30e4494`) as the same-machine baseline: handler-only allocations fell from 25 to 12 (52%), while the full route fell from 37 to 24 (35%). Benchmarks discard request slog output so they remain practical.

### Validation

```bash
gofmt -w service/classroom_service.go service/realtime_data.go service/export_test.go service/cache_policy_test.go handler_test.go
go test ./service -run 'Test(TodayCacheEntryKeepsModelAndJSONInOneGeneration|TodayCacheEntryPublicationKeepsConcurrentGenerationsCoherent|GetCachedDataJSONRejectsCrossDayEntry|GetCachedDataJSONRejectsIneligibleEntry)'
go test . -run TestGetDataFastAndSlowPathsAreHTTPIdentical
go test -run '^$' -bench 'Benchmark(GetDataSuccess|GetDataHandlerOnly|Healthz)$' -benchmem -count=3
```

### Risky Files

- `service/classroom_service.go` / `service/realtime_data.go` — publication must be one immutable entry store, rather than two independently loaded pointers. Read paths must keep the loaded entry through same-day, freshness, completeness, and error checks.
- `handler.go` — the fast path bypasses writeJSON; it must still set Content-Type, status, and write the exact same JSON shape as the typed path.

---

## Phase 4: Frontend Card Replacement (R5)

### Steps

1. [x] Create `frontend/src/components/Panel.tsx` — a `<div className="panel">` wrapper.
2. [x] Create `frontend/src/components/Panel.css` with border, radius, padding, background matching antd Card token values.
3. [x] Add dark-mode CSS variables for panel background and border.
4. [x] Replace `<Card>` imports in BuildingPicker, ClassTimePicker, TodayClassroomTable, GlobalEmpty with `<Panel>`.
5. [x] Remove `Card` from the antd import list in each file.
6. [x] Run `pnpm lint`, `pnpm typecheck`, `pnpm test`.
7. [x] Run `pnpm build && node scripts/check-bundle-size.mjs` — confirm size reduction.

### Validation

```bash
cd frontend && pnpm lint && pnpm typecheck && pnpm test -- --run
pnpm build && node scripts/check-bundle-size.mjs
```

---

## Phase 5: Frontend Tag Replacement (R6)

### Steps

1. [x] Add `.room-tag` styles to `TodayClassroomTable.css` (or a shared tags.css).
2. [x] Replace `<Tag color="...">` with `<span className="room-tag room-tag--{color}">` in TodayClassroomTable.
3. [x] Remove `Tag` from antd imports.
4. [x] Run lint/typecheck/test/build/size-check.

---

## Phase 6: Frontend Typography Replacement (R7)

### Steps

1. [x] Replace `Typography.Text` with `<span>` (add `className="text-secondary"` where `type="secondary"` was used).
2. [x] Replace `Typography.Title` with native `<hN>` elements.
3. [x] Add corresponding CSS classes to the component's colocated CSS or a shared `typography.css`.
4. [x] Remove `Typography` from antd imports in each file.
5. [x] Run lint/typecheck/test/build/size-check.

---

## Final Verification

```bash
# Backend
go test -race ./...
go test -bench BenchmarkGetData -benchmem

# Frontend
cd frontend
pnpm lint
pnpm typecheck
pnpm test -- --run
pnpm build
node scripts/check-bundle-size.mjs
```

## Completion Evidence

- `go test -race ./...`, `go vet ./...`, `go mod tidy -diff`, `go mod verify`, `go build ./...`, and `go build -tags embed_assets ./...`: passed.
- Focused cache publication/eligibility tests passed repeatedly, including under `-race`; fast/slow handler parity and zero-allocation health tests passed.
- Optimization-parent baseline (`30e4494`) versus final, same machine: handler-only `25 → 12 allocs/op` (52% reduction); full route `37 → 24 allocs/op` (35% reduction).
- `BenchmarkHealthz`: `0 B/op`, `0 allocs/op` with the isolated handler writer.
- Frontend lint/typecheck passed; Vitest passed 18 files / 127 tests; production build passed; bundle measured 164,862 B gzip against the 224,343 B task baseline and 230,888 B enforced budget.
- Final independent read-only review found no blocker/high/medium/low findings; `task.py validate` and `git diff --check` passed.

## Follow-up

- Update CHANGELOG.md [Unreleased] with performance improvements.
- Consider adding alloc-count CI gate (go test -benchmem threshold) in a future task.
