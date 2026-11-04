# Operations

Day-to-day operation of a deployed BUPT_EC server: service management, health checks, logs, caching behavior (data and static assets), and troubleshooting.

## Service management

```bash
sudo systemctl status bupt-ec
sudo systemctl restart bupt-ec
sudo systemctl stop bupt-ec
sudo systemctl start bupt-ec
sudo journalctl -u bupt-ec -f
sudo journalctl -u bupt-ec -n 200
```

A readiness 503 is normal right after a restart; `/readyz` returns 200 once
the first classroom refresh succeeds.

On shutdown the server cancels the refresh loop (stopping any in-flight JW
request), stops accepting connections, and drains in-flight requests.

## Timeout budget (API cold path)

Cold `/api/get_data` may wait for a shared JW classroom refresh. The three layers
are intentionally ordered:

| Layer | Budget | Role |
| --- | --- | --- |
| Cold-miss wait (`service.ColdWaitTimeout`) | 5s | handler fails fast with `503` + `Retry-After`; the refresh continues in the background |
| JW classroom refresh context | 30s | refresh-loop upstream work bound (background only) |
| Go HTTP `WriteTimeout` | 15s | headroom over the cold-wait bound for marshal + transfer |
| Nginx `/api/` `proxy_read_timeout` | 60s | leaves transfer margin for the JSON response |

SPA/`/` proxy read timeout stays at 30s. If clients see proxy 504s only on cold
API loads, verify the installed site still has the 60s `/api/` value.

## Health endpoints

Both endpoints bypass gzip and are safe for load-balancer probes.

### `GET /healthz` — liveness

Returns `200 {"status":"ok"}` whenever the process is up.

### `GET /readyz` — readiness

Returns `200` when JW credentials are configured **and** a usable same-day cache exists; `503` otherwise.

The body is deliberately minimal (no runtime internals on the public surface):

```json
{
  "status": "OK",
  "version": "v0.1.6"
}
```

- `version`: build version string. Release binaries get the tag injected via `-ldflags "-X main.version=..."` (`main-<short-sha>` for unreleased `main` builds); locally built binaries report `dev`.
- `cache_fresh`: within the ~5 minute fresh TTL.
- `cache_stale`: cache is still usable for the business day but **past** the fresh TTL (not simply “same calendar day”). Fresh cache has `cache_stale: false`.
- `cache_partial`: at least one configured campus used prior same-day data or an empty skeleton during the latest usable refresh. `partial_campuses` lists the affected campus IDs when present.
- Business day boundaries use **Asia/Shanghai**.

`last_refresh_warning` describes a usable partial outcome, while
`last_refresh_error` describes the latest total refresh failure. A partial cache
can remain ready; use `cache_partial` and `partial_campuses` to identify its
scope. A `503` right after a restart is normal until the first refresh finishes.

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

### Persisted logging settings

`LOG_CALLER` is a persisted installer deployment setting. A normal `--mode=update` copies its saved value unchanged; the `install` and `reconfigure` paths use an explicitly supplied value before a saved value, without adding a flag for it. To change it, rerun the installer in reconfigure mode with the variable supplied:

```bash
sudo LOG_CALLER=1 bash /tmp/bupt-ec-install.sh --mode=reconfigure
```

The service snapshots its environment at startup; it does not watch the env
file for hot reload. A successful reconfigure runs the normal transaction and
restarts the service. Prefer it to editing the env by hand; a direct edit takes
effect only after an explicit service restart.

The installed `/etc/bupt-ec/bupt-ec.env` must remain a regular, root-owned
`0600` file in a root-owned configuration directory that group/other users
cannot modify; do not replace it with a symlink. Before it uses saved values,
the Installer evaluates it only in an isolated child and frames the fixed
supported fields. If the safe loader rejects the file, repair its ownership,
mode, or syntax as root (or move it aside and rebuild with default install)
rather than sourcing an untrusted file.

### Tracing a request with `log_id`

Every `/api/*` request gets a `log_id` that appears:

