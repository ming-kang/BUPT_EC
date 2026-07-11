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

## Metrics (loopback)

The process exposes Prometheus metrics at `GET /metrics` on `APP_ADDR` (default
`127.0.0.1:8080`). The installer Nginx site returns 404 for public `/metrics`
and does not proxy it. Scrape from the host only, for example:

```bash
curl -s http://127.0.0.1:8080/metrics | head
```

Useful series include `bupt_ec_refresh_total`, `bupt_ec_refresh_duration_seconds`,
`bupt_ec_cache_serves_total`, `bupt_ec_login_total`,
`bupt_ec_login_duration_seconds`, and `bupt_ec_refresh_suppressed_total`
(adaptive backoff). Labels are low-cardinality enums only (`outcome`, `source`,
campus id, error kind). Never expect tokens, usernames, URLs, or raw errors as
labels.

Login series (`bupt_ec_login_total` / `bupt_ec_login_duration_seconds`) count
shared JW network login operations only (not override install, cache hits, or
singleflight waiters). `source` is `override` when recovery was caused by a
rejected startup `JW_TOKEN`, otherwise `login`. `outcome` is `success` or
`failed`.

Response encoding: the Gin gzip middleware is the only compressor for
`/metrics`. Prometheus client compression is disabled so scrapers with
`Accept-Encoding: gzip` decompress once into Prometheus text format. Health and
readiness stay uncompressed.

Suggested alerts: sustained `refresh` outcome `failed`, rising
`refresh_suppressed_total`, login failures (`bupt_ec_login_total{outcome="failed"}`),
and refresh durations near 30s.

## Timeout budget (API cold path)

Cold `/api/get_data` may wait for a shared JW classroom refresh. The three layers
are intentionally ordered:

| Layer | Budget | Role |
| --- | --- | --- |
| JW / classroom refresh context | 30s | shared upstream work bound |
| Go HTTP `WriteTimeout` | 45s | allows the handler to finish after refresh |
| Nginx `/api/` `proxy_read_timeout` | 60s | leaves transfer margin for the JSON response |

SPA/`/` proxy read timeout stays at 30s. If clients see proxy 504s only on cold
API loads, verify the installed site still has the 60s `/api/` value.

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
classified internal errors. Upstream JW message text is normalized for single-line
logs (Unicode whitespace/controls/format runes collapsed), secret key/value
fragments redacted, and capped at 256 runes before it appears in those fields.
These diagnostics never include JW credentials or
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
- **Failure backoff**: after a **partial** refresh outcome (cached payload with
  top-level `error`), new refresh attempts are suppressed for a fixed **30
  seconds**. After a **total** refresh failure, consecutive failures escalate a
  base ladder of **30s → 1m → 2m → 5m (cap)** with a small bounded jitter of
  about ±10% of the base step (absolute cap ±5s) so retries do not stampede JW.
  Full success resets the ladder. Stale/partial responses still carry the last
  user-facing error message where applicable. Backoff may span Shanghai
  midnight; yesterday's cache is never reused, and the next allowed attempt can
  refresh the new business day once `nextRefreshAllowed` is reached.
- **Day stamping**: `date` / `expires_at` / `stale_until` on a cache entry are taken when the refresh **finishes** (not when it starts), so a JW round-trip that crosses Shanghai midnight is labeled for the completion day.
- **Cross-day reuse**: never. Yesterday's cache is ignored.

Refreshes are triggered on demand by `GET /api/get_data` and by a process-local warmup scheduler. The scheduler attempts immediately at startup, then behaves according to cache state:

