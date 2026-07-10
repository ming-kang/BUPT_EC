# Operations

Day-to-day operation of a deployed BUPT_EC server: service management, health checks, logs, caching behavior, and troubleshooting.

## Service management

```bash
sudo systemctl status bupt-ec
sudo systemctl restart bupt-ec
sudo systemctl stop bupt-ec
sudo journalctl -u bupt-ec -f          # follow logs
sudo journalctl -u bupt-ec -n 200      # last 200 lines
```

On shutdown the server drains gracefully: it stops accepting connections, finishes in-flight requests, and waits (within a 10-second budget) for any background classroom refresh to complete.

## Health endpoints

Both endpoints bypass gzip and are safe for load-balancer probes.

### `GET /healthz` — liveness

Returns `200 {"status":"ok"}` whenever the process is up.

### `GET /readyz` — readiness

Returns `200` when JW credentials are configured **and** a usable same-day cache exists; `503` otherwise. The body always includes diagnostics:

```json
{
  "status": "OK",
  "jw_credentials_configured": true,
  "runtime": {
    "last_login_success_at": "2026-07-03T08:15:04+08:00",
    "last_login_error": "",
    "last_refresh_success_at": "2026-07-03T08:15:05+08:00",
    "last_refresh_warning": "",
    "last_refresh_error": "",
    "cache_available": true,
    "cache_fresh": true,
    "cache_stale": false,
    "cache_partial": false,
    "cache_date": "2026-07-03"
  }
}
```

- `cache_fresh`: within the ~5 minute fresh TTL.
- `cache_stale`: cache is still usable for the business day but **past** the fresh TTL (not simply “same calendar day”). Fresh cache has `cache_stale: false`.
- `cache_partial`: at least one configured campus used prior same-day data or an empty skeleton during the latest usable refresh. `partial_campuses` lists the affected campus IDs when present.
- Business day boundaries use **Asia/Shanghai**.

`last_refresh_warning` describes a usable partial outcome, while
`last_refresh_error` describes the latest total refresh failure. A partial cache
can remain ready; use `cache_partial` and `partial_campuses` to identify its
scope. A `503` right after a restart is normal until the warmup refresh finishes.

## Logging

Logs are structured JSON via Go's `log/slog`, written to both stdout (visible in `journalctl`) and `run_log/ec.log` relative to the working directory — `/opt/bupt-ec/run_log/ec.log` on an installed server.

File rotation (via lumberjack): 10 MB per file, 5 backups, 30 days retention, rotated files compressed.

Example record:

```json
{"time":"2026-07-03T08:15:05+08:00","level":"INFO","msg":"classroom refresh succeeded","elapsed":"1.2s","log_id":"20260703081504A1B2C3D4E5F6A7B8C9D2"}
```

Partial refreshes use warning level and include `failed_campuses` plus the
classified internal errors. These diagnostics never include JW credentials or
tokens.

Set `LOG_CALLER=1` in the environment file to add source file/line to each record (useful when debugging, off by default).

### Tracing a request with `log_id`

Every `/api/*` request gets a `log_id` that appears:

- in every log record produced while handling that request,
- in the `LogID` response header,
- in the JSON body of error responses from `/api/get_data`.

When a user reports an error, ask for the `log_id` from the error response and search the logs:

```bash
sudo grep '20260703081504A1B2C3D4E5F6A7B8C9D2' /opt/bupt-ec/run_log/ec.log
# or
sudo journalctl -u bupt-ec | grep '20260703081504A1B2C3D4E5F6A7B8C9D2'
```

## Caching behavior

The backend keeps a single same-day in-memory cache of classroom data (business day = **Asia/Shanghai**):

- **Fresh TTL**: about 5 minutes. Fully successful fresh cache (`error` null) is served directly with no JW call.
- **Stale window**: after the fresh TTL, the cached payload may still be served with `stale: true` until the next Shanghai midnight, while a refresh runs in the background (stale-while-revalidate).
- **Partial campus success**: if one campus query fails but another succeeds, the payload is still cached; failed campuses keep prior same-day data when available, `partial_campuses` lists their IDs, and a top-level `error` describes the partial failure. While that partial payload is still inside the fresh TTL, the API **soft-stale revalidates**: it returns the data immediately and kicks a single-flight background refresh so the failed campus is retried without waiting the full 5 minutes.
- **Failure backoff**: after a **total** refresh failure **or** a **partial** refresh outcome (cached payload with top-level `error`), new refresh attempts are suppressed for 30 seconds so JW is not hammered. Stale/partial responses still carry the last user-facing error message where applicable.
- **Day stamping**: `date` / `expires_at` / `stale_until` on a cache entry are taken when the refresh **finishes** (not when it starts), so a JW round-trip that crosses Shanghai midnight is labeled for the completion day.
- **Cross-day reuse**: never. Yesterday's cache is ignored.

Refreshes are triggered on demand by `GET /api/get_data`, once at startup (warmup), and again after each Shanghai midnight. Concurrent requests share a single in-flight refresh. The cache is process-local: restarting clears it, and multiple instances do not share it.

If the teaching affairs system is temporarily unavailable but today's cache exists, the API returns `stale: true` plus an `error` object, and the frontend shows a warning banner (also shown for partial-campus `error` without stale). If a partial cache is followed by a total refresh failure, the newer total-failure warning takes precedence over the older partial warning. The UI keeps the last successful snapshot on background poll failures instead of blanking the page.

## Troubleshooting

| Symptom | Check | Likely cause / fix |
|---|---|---|
| `/readyz` 503, `last_login_error` mentions credentials or config | `sudo grep JW_ /etc/bupt-ec/bupt-ec.env` (as root) | Wrong `JW_USERNAME`/`JW_PASSWORD`. Rerun the installer to re-enter them. |
| `/readyz` 503, `last_refresh_error` set, login OK | JW system reachability from the server | Teaching affairs system down or unreachable; service recovers automatically. |
| API returns 503 with a `log_id` | `grep <log_id>` in logs | See the specific failure in the matching records. |
| Page loads but shows the stale warning | `/readyz` → `cache_fresh` | Upstream refresh failing; check `last_refresh_error`. |
| Service not listening | `sudo journalctl -u bupt-ec -n 50` | Startup validation failed (missing credentials) or port conflict on `APP_ADDR`. |
| 429 responses on `/api/` | Nginx rate limit | Installer default is 30 req/min per IP, burst 20; adjust the Nginx site if needed. |

## Security notes

- Tokens are held in memory only; nothing user-identifiable is persisted.
- `/etc/bupt-ec/bupt-ec.env` is mode `0600`, root-owned. Keep it that way.
- The binary at `/opt/bupt-ec/bupt-ec` is root-owned so the service user cannot replace its own executable.
- Never commit or paste real credentials, tokens, or log files.