- in every log record produced while handling that request,
- in the `X-Log-Id` response header,
- in the JSON body of error responses from `/api/get_data`.

When a user reports an error, ask for the `log_id` from the error response and search the logs:

```bash
sudo grep '20260703081504A1B2C3D4E5F6A7B8C9D2' /opt/bupt-ec/run_log/ec.log
# or
sudo journalctl -u bupt-ec | grep '20260703081504A1B2C3D4E5F6A7B8C9D2'
```

## Caching behavior

The backend keeps a single same-day in-memory cache of classroom data (business day = **Asia/Shanghai**):

- **Fresh TTL**: about 5 minutes. A fully successful fresh snapshot is served directly with no JW call.
- **Stale window**: after the fresh TTL the snapshot is served with `stale: true` until the next Shanghai midnight while the refresh loop is nudged in the background.
- **Partial campus success**: if one campus fails but another succeeds, the snapshot is still published; failed campuses keep prior same-day data when available, `partial_campuses` lists their IDs, and a top-level `error` describes the partial failure. Requests that see a partial snapshot nudge the loop, which retries no sooner than **30 seconds** after the partial result.
- **Failure backoff**: consecutive total failures retry after **30s → 1m → 2m → 5m (cap)**; request nudges inside that window are ignored and cold requests fail fast with the last error. Any usable result resets the ladder. Stale responses carry the last total-failure message when there is one.
- **Day stamping**: `date` / `expires_at` / `stale_until` are taken when the refresh **finishes**, so a JW round-trip that crosses Shanghai midnight is labeled for the completion day.
- **Cross-day reuse**: never. Yesterday's snapshot is ignored.

All JW traffic comes from one in-process refresh loop. It refreshes immediately at startup, then at the next Shanghai midnight after a full success, every 5 minutes while partial, along the failure ladder after total failures, and whenever a request nudges it outside the backoff window. Concurrent requests never start their own JW calls. The cache is process-local: restarting clears it, and multiple instances do not share it.

## Static asset caching (SPA)

The Go binary serves the embedded frontend with two deliberately different policies:

| Resource | Cache-Control | Revalidation |
| --- | --- | --- |
| `/assets/*` (build output, content hash in the filename) | `public, max-age=31536000, immutable` | none needed — a new build produces new filenames |
| `index.html` and every SPA fallback path | `no-cache` | weak `ETag`, so unchanged HTML answers a cheap `304` |
| `favicon.ico` (dist root, no content hash) | `no-cache` | served fresh each time (no `ETag`) |

`no-cache` means "revalidate before reuse", not "never store". That split is
what makes an upgrade visible on the next page load while hashed assets stay
cacheable for a year. Do not add HTML caching in Nginx or a CDN in front of it.

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
- refresh loop and failure backoff
- runtime readiness

Restart clears that memory until the first refresh succeeds again. Running two or more
`bupt-ec` instances behind round-robin is **not** recommended: each instance
logs into JW and refreshes independently, multiplies upstream load, and can
return briefly different readiness or stale flags. The in-process refresh loop is
**not** cross-instance coordination.

### Capacity and failure boundary (current)

| Concern | Behavior |
| --- | --- |
| Concurrent clients | Read one snapshot; one refresh loop per process |
| Scale-out | Vertical (larger host / better JW headroom), not multi-app replicas |
| Restart | Cache miss until the first refresh; brief `/readyz` 503 is expected |
| JW outage | Failure backoff + stale/partial cache while still same Shanghai day |
| Signals | `/readyz`, structured logs (`log_id`) |

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

If the teaching affairs system is temporarily unavailable but today's cache exists, the API returns `stale: true` plus an `error` object, and the frontend shows a warning banner (also shown for partial-campus `error` without stale). When `partial_campuses` is present, the banner names the affected campus or shows its ID. If a partial cache is followed by a total refresh failure, the newer total-failure warning takes precedence over the older partial warning.

