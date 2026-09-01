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

Strategy: cache the `"data":{ ... }` JSON string at refresh time inside the
same immutable entry as the model pointer. One atomic store publishes the
complete generation, so a reader cannot validate model metadata from one
refresh and return JSON from another.

```go
// service/classroom_service.go
type todayCacheEntry struct {
    today     *model.TodayClassrooms
    freshJSON string // immutable pre-marshaled TodayClassrooms representation
}

// publishTodayCache marshals first, then performs exactly one Store:
s.todayCache.Store(&todayCacheEntry{today: today, freshJSON: freshJSON})
```

The handler assembles the per-request envelope around `freshJSON` with a
pooled buffer. The exported service boundary returns a string rather than a
mutable byte slice, so callers cannot corrupt the stored generation. It uses the direct path only after loading one entry and
checking that that entry is same-day, fresh, complete (`partial_campuses` is
empty), error-free, and has bytes. Stale, partial, marshal-failure, and
no-cache responses retain the typed runtime serialization path.

**Variant handling:**
- **Fresh (code=0, stale=false, no error):** Use `freshJSON` from the loaded entry.
- **Stale or partial:** `classroomResponse()` copies and decorates the model; serialize at request time to preserve current warnings and fallback semantics.
- **Error (no cache, code≠0):** Small envelope with `"data":null` — typed struct marshal with buffer pool (R4).

**log_id injection:** The log_id is per-request, so it cannot be pre-cached.
It is written into the envelope header bytes before the pre-cached data field.

### /healthz Constant (R3)

```go
var (
    healthzBody        = []byte(`{"status":"ok"}`)
    healthzContentType = []string{"application/json; charset=utf-8"}
)

func (server *HTTPServer) Healthz(w http.ResponseWriter, _ *http.Request) {
    w.Header()["Content-Type"] = healthzContentType
    w.WriteHeader(http.StatusOK)
    _, _ = w.Write(healthzBody)
}
```

The body and header value are reused immutable slices: no `json.Marshal`, map
payload, interface boxing, or per-call header-value slice. The isolated handler
benchmark locks this at zero allocations; end-to-end `net/http` response-writer
allocation is outside the handler contract.

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
- **Tests:** Existing behavior tests remain green. Focused cache-generation, eligibility, fast/slow parity, zero-allocation health, and benchmark coverage were added.

## Risks

- **R2 (pre-serialized cache) eligibility drift:** The stored string is fresh-shaped, but only complete, error-free, unexpired same-day entries may use it. Mitigation: `GetCachedDataJSON` performs every eligibility check against the same loaded entry; stale, partial, and error responses always use runtime serialization.
- **Frontend CSS specificity:** antd's CSS-in-JS may have injected styles that override our replacements. Mitigation: test in both light and dark mode; our native elements have no antd class names so no conflict.
