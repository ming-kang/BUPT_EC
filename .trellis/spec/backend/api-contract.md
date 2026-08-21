# API Response Contract

## Routes

`router.go::Routes() http.Handler` defines the public HTTP surface. It builds
an `http.ServeMux` and wraps it in the middleware chain (outer → inner):
`gzipSkipProbes` → gzhttp wrapper → `apiLogContext` → `recovery` → mux.
`main.go` wires the result as the `http.Server` `Handler`. Handlers use plain
`func(w http.ResponseWriter, r *http.Request)` signatures and shape JSON
through `router.go::writeJSON` (`Content-Type: application/json;
charset=utf-8`, `json.Marshal`, no trailing newline).

| Route | Handler | Contract |
| --- | --- | --- |
| `GET /api/get_data` | `handler.go::GetData` | Returns today's classroom data or a safe service error. |
| `GET /healthz` | `handler.go::Healthz` | Liveness probe: `200 {"status":"ok"}`. |
| `GET /readyz` | `handler.go::Readyz` | Readiness probe; 503 when not ready. Minimal surface is `status`+`version`; the full runtime diagnostics block (credential/cache details) is opt-in via `READYZ_DIAGNOSTICS` (`config.ReadyzDiagnostics`). |
| `GET /metrics` | `handler.go::Metrics` | Loopback Prometheus exposition from an isolated registry; nil handler → 404. |
| `GET /assets/` | `router.go::immutableCache` + `http.FileServerFS` | Hashed frontend build assets with immutable caching; directory paths are 404. |
| any other path/method | `router.go::Routes` `fallback` | Unknown `/api/*` → JSON 404 `{"code":404,"msg":"not found","log_id":...}`; existing dist-root files (`favicon.ico`) → served `no-cache`; everything else → SPA fallback to `index.html`. |

Compression is owned by `klauspost/compress/gzhttp`
(`router.go::newGzipWrapper`), configured with `EnableZstd(false)` (gzip-only
behavior surface for now), an explicit `ContentTypes` allowlist
(json/html/plain/css/javascript/svg/xml/wasm — deliberately no `image/png`,
`font/woff2`, or `application/octet-stream`), and
`gzhttp.MinSize(gzhttp.DefaultMinSize)` (1024 bytes, written out as
documentation). Responses that already carry `Content-Encoding` pass through
untouched — gzhttp skips them automatically. `/healthz` and `/readyz` bypass
the wrapper entirely via the path-level `gzipSkipProbes` shim, so probe
responses are never compressed and never carry `Vary`. `/metrics` flows
through the same wrapper (the parameterless `text/plain` allowlist entry
matches `text/plain; version=0.0.4`); production constructs
`promhttp.HandlerFor` with `DisableCompression: true` as a second guard so
the body is compressed at most once. Scrapers with `Accept-Encoding: gzip`
must get valid Prometheus text after a single decompress. Public Nginx keeps
`location = /metrics` `return 404` and does not proxy the path.

> **Gotcha (gzhttp content types)**: gzhttp's *default* `ContentTypeFilter`
> does not exclude `image/png` or `font/woff2`. The explicit `ContentTypes`
> allowlist in `newGzipWrapper` is what keeps already-compressed static
> assets from being re-compressed — do not drop it when touching options.

### Static Asset Caching

| Resource | Cache-Control | ETag |
| --- | --- | --- |
| `/assets/*` (content-hashed filenames) | `public, max-age=31536000, immutable` | none — the hash in the filename is the version |
| `index.html` (incl. every SPA fallback) | `no-cache` | weak `W/"<first 8 bytes of sha256, hex>"`, computed once in `Routes()` |
| `favicon.ico` (dist root, unhashed) | `no-cache` | none |

- `index.html` is read once at assembly time and served through
  `http.ServeContent` with a zero modtime, so conditional requests go purely
  through `If-None-Match` → 304. The ETag is **weak** on purpose: gzhttp
  keeps the original ETag on compressed variants, and one strong ETag over
  two byte streams would violate RFC 7232; weak comparison still yields 304.
