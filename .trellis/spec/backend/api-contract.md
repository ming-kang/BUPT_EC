# API Response Contract

## Routes

`router.go` defines the public HTTP surface:

| Route | Handler | Contract |
| --- | --- | --- |
| `GET /api/get_data` | `handler.go::GetData` | Returns today's classroom data or a safe service error. |
| `GET /healthz` | `handler.go::Healthz` | Liveness probe: `200 {"status":"ok"}`. |
| `GET /readyz` | `handler.go::Readyz` | Readiness probe with credential/cache/runtime status; 503 when not ready. |
| non-API paths | `router.go::NoRoute` | Serve embedded `frontend/dist/index.html` for SPA routing. |
| unknown `/api/*` paths | `router.go::NoRoute` | JSON 404: `{"code":404,"msg":"not found"}`. |

`router.go` also applies gzip to normal responses when the client accepts gzip,
but intentionally skips `/healthz` and `/readyz` for simple probes.

## `/api/get_data` Response Shape

Success responses use this envelope:

```json
{
  "code": 0,
  "data": { "date": "2026-07-06", "campuses": [] }
}
```

Failure responses use HTTP 503 and this envelope:

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

Each room contains:

- `name`: room name without the building prefix.
- `display_name`: stable building-room label such as `教学实验综合楼-N104`.
- `capacity`: parsed integer capacity.
- `free_nodes`: integer class-period numbers.
- `free_times`: `{node,time}` pairs corresponding to `free_nodes`.

`classroom_builder.go` is responsible for parsing JW rows such as
`教学实验综合楼-N104(229)` and merged rooms such as `未来学习大楼-202-203(60)`.
Tests in `service/realtime_data_test.go` cover normal rooms, merged rooms,
full-width parentheses, and room deduplication.

## Frontend Boundary

The frontend calls only `/api/get_data` for classroom data. Important consumers:

- `frontend/src/useTodayClassrooms.js` fetches and validates the response shape.
- `frontend/src/components/BuildingPicker.jsx` reads campus `buildings`.
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

When changing this contract, update backend tests, frontend validation, affected
components, user docs, and `CHANGELOG.md` if users can observe the change.

## Scenario: Frontend Snapshot Validity and Reload Backoff

### 1. Scope / Trigger

Apply this contract whenever frontend fetch merging, classroom cache timestamps,
reload scheduling, or partial-campus warnings change. It prevents a browser tab
from retaining yesterday's classroom data or polling faster than the backend can
refresh.

### 2. Signatures

```js
isUsableBusinessDaySnapshot(data, nowMs = Date.now())
mergeFetchResult(prev, next, nowMs = Date.now())
failureRetryDelay(failureCount)
nextReloadDelay(data, { failureCount = 0, nowMs = Date.now() } = {})
```

### 3. Contracts

- A displayable snapshot has an array `campuses`, a `date` equal to the current
  Asia/Shanghai date, and a parseable future `stale_until`.
- `mergeFetchResult` keeps prior data after a client failure only while that
  snapshot remains displayable; otherwise it returns `data: null`.
- Consecutive client failures retry after 5s, 10s, 20s, then 30s maximum. A
  valid HTTP 200 classroom payload resets the count.
- A partial payload polls at 30s; an ordinary stale payload may poll at 5s; a
  fresh payload waits for `expires_at`.
- Every scheduled delay is capped by `stale_until`, even when `expires_at` or
  failure backoff would cross Shanghai midnight.
- Background retries never enable the full-page spinner. If the timer wakes at
  an invalid boundary, clear the campuses before starting the reload.
- `partial_campuses` is optional. When present, warnings resolve IDs through the
  payload's campus names and fall back to the ID when no name is available.

### 4. Validation & Error Matrix

| Condition | Frontend result |
| --- | --- |
| same-day snapshot, future `stale_until`, fetch failure | preserve data, set `stale` and `client_refresh_failed` |
| previous-day or expired snapshot, fetch failure | hard error envelope with `data: null` |
| missing/invalid `date`, `stale_until`, or `campuses` | fail closed without throwing |
| hard error with failure count 1/2/3/4+ | retry after 5s/10s/20s/30s |
| valid partial payload | reset client-failure count, poll after 30s |
| valid stale payload | poll after at most 5s |
| fresh expiry later than `stale_until` | wake at `stale_until` |

### 5. Good/Base/Bad Cases

- Good: a tab opened before midnight wakes at `stale_until`; if the reload
  fails, yesterday's campus filters and table disappear and automatic retry
  continues.
- Base: a fresh complete payload waits until `expires_at` and resets prior
  transport failure backoff.
- Bad: testing only `campuses` and retaining any prior `code: 0` payload after
  midnight.

### 6. Tests Required

- `classroomDataValidity.test.js`: same-day, cross-day, expired, and malformed
  required fields.
- `useTodayClassrooms.test.js`: preserve valid prior data, hard-empty invalid
  prior data, and reset/increment consecutive failure count.
- `reloadSchedule.test.js`: 5/10/20/30 cap, hard-empty retry, 30s partial poll,
  5s stale poll, and `stale_until` earlier than expiry/backoff.
- `todayClassroomsResponse.test.js`: campus-name warning and missing-field
  compatibility.

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

## Health and Readiness

`/healthz` must stay cheap and independent of JW credentials or cache state.
Use it for liveness only.

`/readyz` may return 503 when credentials are missing or no usable same-day
cache exists. Its body includes:

- `status`: HTTP status text;
- `jw_credentials_configured`: result of the injected immutable
  `config.RuntimeConfig.HasJWCredentials()` predicate;
- `runtime`: `service.RuntimeStatus` diagnostics.

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
