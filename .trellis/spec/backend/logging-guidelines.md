# Logging Guidelines

## Overview

The backend uses Go `log/slog` with a JSON handler configured by `logs.Init`.
Production-style startup writes logs to both stdout and `run_log/ec.log` through
lumberjack rotation; test/helper startup writes to stdout only.

Reference files:

- `logs/log_util.go`: logger setup, `log_id` generation, Gin context helpers.
- `logs/logger.go`: slog handler wrapper that injects `log_id` into records.
- `router.go`: applies `logs.SetNewContextForGinContext` to `/api` routes.
- `handler.go`, `service/realtime_data.go`, `service/token_manager.go`: request
  and refresh/login logging examples.
- `docs/operations.md`: operator-facing log behavior.

## Logger Initialization

`logs.Init(true)` is called from application initialization and creates:

- a JSON `slog` handler at info level;
- stdout output for systemd/journal collection;
- `run_log/ec.log` with max size 10 MB, 5 backups, 30 days, compressed;
- optional caller source fields when `LOG_CALLER=1` or `LOG_CALLER=true`.

Tests and non-main setup use `logs.Init(false)` so they do not create rotating
log files.

Do not create independent loggers in handlers or service code. Use the default
`slog` logger configured by `logs.Init`.

## Request `log_id` Propagation

Every `/api/*` request must pass through `logs.SetNewContextForGinContext` in
`router.go`. That middleware:

- creates a context containing `logs.LogIDKey`;
- stores the context in Gin under `"ctx"`;
- sets the `LogID` response header;
- lets `logs.GetContextFromGinContext` retrieve the same context in handlers.

The custom slog handler adds the `log_id` attribute to records when the context
contains one. API error responses also include `log_id` so operators can match a
client-visible failure to structured logs.

When adding API handlers, always log with the request context:

```go
ctx := logs.GetContextFromGinContext(c)
slog.InfoContext(ctx, "GetData")
```

Avoid `slog.Info`, `log.Printf`, or `fmt.Println` in request paths because they
lose correlation data.

## Log Levels and Events

Current local pattern:

- `InfoContext`: successful request entry, JW login success, classroom refresh
  success.
- `WarnContext`: JW login failure and classroom refresh failure.

There is no debug-level logging in normal runtime because the configured level
is info. If a future task adds debug logs, make them safe and useful when
enabled, but do not rely on them for production diagnostics.

Useful log attributes include elapsed durations, stable operation names, and
classified error values. Prefer structured attributes such as `"elapsed"` and
`"err"` rather than concatenated message strings.

## What to Log

Log operational state transitions that help diagnose outages without exposing
private data:

- API handler entry for `/api/get_data`.
- JW login success or failure with elapsed time.
- Classroom refresh success or failure with elapsed time.
- Errors already classified by service code.
- Readiness/runtime status through `/readyz`, not by dumping status on every
  request.

For new long-running or external operations, log start/end or failure at the
layer that owns the operation. Include the request context when one exists.

## What Not to Log

Never log:

- `JW_USERNAME`, `JW_PASSWORD`, `JW_TOKEN`, or the encrypted login password.
- Raw upstream response bodies that may include tokens or account details.
- Unvalidated API URLs from `serverconfig.json` before `validateJWAPIURL` has
  accepted them.
- Full `.env` contents or `/etc/bupt-ec/bupt-ec.env` contents.
- User-specific account details from the JW system.

`service.SafeErrorMessage` is for API clients, not logs. Logs may include the
classified internal error, but keep secrets out of the error text.

## Deployment Notes

On installed servers, the working directory is `/opt/bupt-ec`, so rotating logs
go to `/opt/bupt-ec/run_log/ec.log`. The installer gives the service user write
access only to that log directory. If logging paths or systemd hardening change,
update `scripts/install.sh` and `docs/operations.md` together.

## Anti-Patterns

- Adding route handlers outside `/api` that need request correlation but do not
  use `logs.SetNewContextForGinContext`.
- Logging with a background context when a request context is available.
- Adding plaintext logs of credentials to make JW debugging easier.
- Relying only on file logs in environments where stdout is the primary log
  stream.