- `/assets/...` **directory** paths (trailing slash) always return 404 in
  `immutableCache`: listings have no content hash and must never be served
  with (or without) the immutable header.
- Missing `/assets/<name>` files are a real 404, not an `index.html`
  fallback — a wrong asset name must not receive HTML.

### HTTP Surface Behavior Facts

- `http.ServeMux` redirects unclean paths (`//healthz`, `..` segments) and
  `/assets` (no trailing slash) with **307** to the cleaned path; this is
  mux-builtin behavior and not configurable.
- Trailing-slash variants such as `/api/get_data/` are **not** redirected;
  they fall through to the fallback (JSON 404 for API paths, SPA otherwise).
- The catch-all `mux.Handle("/", fallback)` is method-independent, so the
  ServeMux can never answer 405: `POST /api/get_data` lands in the fallback
  and yields the JSON 404 envelope.
- Every response passing through the gzhttp wrapper carries
  `Vary: Accept-Encoding` — including identity, 404, and 304 responses.
- HEAD requests are never compressed.
- `apiLogContext` sets the `X-Log-Id` response header for `/api/*` requests
  only; SPA and static responses must not carry it.
- `recovery` sits innermost so a panic converts to a clean 500 (when no
  bytes were written) before reaching the gzhttp writer; the panic value and
  stack go to slog with the request `log_id`.

> **Gotcha (go:embed)**: `//go:embed all:dist` in `web/embed_enabled.go`
> requires at least one matching file. Building with `-tags embed_assets`
> while `web/dist` is empty or missing fails compilation; the tag-less
> default build (placeholder FS in `web/embed_disabled.go`) is what keeps a
> bare clone compiling.

## `/api/get_data` Response Shape

Success responses use this envelope (log_id mirrors the failure envelope so
both paths are correlatable against server logs and the `X-Log-Id` header):

```json
{
  "code": 0,
  "log_id": "20260706120000ABCDEF...",
  "data": { "date": "2026-07-06", "campuses": [] }
}
```

Failure responses use HTTP 503 and this envelope. When the failure is a
cold-start warming state — `service.ErrNoTodayCache` or
`service.ErrRefreshWaitTimeout` (the bounded cold-miss wait expired while the
background refresh kept running) — the response also carries
`Retry-After: 5`. Other 503 causes (config/auth failures) carry no
`Retry-After`:

```json
{
  "code": 503,
  "msg": "数据获取失败，请稍后重试",
  "log_id": "20260706120000ABCDEF...",
  "data": null
}
```

The message must come from `service.SafeErrorMessage`. Do not return raw JW
errors, upstream response bodies, URLs, credentials, or tokens to clients.

## `TodayClassrooms` JSON Contract

`service/model/realtime_data.go` is the source of truth for the public payload:

```text
TodayClassrooms
├── date: string
├── updated_at: RFC3339 timestamp (latest refresh-attempt completion)
├── expires_at: RFC3339 timestamp
├── stale_until: RFC3339 timestamp
├── stale: boolean
├── campuses: CampusInfo[]
├── partial_campuses?: string[]
└── error: APIError | null
```

Each campus contains:

- `id`: JW campus ID such as `01` or `04`.
- `name`: display name such as `西土城` or `沙河`.
- `buildings`: normalized building groups.
- `nodes`: class-period summaries for that campus.

Each building contains:

- `name`: raw upstream building name; selection state and table filters key on it.
- `display_name`: user-facing label normalized server-side (alias table such as
  未来学习大楼→主楼, numeric names prefixed 教). Mirrors the `RoomInfo.display_name`
  precedent in `classroom_builder.go`.

Each room contains:

- `name`: room name without the building prefix.
- `display_name`: stable building-room label such as `教学实验综合楼-N104`.
- `capacity`: parsed integer capacity. Wire guarantee: always ≥ 1 — JW CLASSROOMS
  tokens carry a positive `(N)` suffix (live-guarded by
  `TestJWRoomTokensCarryPositiveCapacitySuffix`). A 0 can only appear as parse
  degradation for a malformed suffix-less token, which also lands in the 未分组
  building; the frontend's 未知 fallback is intended for exactly that case.
- `free_nodes`: integer class-period numbers.
- `free_times`: `{node,time}` pairs corresponding to `free_nodes`.

`classroom_builder.go` is responsible for parsing JW rows such as
`教学实验综合楼-N104(229)` and merged rooms such as `未来学习大楼-202-203(60)`.
Tests in `service/realtime_data_test.go` cover normal rooms, merged rooms,
full-width parentheses, and room deduplication.

## Frontend Boundary

The frontend calls only `/api/get_data` for classroom data. Important consumers:

- `frontend/src/useTodayClassrooms.js` owns the single `useSWR` call for the
  endpoint (key `TODAY_CLASSROOMS_KEY`, `useTodayClassrooms.js:21`), validates
  the response shape inside its fetcher, and derives the UI envelope at render
  time.
- `frontend/src/components/BuildingPicker.tsx` reads campus `buildings`.
- `frontend/src/components/ClassTimePicker.jsx` reads campus `nodes`.
- `frontend/src/components/TodayClassroomTable.jsx` filters rooms by selected
  building and selected class periods.

Preserve these semantics unless the frontend is updated in the same change:

- `campuses` is an array containing both configured campuses when refresh
  succeeds.
- `nodes` is per-campus, not global.
- `free_nodes` uses integer node numbers and is suitable for intersection
  filtering: a room is available for selected periods only when all selected
  nodes appear in `free_nodes`.
- `display_name` is the stable human-readable room key shown to users.
- `stale=true` means the payload is usable but came from an expired same-day
  cache. If a refresh failed, `error` may describe the stale condition with a
  safe message.
- `partial_campuses` lists configured campus IDs that failed during the usable
  refresh. It is omitted for complete payloads; a partial payload may still be
  fresh by age and is returned with HTTP 200.
- `updated_at` is the refresh attempt completion time. If a partial refresh
  reuses prior same-day campus data, do not present it as every campus's data
  freshness timestamp.

The hook hands components a single normalized envelope, not SWR's raw state:

```text
resp
├── code: 0 for a usable snapshot, otherwise the HTTP status or business code
├── msg:  safe message (never a raw transport/JW error)
├── data: TodayClassrooms | null
└── logId: server correlation ID, "" when unknown — error envelopes only
```

`logId` is a frontend-side field (it does not exist in the backend success
envelope): the fetch boundary reads `payload.log_id`, falling back to the
`X-Log-Id` response header, and `components/GlobalEmpty.jsx:19-23` renders it
as a dim `log_id: …` line under the error description so a user can quote it
in a bug report. Do not surface it on code-0 stale/partial envelopes — the
warning Alert has no correlation-ID story and the success body carries no
`log_id`.

When changing this contract, update backend tests, frontend validation, affected
components, user docs, and `CHANGELOG.md` if users can observe the change.

## Scenario: Frontend Snapshot Validity and Reload Backoff

### 1. Scope / Trigger

Apply this contract whenever frontend fetch merging, classroom cache timestamps,
reload scheduling, SWR configuration, or partial-campus warnings change. It
prevents a browser tab from retaining yesterday's classroom data or polling
faster than the backend can refresh.

The data layer is `useSWR` plus pure glue: SWR owns transport, cache, dedupe,
focus/reconnect revalidation and the retry chain; the repo owns the schedule
math (`reloadSchedule.js`), the validity predicate
(`classroomDataValidity.js`), and the render-time derivation that turns SWR's
`data`/`error` tracks back into the single `resp` envelope the UI consumes.

### 2. Signatures

