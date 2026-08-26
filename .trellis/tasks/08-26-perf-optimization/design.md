# Design: Performance Optimization

## Architecture Overview

The optimization operates in two orthogonal tracks:

1. **Backend hot-path allocation reduction** — zero-copy JSON serving for the dominant endpoint.
2. **Frontend bundle trimming** — replace heavyweight antd wrappers with native HTML+CSS.

Neither track changes the API contract, data flow, or component behavior.

---

## Backend Design

### Typed Response Structs (R1)

```go
// handler.go

type getDataSuccessResponse struct {
    Code    int                    `json:"code"`
    LogID   string                 `json:"log_id"`
    Version string                 `json:"version"`
    Data    *model.TodayClassrooms `json:"data"`
}

type getDataErrorResponse struct {
    Code    int    `json:"code"`
    Msg     string `json:"msg"`
    LogID   string `json:"log_id"`
    Version string `json:"version"`
    Data    any    `json:"data"` // always nil, serializes as null
}

type healthzResponse struct {
    Status string `json:"status"`
}

type readyzResponse struct {
    Status  string         `json:"status"`
    Version string         `json:"version"`
}

type readyzDiagnosticsResponse struct {
    Status                  string        `json:"status"`
    JWCredentialsConfigured bool          `json:"jw_credentials_configured"`
    Runtime                 RuntimeStatus `json:"runtime"`
    Version                 string        `json:"version"`
}

type notFoundResponse struct {
    Code  int    `json:"code"`
    Msg   string `json:"msg"`
    LogID string `json:"log_id"`
}
```

Benefits: struct encoder cache in encoding/json (one-time reflection, zero map/bucket allocs).

### Pre-serialized JSON Cache (R2)

Strategy: cache the `"data":{ ... }` JSON bytes at refresh time. On read, assemble the full envelope by concatenating pre-built fragments.

```go
// service/classroom_service.go (new field)
type classroomJSONCache struct {
    freshJSON []byte // pre-marshaled full TodayClassrooms for fresh variant
    staleJSON []byte // same payload with "stale":true patched
}

// Updated on every todayCache.Store():
//   freshJSON = json.Marshal(todayData)
//   staleJSON = json.Marshal(todayDataWithStale)
```

The handler then writes:
```go
// Conceptual (actual implementation uses a writer pattern):
buf := pool.Get()
buf.WriteString(`{"code":0,"log_id":"`)
buf.WriteString(logID)
buf.WriteString(`","version":"`)
buf.WriteString(version)
buf.WriteString(`","data":`)
buf.Write(cachedJSON)
buf.WriteByte('}')
w.Write(buf.Bytes())
pool.Put(buf)
```

**Variant handling:**
- **Fresh (code=0, stale=false, no error):** Use `freshJSON`.
- **Stale (code=0, stale=true, error object):** The stale response has additional fields (`stale`, `error`, `partial_campuses`). Since `classroomResponse()` copies and mutates these, we pre-serialize the stale variant too at refresh time. If partial_campuses differ from the cached variant, fall back to runtime marshal (rare case: only on partial-campus degradation).
- **Error (no cache, code≠0):** Small envelope with `"data":null` — typed struct marshal with buffer pool (R4). No pre-serialization needed.

**Invalidation:** The pre-serialized bytes are stored alongside `todayCache` as a sibling `atomic.Pointer`. Both are set atomically in `doRefreshTodayClassrooms` after building the result. The only consumer is `GetTodayClassrooms` (already under the refresh-or-read logic).

**log_id injection:** The log_id is per-request, so it cannot be pre-cached. It's written into the envelope header bytes before the pre-cached data field. This is ~60 bytes of allocation per request (the buffer from the pool) vs ~40KB of re-marshaling saved.

### /healthz Constant (R3)

```go
var healthzBody = []byte(`{"status":"ok"}`)

func (server *HTTPServer) Healthz(w http.ResponseWriter, _ *http.Request) {
    w.Header().Set("Content-Type", "application/json; charset=utf-8")
    w.WriteHeader(http.StatusOK)
    _, _ = w.Write(healthzBody)
}
```

Zero allocs: no json.Marshal, no map, no interface boxing.

### writeJSON Buffer Pool (R4)

```go
var jsonBufPool = sync.Pool{
    New: func() any { return new(bytes.Buffer) },
}

func writeJSON(w http.ResponseWriter, status int, v any) {
    buf := jsonBufPool.Get().(*bytes.Buffer)
    buf.Reset()
    defer jsonBufPool.Put(buf)
    if err := json.NewEncoder(buf).Encode(v); err != nil {
        w.WriteHeader(http.StatusInternalServerError)
        return
    }
    body := buf.Bytes()
    // Encode adds trailing newline; trim it to match prior behavior.
    if len(body) > 0 && body[len(body)-1] == '\n' {
        body = body[:len(body)-1]
    }
    w.Header().Set("Content-Type", "application/json; charset=utf-8")
    w.WriteHeader(status)
    _, _ = w.Write(body)
}
```

This covers /readyz, 404 fallback, and error envelopes — low-traffic paths where full pre-serialization isn't worth the complexity.

---

## Frontend Design

### Card Replacement (R5)

Ant Design's `<Card>` in this project is always:
```tsx
<Card style={{ ... }}>children</Card>
```

Replace with:
```tsx
<div className="panel">children</div>
```

CSS (shared `panel.css`):
```css
.panel {
  border-radius: 8px;
  border: 1px solid var(--border-color, #f0f0f0);
  padding: 16px;
  background: var(--bg-card, #fff);
}
```

Dark mode already uses CSS variables via antd's ConfigProvider token injection. We replicate the relevant tokens as CSS custom properties set on `:root` / `[data-theme="dark"]`.

### Tag Replacement (R6)

`<Tag color="blue">room</Tag>` → `<span className="room-tag room-tag--blue">room</span>`

```css
.room-tag {
  display: inline-block;
  padding: 0 7px;
  font-size: 12px;
  line-height: 20px;
  border-radius: 4px;
  border: 1px solid currentColor;
}
.room-tag--blue { color: #1677ff; background: #e6f4ff; border-color: #91caff; }
.room-tag--green { color: #52c41a; background: #f6ffed; border-color: #b7eb8f; }
```

### Typography Replacement (R7)

- `Typography.Text` → `<span>` (with `className="text-secondary"` for `type="secondary"`).
- `Typography.Title level={4}` → `<h4 className="heading-4">`.

Styles match antd's default token values.

---

## Compatibility & Migration

- **API contract:** Byte-identical JSON output (verified by existing handler_test.go).
- **Frontend rendering:** Visual regression checked manually (panel borders, tag colors, text sizes).
- **Build:** No new dependencies; antd remains for Button, Modal, Switch, Alert, Spin, Empty, ConfigProvider, Divider (Phase 3 removals, out of scope).
- **Tests:** All existing tests pass without modification. New benchmark test added for /api/get_data alloc count.

## Risks

- **R2 (pre-serialized cache) stale variant:** If partial_campuses list changes mid-TTL (extremely rare: only on multi-campus partial recovery during a single 5-minute window), the pre-cached stale bytes may not reflect the latest partial_campuses. Mitigation: fall back to runtime marshal when the cached partial_campuses differ from the request-time state.
- **Frontend CSS specificity:** antd's CSS-in-JS may have injected styles that override our replacements. Mitigation: test in both light and dark mode; our native elements have no antd class names so no conflict.
