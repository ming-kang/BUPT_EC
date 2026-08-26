# Implementation Plan: Performance Optimization

## Phase 1: Backend Typed Structs + /healthz Constant (R1, R3)

### Steps

1. [ ] Define typed response structs in `handler.go` (getDataSuccessResponse, getDataErrorResponse, readyzResponse, readyzDiagnosticsResponse, notFoundResponse).
2. [ ] Replace `map[string]any` in `GetData`, `Readyz`, and the 404 fallback with the typed structs.
3. [ ] Replace `/healthz` with `w.Write(healthzBody)` (pre-computed `[]byte`).
4. [ ] Run `go test -race ./...` — all existing handler_test.go assertions must pass unchanged (byte-compatible JSON).
5. [ ] Add `BenchmarkGetDataAllocations` in `handler_test.go` to baseline allocs/op.

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

1. [ ] Add `var jsonBufPool = sync.Pool{...}` in `router.go`.
2. [ ] Rewrite `writeJSON` to use pooled buffer + `json.NewEncoder`.
3. [ ] Trim trailing newline from Encoder output (Encode adds `\n`; Marshal does not).
4. [ ] Run handler tests — verify byte-identical output.

### Validation

```bash
go test -race ./...
```

---

## Phase 3: Pre-serialized JSON Cache (R2)

### Steps

1. [ ] Add `type cachedJSON struct { freshBytes, staleBytes []byte }` to `service/classroom_service.go`.
2. [ ] Add `jsonCache atomic.Pointer[cachedJSON]` field on `ClassroomService`.
3. [ ] In `doRefreshTodayClassrooms`, after `todayCache.Store(today)`, pre-marshal fresh and stale variants and store in `jsonCache`.
4. [ ] Export a method `GetPreserializedJSON(stale bool) ([]byte, bool)` that returns the cached bytes (false if unavailable or partial_campuses mismatch).
5. [ ] In `handler.go GetData`: on success path, attempt `GetPreserializedJSON`; if available, write envelope header + cached data bytes directly (skip writeJSON). If unavailable, fall back to the typed struct marshal.
6. [ ] Add a helper `writePreserializedGetData(w, status, logID, version string, dataJSON []byte)` in `router.go` using the buffer pool.
7. [ ] Update `BenchmarkGetDataAllocations` — confirm ≥50% alloc reduction.

### Validation

```bash
go test -race ./...
go test -run BenchmarkGetData -bench . -benchmem
```

### Risky Files

- `service/realtime_data.go` — adding the pre-serialization call must not change the refresh timing or error handling. The marshal is appended after the existing `todayCache.Store` and before the return.
- `handler.go` — the fast path bypasses writeJSON; must still set Content-Type, status, and write the exact same JSON shape.

---

## Phase 4: Frontend Card Replacement (R5)

### Steps

1. [ ] Create `frontend/src/components/Panel.tsx` — a `<div className="panel">` wrapper.
2. [ ] Create `frontend/src/components/Panel.css` with border, radius, padding, background matching antd Card token values.
3. [ ] Add dark-mode CSS variables for panel background and border.
4. [ ] Replace `<Card>` imports in BuildingPicker, ClassTimePicker, TodayClassroomTable, GlobalEmpty with `<Panel>`.
5. [ ] Remove `Card` from the antd import list in each file.
6. [ ] Run `pnpm lint`, `pnpm typecheck`, `pnpm test`.
7. [ ] Run `pnpm build && node scripts/check-bundle-size.mjs` — confirm size reduction.

### Validation

```bash
cd frontend && pnpm lint && pnpm typecheck && pnpm test -- --run
pnpm build && node scripts/check-bundle-size.mjs
```

---

## Phase 5: Frontend Tag Replacement (R6)

### Steps

1. [ ] Add `.room-tag` styles to `TodayClassroomTable.css` (or a shared tags.css).
2. [ ] Replace `<Tag color="...">` with `<span className="room-tag room-tag--{color}">` in TodayClassroomTable.
3. [ ] Remove `Tag` from antd imports.
4. [ ] Run lint/typecheck/test/build/size-check.

---

## Phase 6: Frontend Typography Replacement (R7)

### Steps

1. [ ] Replace `Typography.Text` with `<span>` (add `className="text-secondary"` where `type="secondary"` was used).
2. [ ] Replace `Typography.Title` with native `<hN>` elements.
3. [ ] Add corresponding CSS classes to the component's colocated CSS or a shared `typography.css`.
4. [ ] Remove `Typography` from antd imports in each file.
5. [ ] Run lint/typecheck/test/build/size-check.

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

## Follow-up

- Update CHANGELOG.md [Unreleased] with performance improvements.
- Consider adding alloc-count CI gate (go test -benchmem threshold) in a future task.