```js
// frontend/src/classroomDataValidity.js
isUsableBusinessDaySnapshot(data, nowMs = Date.now())            // :7

// frontend/src/reloadSchedule.js  (unchanged by the SWR migration)
failureRetryDelay(failureCount)                                  // :13
withJitter(delayMs, random = Math.random)                        // :52
nextReloadDelay(data, { failureCount = 0, nowMs = Date.now(), random = Math.random } = {})  // :81

// frontend/src/todayClassroomsResponse.js
normalizeResponse(payload) -> { ok: true, resp } | { ok: false, reason }   // :49

// frontend/src/apiError.js
new ApiError(message, { status, code, logId })                   // :6

// frontend/src/useTodayClassrooms.js — pure glue, all separately testable
hasUsableClassroomData(resp, nowMs = Date.now())                 // :32
shouldFullPageSpin(isBackground, hasUsableData)                  // :43
mergeFetchResult(prev, next, nowMs = Date.now())                 // :55
fetchTodayClassrooms(url)                                        // :126  SWR fetcher
pollingInterval(latest)                                          // :184  refreshInterval
retryDelayFor(retryCount, latestData, options = {})              // :195
retryOnError(error, key, config, revalidate, opts)               // :208  onErrorRetry

// default export, return shape unchanged from the pre-SWR hook
useTodayClassrooms() -> { resp, spinning, reloading, isError, retry }   // :225
```

`nextFailureCount` no longer exists: SWR's 1-based `retryCount` is the failure
counter, and success/non-retry revalidations reset it internally.

### 3. Contracts

- A displayable snapshot has an array `campuses`, a `date` equal to the current
  Asia/Shanghai date, and a parseable future `stale_until`.
- Throwing is the single error channel. `fetchTodayClassrooms` throws `ApiError`
  for every outcome that must not replace the cached snapshot: transport
  failure, 40s `AbortSignal.timeout`, non-2xx status, malformed payload
  (`normalizeResponse` returning `{ ok: false }`), a non-zero service envelope
  over HTTP 2xx, and a success envelope whose cache metadata is not displayable.
  Anything it returns is a usable snapshot. `normalizeResponse` itself never
  throws — it returns a discriminated result, so parsing is not control flow.