The UI keeps the last successful snapshot after a browser/network refresh failure only while its `date` still matches the current Asia/Shanghai business day, and `stale_until` is in the future. Once the snapshot crosses midnight or passes `stale_until`, the page clears the classroom filters/table and shows the hard error state instead of yesterday's data. Hard-empty and repeated client-refresh failures retry automatically after 10, 20, 30, then at most 60 seconds; a valid response resets the failure count. Partial-campus payloads poll no faster than the backend's 30-second refresh backoff (base), ordinary stale payloads no faster than 15 seconds, and fresh payloads wait until `expires_at` (1s floor). Auto-reload applies a small **positive-only** jitter (≤10% of the base delay, absolute cap 5s) so multi-tab clients desync without shortening those minimum intervals, then clamps the final delay to remaining `stale_until` time so a large random sample cannot schedule past the hard display deadline. Hidden tabs issue no requests; becoming visible after the deadline revalidates promptly and does not keep showing yesterday's filters. These background retries do not trigger the full-page loading spinner.

## Troubleshooting

### Installer rollback failures

An installer failure before commit (download, checksum, extraction, or rendering) leaves the installed binary and configuration untouched. A failure after commit starts an automatic rollback and should end with `Rollback completed.` before the installer exits non-zero.

If the message instead says the automatic rollback was incomplete, the root-only journal at `/var/lib/bupt-ec/transaction` is kept. Inspect `systemctl status bupt-ec`, `nginx -t`, and the installer output, fix the reported problem, then rerun the installer: it restores from the journal before starting a new deployment. The same applies after a crash or power loss during an install or upgrade.

| Symptom | Check | Likely cause / fix |
|---|---|---|
| Installer exits during upgrade | Look for `Rollback completed.` and run `sudo systemctl status bupt-ec` | New Nginx/service/health validation failed; the previous installation was restored. Fix the reported cause before retrying. |
| Installer reports incomplete rollback | Run `sudo nginx -t` and inspect `journalctl` | Automatic restoration or old-service restart also failed; fix the cause and rerun the installer to retry the restore from `/var/lib/bupt-ec/transaction`. |
| `/readyz` 503 and startup logs mention credentials or config | `sudo grep JW_ /etc/bupt-ec/bupt-ec.env` (as root) | Wrong `JW_USERNAME`/`JW_PASSWORD`. Rerun with `--mode=reconfigure`, supplying corrected values. |
| `/readyz` 503, login succeeds, refresh errors in the logs | JW system reachability from the server | Teaching affairs system down or unreachable; service recovers automatically. |
| API returns 503 with a `log_id` | `grep <log_id>` in logs | See the specific failure in the matching records. |
| Page loads but shows the stale warning | `journalctl -u bupt-ec` for refresh errors | Upstream refresh failing; check the logged `last_refresh_error`-equivalent diagnostics. |
| Browser still shows the old UI after an upgrade | `curl -sI https://<domain>/` → `Cache-Control` must be `no-cache`; `curl -s http://127.0.0.1:8080/readyz` → `version` | Service not restarted or old artifact installed (check `version` first). If the header is not `no-cache`, an Nginx/CDN layer is caching HTML — stop caching `index.html`; hashed `/assets/*` are safe to cache. A hard reload confirms which side is stale. |
| Service not listening | `sudo journalctl -u bupt-ec -n 50` | Startup validation failed (missing credentials) or port conflict on `APP_ADDR`. |
| 429 responses on `/api/` | Nginx rate limit | Installer default is 30 req/min per IP, burst 20; adjust the Nginx site if needed. |

## Security notes

- Tokens are held in memory only; nothing user-identifiable is persisted.
- `/etc/bupt-ec/bupt-ec.env` is a regular root-owned mode-`0600` file in a root-controlled, non-writable-by-others directory. Keep it that way; the installer refuses unsafe ownership, mode, or symlink layouts.
- The binary at `/opt/bupt-ec/bupt-ec` is root-owned so the service user cannot replace its own executable.
- Never commit or paste real credentials, tokens, or log files.
