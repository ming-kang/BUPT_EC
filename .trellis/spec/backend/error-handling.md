# Error Handling

## Overview

Backend errors are classified in `service/jw_error.go`, logged internally with
context, and converted to safe client messages at the HTTP boundary. The public
API must never expose raw JW errors, credentials, tokens, upstream response
bodies, or internal stack details.

Reference files:

- `service/jw_error.go`: JW error kinds, classification, and safe messages.
- `service/jw_client.go`: upstream HTTP and business-code error creation.
- `service/realtime_data.go`: auth retry, refresh failure recording, stale API
  error metadata.
- `service/refresh_coordinator.go`: refresh backoff and last-error retention.
- `handler.go`: client-facing JSON response envelopes.

## Error Types

Use `newJWError(kind, op, err, format, args...)` for JW-related failures. The
local `jwErrorKind` values are the stable internal taxonomy:

| Kind | Meaning | Safe client message category |
| --- | --- | --- |
| `jw_config_error` | missing credentials or invalid server config/API URL | service configuration incomplete |
| `jw_auth_failed` | token rejected, expired, or unauthorized during query | JW login/auth failed |
| `jw_login_failed` | login endpoint failed or returned no token | JW login/auth failed |
| `jw_query_failed` | classroom query failed | JW query failed |
| `jw_bad_response` | upstream response could not be parsed or had wrong shape | JW query failed |
| `jw_unavailable` | timeout, cancellation, or unclassified upstream failure | generic data failure |

`classifyError` uses `errors.As`, so it still finds a `*jwError` inside joined
errors. Preserve this behavior when wrapping errors. `TestClassifyErrorHandlesJoinedJWError`
in `service/realtime_data_test.go` guards it.

## Propagation Pattern

Keep detailed errors inside the service layer and safe messages at the handler
layer:

1. Low-level JW/client code returns a classified `newJWError` with an operation
   name such as `"jw login"`, `"jw query"`, or `"serverconfig"`.
2. Service orchestration may join related errors with `errors.Join`, for
   example auth failure plus failed retry in `queryCampus`.
3. Refresh code records failures in runtime status and logs them with
   `slog.WarnContext`.
4. `handler.go::GetData` converts the error to `service.SafeErrorMessage(err)`
   and includes the request `log_id` for operator correlation.

Do not turn service errors into HTTP responses inside `service/`. Do not import
Gin into `service/`.

## Auth Retry and Backoff

`service/realtime_data.go::queryCampus` has the only token-retry flow:

- call `TokenManager.EnsureToken(ctx, false)`;
- query the campus;
- if the error is not `jwErrorAuth`, return it;
- if it is `jwErrorAuth`, call
  `RefreshAfterAuthFailure(ctx, failedToken)`; that method rechecks token state
  inside singleflight, logs in only when still necessary, and returns the token
  for one retry.

`service/refresh_coordinator.go` separately prevents refresh storms. A failed
refresh stores `lastRefreshErr` and sets `nextRefreshAllowed` for the 30-second
stale refresh backoff. Do not add unbounded retry loops around JW endpoints.

## API Error Responses

`handler.go::GetData` returns this shape on service failure:

```json
{
  "code": 503,
  "msg": "教务系统查询失败，请稍后重试",
  "log_id": "20260706120000ABCDEF...",
  "data": null
}
```

The `msg` value must come from `SafeErrorMessage`. The `log_id` comes from
`logs.GetLogIDFromContext(ctx)` and lets operators find matching structured log
records.

Stale success responses are different: `GetTodayClassrooms` may return usable
same-day data with `stale=true` and `data.error` set to an `APIError` from
`staleAPIError`. This is still an HTTP 200 success payload because the data is
usable.

Error precedence follows the latest known refresh outcome:

- while a refresh is still in flight, return the cache's existing warning;
- after a total refresh failure, its stale failure warning overrides an older
  partial-campus warning;
- during total-failure backoff, keep returning the latest total failure;
- a new partial result replaces the old cached payload and warning.

Do not use a generic “prefer cached error” helper: cached warnings are older
facts and must not mask a completed total failure.

## Upstream Response Handling

JW responses often use business codes inside JSON bodies. Keep HTTP status and
business-code classification together in `service/jw_client.go` helpers:

- classify business code `401`/`403` as auth failure even if HTTP status is 500;
- trim remote messages with `safeRemoteMessage` before storing them in internal
  errors;
- classify malformed response bodies as `jw_bad_response` rather than returning
  raw JSON parser errors to clients.

Tests such as `TestParseJWQueryResponseClassifiesBusinessAuthCode` and
`TestClassifyJWHTTPErrorUsesBusinessAuthCode` document these edge cases.

## Common Mistakes

- Returning `err.Error()` in JSON responses. Use `SafeErrorMessage`.
- Logging or returning JW tokens, passwords, encrypted password payloads, or raw
  upstream bodies.
- Treating every upstream non-200 response as a generic query failure; business
  auth codes must clear and refresh the token.
- Clearing whatever token is currently cached after an old request fails, or
  clearing outside the singleflight recovery closure. Pass the rejected value to
  `RefreshAfterAuthFailure` so delayed requests reuse a newer token.
- Hiding stale refresh failures from `TodayClassrooms.error` or runtime status.
- Preferring an older partial warning over a newer total refresh failure.