- `ApiError` carries the real HTTP `status`, the business envelope `code` and
  the server `logId`. The derived envelope's `code` is `status ?? businessCode
  ?? 500`, never a blanket 500 (`useTodayClassrooms.js:103`).
- `logId` comes from `payload.log_id` with the `X-Log-Id` response header as
  fallback (`useTodayClassrooms.js:141`). It rides on hard-error envelopes only
  (`resp.logId`); code-0 stale merges never surface it.
- `mergeFetchResult` keeps prior data after a client failure only while that
  snapshot remains displayable; otherwise it returns `data: null`. It runs at
  **render time** over SWR's separate `data`/`error` tracks (a throwing fetcher
  leaves `data` untouched), not at setState time.
- Consecutive client failures retry after 10s, 20s, 30s, then 60s maximum. A
  valid HTTP 200 classroom payload resets the count. The counter is SWR's
  `retryCount`, which starts at 1 and maps 1:1 onto the ladder — pass it
  straight to `retryDelayFor`, no offset arithmetic.
- Polling and the retry ladder are never both armed: SWR suspends
  `refreshInterval` while the cache holds an error, and `onErrorRetry` owns
  scheduling until a revalidation succeeds.
- A partial payload base-polls at 30s; an ordinary stale payload base-polls at
  15s; a fresh payload waits for `expires_at` (1s floor).
- Scheduling pipeline: base delay → **one** unit random sample → positive-only
  jitter `base + sample * min(base*10%, 5s)` → absolute clamp to
  `max(0, stale_until - now)` when the snapshot is still displayable. Business
  deadline wins when it is earlier than the rate-limit floor. Jitter must not
  shorten documented minimum intervals.
- Invalid / throwing / non-finite random sources fall back to sample 0.5 and
  never yield NaN delays.
- Background retries never enable the full-page spinner. When a render observes
  a code-0 snapshot that has crossed midnight or `stale_until`, it clears the
  campuses (`EXPIRED_SNAPSHOT_MESSAGE` envelope, `useTodayClassrooms.js:285`)
  while the clamped reload is in flight.
- Hidden tabs issue zero classroom requests. Three separate mechanisms carry
  that, and all three must stay in place:
  1. `refreshWhenHidden` stays at its `false` default, so the polling chain
     keeps re-arming its timer but skips the fetch while hidden.
  2. SWR only chains a *new* retry when `isActive()` (visible and online), so
     the ladder pauses on hide. This precondition is gated on
     `revalidateOnFocus`/`revalidateOnReconnect` being enabled — turning
     `revalidateOnFocus` off makes it vacuously true and lets hidden tabs
     keep retrying. Keep `revalidateOnFocus: true`.
  3. An **already-armed** retry timer has no visibility gate inside SWR, so
     `retryOnError`'s own `setTimeout` callback re-checks
     `document.visibilityState` and abandons the attempt when hidden
     (`useTodayClassrooms.js:214-222`).
- Becoming visible after `stale_until` revalidates promptly instead of keeping
  yesterday's filters for a normal poll interval. `revalidateOnFocus` (which
  listens to both `visibilitychange` and `window.focus`) carries this, and it
  also re-issues the request that a hidden tab abandoned above.
  `focusThrottleInterval` is `FOCUS_THROTTLE_MS = 15_000`
  (`useTodayClassrooms.js:27`), aligned with `STALE_POLL_MS`: the 5s default
  would let multi-tab switching exceed the Nginx 30 req/min budget, and the
  throttle is also what makes repeated visible events fire a single reload.
- **Accepted semantic drift from the pre-SWR scheduler**: a focus-triggered
  revalidation that fails resets the backoff ladder to 10s, because it reaches
  the fetcher without a `retryCount`. The old hand-written layer only cleared
  `failureCount` on success. This is one notch more aggressive by design and
  stays inside the request budget — the 15s focus throttle caps how often a
  reset can happen, and the ladder still caps at 60s and still clamps to
  `stale_until`.
- Unmounting does not abort an in-flight request; SWR only ignores the late
  result. Acceptable for a single-page app that unmounts only on navigation
  away, but it means "unmount cancels the request" is not a testable guarantee.
- `partial_campuses` is optional. When present, warnings resolve IDs through the
  payload's campus names and fall back to the ID when no name is available.

### 4. Validation & Error Matrix

| Condition | Frontend result |
| --- | --- |
| same-day snapshot, future `stale_until`, fetch failure | preserve data, set `stale` and `client_refresh_failed` |
| previous-day or expired snapshot, fetch failure | hard error envelope with `data: null` |
| missing/invalid `date`, `stale_until`, or `campuses` | fail closed without throwing; `normalizeResponse` returns `{ ok: false, reason }` and the fetcher converts it to one `ApiError` |
| non-2xx status | envelope `code` is the real HTTP status, not 500 |
| non-zero envelope over HTTP 2xx | envelope `code` is the business code |
| hard error with `retryCount` 1/2/3/4+ | retry after 10s/20s/30s/60s |
| failing revalidation triggered by focus/manual retry (no `retryCount`) | ladder restarts at 10s (accepted drift, throttled to once per 15s) |
| hard error carrying `log_id` body or `X-Log-Id` header | `resp.logId` set; `GlobalEmpty` renders the `log_id` line |
| valid partial payload | reset client-failure count, base poll 30s + positive jitter |
| valid stale payload | base poll 15s + positive jitter |
| fresh expiry later than `stale_until` | wake at `stale_until` (post-jitter clamp) |
| sample=1 near hard deadline | final delay ≤ remaining `stale_until` |
| no data yet (pre-first-fetch `refreshInterval` call) | interval falls back to `MIN_FRESH_DELAY_MS`, never 0/null |
| tab hidden when an armed retry fires | request abandoned; `revalidateOnFocus` reissues it on return |

### 5. Good/Base/Bad Cases

- Good: a tab opened before midnight wakes at `stale_until`; if the reload
  fails, yesterday's campus filters and table disappear and automatic retry
  continues.
- Base: a fresh complete payload waits until `expires_at` and resets prior
  transport failure backoff.
- Bad: testing only `campuses` and retaining any prior `code: 0` payload after
  midnight; applying symmetric jitter that schedules past `stale_until`; or
  letting `refreshInterval` return `nextReloadDelay(...)` unguarded, which
  silently ends all polling on the first call.

### 6. Tests Required

- `classroomDataValidity.test.js`: same-day, cross-day, expired, and malformed
  required fields.
- `reloadSchedule.test.js`: 10/20/30/60 cap, hard-empty retry, 30s partial /
  15s stale bases, single random sample, positive-only jitter bounds, invalid
  RNG fallbacks, and post-jitter `stale_until` clamp (including sample=1).
- `todayClassroomsResponse.test.js`: `normalizeResponse` returning
  `{ ok: false, reason }` for every malformed shape (assert the discriminated
  result, never `toThrow`), safe normalization of non-zero service envelopes,
  the campus-name warning, and missing-field compatibility.
- `apiError.test.js`: `status` / `code` / `logId` stay structured on the error
  object, and `logId` defaults to `""` when no details are passed.
- `useTodayClassrooms.test.js` (pure glue, node env): `hasUsableClassroomData`
  accept/reject matrix, `shouldFullPageSpin` quadrants, `pollingInterval`
  reading the snapshot out of the cached envelope and never returning a falsy
  interval, `retryDelayFor` mapping the 1-based `retryCount` onto the ladder and
  clamping to a displayable `stale_until`, and the six `mergeFetchResult`
  scenarios (preserve, non-ok envelope, hard-empty, cross-day/expired clear,
  fail-closed metadata, successful replace).
- `useTodayClassrooms.lifecycle.test.jsx` (jsdom, 12+ cases): initial load; late
  responses after unmount being harmless (no abort); manual retry issuing a
  second request and clearing the full-page error; a failed background reload
  keeping the last good data; timer cleanup on unmount; the 40s timeout safe
  message; real HTTP status plus body `log_id` in the error envelope;
  `X-Log-Id` header fallback; **background polling still alive after the first
  load** (the falsy-chain-death regression); no background reload while hidden;
  an armed retry abandoning itself while hidden and recovering on focus; the
  ladder restarting at 10s after an intervening success; and hide → past
  deadline → visible → cleared snapshot with a single prompt reload.
- `components/GlobalEmpty.test.jsx`: the `log_id` line renders for error
  envelopes carrying one and is omitted when `logId` is empty or missing.
- Component tests for the other consumers of this contract:
  `components/TodayClassroomTable.test.jsx` (native `thead` with three
  `scope="col"` headers, `free_nodes` intersection filtering, capacity
  rendering, and the room modal following background refreshes without
  closing), `components/ClassTimePicker.test.jsx`,
  `components/BuildingPicker.test.jsx`,
  `components/CampusButtonGroup.test.jsx` (derived selection and
  `aria-pressed`), and `components/ErrorBoundary.test.jsx`.

> **Gotcha (SWR test isolation)**: SWR's cache is a module global. Every test
> that mounts the hook must wrap the probe in
> `<SWRConfig value={{ provider: () => new Map() }}>`, or `data`, `error` and
> dedupe state leak across cases. Keep `dedupingInterval` at the production
> default: the always-alive polling chain fires a bootstrap tick roughly 1s
> after mount (the pre-first-data interval) and production deduping absorbs it,
> while `0` turns that tick into a real request and makes fetch counts flaky
> (bound `mutate()` bypasses deduping, so manual retry still works). Fake
> timers cannot advance jsdom's internal `AbortSignal.timeout` clock, so the
> timeout case rebuilds it on the patched global `setTimeout`
> (`useTodayClassrooms.lifecycle.test.jsx:103-114`).

### 7. Wrong vs Correct

#### Wrong

```js
if (prev?.code === 0 && Array.isArray(prev.data?.campuses)) return prev;
```

#### Correct

```js
const canPreserve = isUsableBusinessDaySnapshot(prev?.data, nowMs);
return canPreserve
  ? { code: 0, data: { ...prev.data, stale: true, error: clientError } }
  : { code: 500, msg: message, data: null };
