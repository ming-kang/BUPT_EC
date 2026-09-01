# Performance Optimization: Backend Pre-serialization & Frontend Antd Replacement

## Goal

Reduce per-request memory allocations and JSON serialization overhead on the backend, and reduce frontend gzipped bundle size by replacing heavyweight Ant Design components with native HTML+CSS equivalents. The user-facing behavior, API contract, and visual appearance remain unchanged.

## Background

The backend serves the same immutable `*model.TodayClassrooms` payload (refreshed every 5 minutes) to every request, but re-serializes it to JSON on each call via `json.Marshal(map[string]any{...})`. Each request allocates a fresh map, boxes interface values, and walks the full struct tree via reflection.

The frontend is at 224,343 B gzip (budget 230,888 B) with only ~6 KB headroom. Ant Design's `Card`, `Tag`, and `Typography` components are used as simple visual wrappers but pull in full rc-* dependency chains.

## Confirmed Technical Facts

- `handler.go:56-79`: `GetData` builds a `map[string]any` with `"code"`, `"log_id"`, `"version"`, `"data"` on every request.
- `router.go:33-40`: `writeJSON` calls `json.Marshal(v any)` each time (no buffer pooling).
- `service/classroom_service.go:44`: `todayCache atomic.Pointer[model.TodayClassrooms]` — immutable after Store.
- `service/realtime_data.go:311-323`: `classroomResponse()` shallow-copies before setting Stale/Error.
- Cache variants: fresh (Stale=false, Error=nil), stale (Stale=true, Error=nil or set), partial (Stale=false, Error set, PartialCampuses populated).
- `router.go:33`: `writeJSON` is also used for `/healthz` (constant body), `/readyz`, and 404 fallbacks.
- Frontend: `Card` in BuildingPicker, ClassTimePicker, TodayClassroomTable, GlobalEmpty — always a container div.
- Frontend: `Tag` in TodayClassroomTable — a colored label span.
- Frontend: `Typography.Text` and `Typography.Title` — styled text spans/headings.
- Vite chunks: `react-vendor` + `antd-vendor` (captures ALL antd/rc-* as one block).
- check-bundle-size.mjs budget: 230,888 B gzip-9 over all js/css/html/svg in dist/.

## Requirements

### R1: Backend typed response structs

Replace `map[string]any` in `handler.go` with typed Go structs for all envelope shapes (success, error, readyz, healthz, 404). Enables the `encoding/json` struct encoder cache and eliminates per-request map/interface-box allocations.

### R2: Backend pre-serialized JSON for /api/get_data

Cache the pre-serialized JSON `data` field at refresh time. On the hot path, write the envelope header (code, log_id, version) + the pre-built `data` bytes directly, avoiding per-request `json.Marshal` of the full campus/room tree.

Handle the variant cases:
- Fresh: pre-serialized data as-is.
- Stale: pre-serialized data with `"stale":true` patched or a separate stale variant.
- Error (failure, no cache): no pre-serialized data needed (writes `"data":null`).

### R3: Backend /healthz constant response

Pre-compute `[]byte(`{"status":"ok"}`)` once; serve it with zero per-request allocation.

### R4: Backend writeJSON buffer pool

Use a `sync.Pool` of `bytes.Buffer` for the remaining cases that still need runtime marshaling (readyz, 404 envelope, error envelope).

### R5: Frontend — replace antd Card with native div+CSS

Replace `<Card>` in BuildingPicker, ClassTimePicker, TodayClassroomTable, GlobalEmpty with a styled `<div>` or `<section>` that replicates the visual appearance (border-radius, padding, shadow).

### R6: Frontend — replace antd Tag with native span+CSS

Replace `<Tag>` in TodayClassroomTable with a `<span className="room-tag">` carrying the same colors/padding.

### R7: Frontend — replace antd Typography with native elements

Replace `Typography.Text` with `<span>` and `Typography.Title` with `<h2>`/`<h3>` across the app, applying the same type styles via CSS.

## Acceptance Criteria

- [x] `go test -race ./...` passes.
- [x] `pnpm -C frontend test` (127 tests) passes.
- [x] `pnpm -C frontend lint` and `pnpm -C frontend typecheck` pass.
- [x] `/api/get_data` benchmark shows ≥50% fewer allocations per request vs baseline.
- [x] `/healthz` handler achieves 0 allocs/op.
- [x] Frontend bundle size (gzip-9 total) stays ≤ 224,343 B (current baseline or lower); run `check-bundle-size.mjs`.
- [x] The UI renders identically (manual visual check): Card borders, Tag colors, Typography sizing unchanged.
- [x] No new runtime dependencies added to either Go or frontend.
- [x] Existing API contract unchanged: JSON field names, HTTP status codes, header behavior identical.

## Out of Scope

- Switching JSON libraries (go-json, sonic).
- Full antd removal or migration to headless UI.
- antd-vendor chunk splitting (high effort, no total-size reduction).
- Safari 16+ build target upgrade (breaks old iOS).
- Frontend component logic changes (SWR, revalidation, selection state).

## Notes

- R2 (pre-serialization) is the highest-value single change but requires careful variant handling. R1 (typed structs) is a natural prerequisite that simplifies R2.
- R5–R7 recover ~6-11 KB gzip budget headroom without risk, following the same pattern as the prior `@ant-design/icons` → inline SVG removal.