- no usable cache: retry after 30 seconds, 1 minute, 2 minutes, then at most every 5 minutes (while still respecting the refresh coordinator's `nextRefreshAllowed` backoff);
- partial cache: retry no faster than the 5-minute fresh TTL, unless the Shanghai day boundary arrives first;
- complete cache: wait until the next Shanghai midnight plus a small 1–5 second jitter.

This means a midnight refresh suppressed by a backoff is retried after the allowed time instead of being abandoned until the following day. Concurrent requests and warmup share the same single in-flight refresh. The cache is process-local: restarting clears it, and multiple instances do not share it.

## Deployment topology (supported today)

### Recommended

One host runs:

- one `bupt-ec` process (systemd unit, loopback `APP_ADDR`)
- one Nginx reverse proxy (TLS, rate limit, static SPA via the Go binary embed)

This is the only topology the installer and docs treat as production-supported.

### Process-local state (not shared)

Each process privately owns:

- same-day classroom cache
- JW token / API URL cache
- refresh singleflight and adaptive backoff
- runtime readiness / metrics

Restart clears that memory until warmup succeeds again. Running two or more
`bupt-ec` instances behind round-robin is **not** recommended: each instance
logs into JW and refreshes independently, multiplies upstream load, and can
return briefly different readiness or stale flags. In-process singleflight is
**not** cross-instance coordination.

### Capacity and failure boundary (current)

| Concern | Behavior |
| --- | --- |
| Concurrent clients | Share one refresh attempt per process |
| Scale-out | Vertical (larger host / better JW headroom), not multi-app replicas |
| Restart | Cache miss until warmup; brief `/readyz` 503 is expected |
| JW outage | Adaptive backoff + stale/partial cache while still same Shanghai day |
| Signals | `/readyz`, structured logs (`log_id`), loopback `/metrics` |

No SLA or QPS figure is claimed without capacity testing.

### Future expansion options (not implemented)

These are design options only. None of Redis, leader election, or HA multi-writer
exists in this repository today.

| Option | Idea | Trade-offs |
| --- | --- | --- |
| A. Shared typed cache + distributed refresh lock | Multiple API processes read one cache; one refresh owner at a time | Needs external store + lock; token ownership and secret distribution become explicit |
| B. Leader/fetcher + read replicas | One writer builds the snapshot; others serve read-only | Clear JW load control; leader failover and snapshot freshness protocol required |
| C. Stay single-instance + cold spare | Keep process-local design; recover with systemd restart / standby host | Simplest; no online multi-AZ active/active |

Until one of those is designed and implemented, operate a single primary instance.

During graceful shutdown, the application cancels the warmup scheduler before draining HTTP handlers, then waits for the scheduler and already-started refresh workers. Once background draining begins, no new refresh worker can be added.

If the teaching affairs system is temporarily unavailable but today's cache exists, the API returns `stale: true` plus an `error` object, and the frontend shows a warning banner (also shown for partial-campus `error` without stale). When `partial_campuses` is present, the banner names the affected campus or shows its ID. If a partial cache is followed by a total refresh failure, the newer total-failure warning takes precedence over the older partial warning.

The UI keeps the last successful snapshot after a browser/network refresh failure only while its `date` still matches the current Asia/Shanghai business day, `stale_until` is valid and in the future, and `campuses` remains an array. Once the snapshot crosses midnight or passes `stale_until`, the page clears the classroom filters/table and shows the hard error state instead of yesterday's data. Hard-empty and repeated client-refresh failures retry automatically after 10, 20, 30, then at most 60 seconds; a valid response resets the failure count. Partial-campus payloads poll no faster than the backend's 30-second refresh backoff (base), ordinary stale payloads no faster than 15 seconds, and fresh payloads wait until `expires_at` (1s floor). Auto-reload applies a small **positive-only** jitter (≤10% of the base delay, absolute cap 5s) so multi-tab clients desync without shortening those minimum intervals, then clamps the final delay to remaining `stale_until` time so a large random sample cannot schedule past the hard display deadline. Hidden tabs cancel the reload timer; becoming visible after the deadline revalidates promptly and does not keep showing yesterday's filters. These background retries do not trigger the full-page loading spinner.

## Troubleshooting

### Installer rollback failures

An installer failure before commit (download, checksum, extraction, or rendering) leaves the installed binary and configuration untouched. A failure after commit starts an automatic rollback and should end with `Rollback completed.` before the installer exits non-zero.

If the message instead says the automatic rollback was incomplete, note the printed recovery directory immediately. It is mode `0700` and contains the snapshot manifest plus any previous binary/config files, including a mode-`0600` env backup. Keep it root-only, inspect `systemctl status bupt-ec`, `nginx -t`, and the installer output, then restore or rerun the installer only after understanding which validation failed. The directory is intentionally preserved for manual recovery; delete it securely after recovery succeeds.

| Symptom | Check | Likely cause / fix |
|---|---|---|
| Installer exits during upgrade | Look for `Rollback completed.` and run `sudo systemctl status bupt-ec` | New Nginx/service/health validation failed; the previous installation was restored. Fix the reported cause before retrying. |
| Installer reports incomplete rollback | Preserve the printed root-only recovery directory; run `sudo nginx -t` and inspect `journalctl` | Automatic restoration or old-service restart also failed; use the preserved snapshot for manual recovery. |
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