```

#### Wrong

```js
// Chain death: nextReloadDelay(undefined) is null before the first payload,
// and SWR ends the polling chain for good on a falsy interval.
refreshInterval: (latest) => nextReloadDelay(latest?.data, { failureCount: 0 }),
```

#### Correct

```js
// Module-level identity + never-falsy floor (useTodayClassrooms.js:184).
export function pollingInterval(latest) {
  const delay = nextReloadDelay(latest?.data, { failureCount: 0 });
  return Math.max(1, delay ?? MIN_FRESH_DELAY_MS);
}
```

> **Gotcha (SWR polling chain)**: the polling effect depends only on
> `[refreshInterval, refreshWhenHidden, refreshWhenOffline, key]` and re-arms
> itself through `execute → .then(next)`. Two consequences: (1) any falsy
> return value (`0`, `null`, `undefined`) stops polling **permanently**, not
> just for one tick — hence the `Math.max(1, … ?? MIN_FRESH_DELAY_MS)` wrapper
> and the dedicated "polling still alive after the first load" test; (2) the
> function must keep a **stable module-level identity** — an inline arrow gives
> the effect a new dependency on every render, which restarts the timer and
> lets unrelated re-renders indefinitely postpone a 15s/30s poll.
>
> **Gotcha (keepPreviousData)**: it only returns the laggy value while the
> *current key* has no cache entry, i.e. across key switches. This app's key is
> the constant `"/api/get_data"`, so the option is a no-op here — do not add it
> believing it implements "keep the last snapshot on failure". That behavior
> comes from SWR storing `data` and `error` on separate tracks: a throwing
> fetcher writes `error` and leaves `data` intact, and `mergeFetchResult`
> derives the stale envelope from the pair at render time.

## Health and Readiness

`/healthz` must stay cheap and independent of JW credentials or cache state.
Use it for liveness only.

`/readyz` may return 503 when credentials are missing or no usable same-day
cache exists. Its body includes:

- `status`: HTTP status text;
- `version`: build version string (`main.version`, default `"dev"`; injected at
  release time via `go build -ldflags "-X main.version=<value>"` — release tag
  for tag builds, `nightly-<short-sha>` for nightly builds);
- `jw_credentials_configured`: result of the injected immutable
  `config.RuntimeConfig.HasJWCredentials()` predicate;
- `runtime`: `service.RuntimeStatus` diagnostics.

> **Gotcha (version injection)**: `-X main.version=...` only binds in a real
> `go build`. In `go test` the test binary's main package is the synthesized
> test main, so `go test -ldflags "-X main.version=..."` silently no-ops (the
> package under test is addressed as `BUPT_EC`, not `main`). Also, Go 1.19+
> deliberately excludes `-ldflags` values from `go version -m` build info —
> audit the injected value at runtime via `/readyz` or the startup log, never
> via `go version -m`.

Runtime cache diagnostics keep age and completeness separate:

- `cache_fresh` / `cache_stale`: age state;
- `cache_partial`: whether the usable payload is incomplete;
- `partial_campuses`: affected campus IDs;
- `last_refresh_warning`: sanitized partial outcome warning;
- `last_refresh_error`: sanitized latest total failure.

Do not put secrets or raw upstream payloads in readiness responses.

## Anti-Patterns

- Returning HTML for unknown `/api/*` paths; API clients expect JSON 404.
- Changing public JSON tags without updating the frontend consumer.
- Treating `free_times` as the source of filtering truth when the frontend uses
  `free_nodes` for period intersection.
- Compressing probe endpoints in ways that complicate health checks.
