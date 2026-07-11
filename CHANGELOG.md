# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Add user-visible changes to the `[Unreleased]` section as part of the change itself. `scripts/release.sh` turns that section into the next version entry, and the Release workflow publishes it as the GitHub release notes. See [docs/release.md](docs/release.md) for the full release process.

## [Unreleased]

## [0.1.5] - 2026-07-11

### Security

- Upstream JW error text is sanitized before internal logs/errors: Unicode
  whitespace, line/paragraph separators, C0/C1 controls, and format/bidi
  controls fold to ASCII space (normalize before redaction so Unicode spaces
  cannot bypass sensitive key/value matching); token/password/account/Bearer
  fragments redact; messages cap at 256 runes (clients still only see fixed
  SafeErrorMessage text).
- Go builds and releases now require patched Go 1.25.12 (or Go 1.26.5+), and
  the reachable `quic-go` dependency is updated to v0.59.1.
- PR and release quality gates now audit production frontend dependencies at
  moderate severity and the complete frontend toolchain at high severity.
- Installer no longer auto-falls back to third-party download proxies when
  GitHub is unreachable; operators must set an explicit `DOWNLOAD_BASE_URL`
  for a mirror they trust (same-origin checksums verify integrity only).
- Installer now fails closed when `checksums.txt` cannot be downloaded or the
  package hash is missing/fails verification (`SKIP_CHECKSUM=1` is the only
  explicit break-glass opt-out).
- Default listen address is `127.0.0.1:8080` when `APP_ADDR` is unset (no longer
  binds all interfaces via `:8080`).
- JW outbound HTTP no longer follows redirects, so custom `token` headers and
  login form bodies cannot be sent to unvalidated hosts.

### Added

- Process-local Prometheus metrics on loopback `GET /metrics` (refresh, cache
  serve, login, campus failure counters/histograms). Public Nginx returns 404
  for `/metrics`.
- `/readyz` now separates cache age from completeness with `cache_partial`,
  `partial_campuses`, and a sanitized `last_refresh_warning`.
- Shared quality gate workflow (`.github/workflows/quality.yml`) for PRs and
  main, including `go mod tidy -diff`, `go mod verify`, frontend audits, and
  frontend tests.
- JW protocol offline unit fixtures and React hook lifecycle tests
  (`useTodayClassrooms` harness).

### Changed

- Stable tag releases publish changelog-only notes (`body_path`) without
  GitHub-generated appendices. Nightly may still use generated notes.
- Total JW refresh failures escalate backoff 30s → 1m → 2m → 5m (cap) with
  bounded symmetric jitter (±10% of the base step, absolute cap ±5s); full
  success resets the ladder; partial success keeps the fixed 30s soft backoff
  without total-failure jitter.
- Frontend auto-reload uses rate-aware delays (stale ≥15s, partial ≥30s, failure
  10/20/30/60s) with positive-only bounded jitter (≤10% of base, cap 5s), clamps
  the final delay to `stale_until` after jitter, pauses while the tab is hidden,
  and aborts hung `/api/get_data` fetches after 40s.
- Background warmup runs immediately, retries partial cache at a low frequency,
  and schedules complete-cache refreshes after each Shanghai midnight with a
  small jitter. Graceful shutdown stops the scheduler before draining workers.
- Multi-campus refresh keeps partial results when one campus fails, merging prior
  same-day data when available; responses and readiness identify affected campus
  IDs, and the frontend prefers usable campuses instead of empty placeholders.
- Business “today” and cache day boundaries use Asia/Shanghai (not the host TZ).
  Cache `Date`/`ExpiresAt`/`StaleUntil` are stamped at refresh completion.
- Runtime configuration is snapshotted once at startup (`config.Load`); process
  env overrides `.env`; malformed dotenv or invalid listen address fails closed.
- Classroom cache is a typed `TodayClassroomCache` with an injectable shared
  `Clock` for refresh/backoff scheduling.
- Dark-mode pre-hydration bootstrap is a CSP-safe module script; theme follows
  system `prefers-color-scheme` only (no conflicting `localStorage.darkMode`).
- Selection preference persistence lives in `SelectionProvider` effects so the
  reducer stays pure.
- Settings label for ended periods: “允许选择已结束节次”; `updated_at` is labeled
  as the current data update time (not last refresh attempt).
- `/readyz` `cache_stale` means usable cache past the fresh TTL (not merely
  “still within the same calendar day”).
- Docs (README, deployment/upgrading/operations/development, AGENTS) cover
  partial-campus soft-stale retry, day-stamped cache metadata, frontend
  keep-last-good reload, single-instance topology, and recommend stable tags for
  production installs (`nightly` remains first-install fallback only).
- Backend startup composition folded into `main.go`; repository ignore rules
  refreshed for local-only artifacts.

### Fixed

- Installer `DOWNLOAD_BASE_URL` validation rejects userinfo, query, fragment,
  empty hosts, and non-HTTP(S) schemes (even with the insecure opt-in); logs a
  safe host label only; and restricts curl initial/redirect protocols to HTTPS
  (or HTTP+HTTPS for explicit HTTP break-glass mirrors).
- Installer installs and upgrades are transactional: download/checksum/extract/
  render finish before existing files change; late failures after service start
  or Nginx reload restore prior active/enabled state (or remove first-install
  targets) instead of mixed binary/config; generated `/api/` `proxy_read_timeout`
  is 60s for cold refresh budgets.
- Stable, nightly, and pinned installer commands select the matching release
  explicitly; the installer remembers the selected channel or tag for upgrades
  instead of silently falling back to `nightly`.
- Frontend auto-reload jitter is positive-only, samples randomness once per
  schedule, and clamps to remaining `stale_until` so multi-tab clients cannot
  schedule past the hard display deadline.
- Loopback `GET /metrics` no longer double-gzips when scrapers send
  `Accept-Encoding: gzip` (Prometheus handler compression disabled; router gzip
  remains the sole encoder).
- JW login metrics (`bupt_ec_login_total` / `bupt_ec_login_duration_seconds`)
  emit at the shared `TokenManager` login boundary with low-cardinality
  `outcome`/`source` labels (one sample per singleflight operation).
- Partial-campus cache hits soft-revalidate inside the fresh TTL instead of
  skipping JW retries for the full 5 minutes; total failure after partial cache
  replaces the older partial warning.
- Day-boundary auto-reload uses Asia/Shanghai business date and `stale_until`;
  failed reloads clear cross-day or expired snapshots instead of retaining
  yesterday’s filters and table.
- Background auto-refresh no longer full-page spins or replaces a successful
  same-day snapshot with an empty error envelope.
- Ended class periods are dropped from selection so they cannot block room
  filters; empty/malformed period times are not treated as ended.
- Gzip negotiation honors Accept-Encoding q-values (including `gzip;q=0`).
- Exact path `/api` and unknown `/api/*` routes return correlated JSON 404 with
  `LogID` header and body `log_id`.
- Shared classroom refresh workers preserve the initiator `log_id` without
  inheriting client cancellation.
- Concurrent auth failures for the same rejected token share one login; delayed
  failures reuse the replacement token; `JW_TOKEN` is invalidated until restart
  only when that override token was actually rejected.
- HTTP `WriteTimeout` is longer than the cold classroom refresh budget.
- Logging initialization returns an error instead of `log.Fatal`; `NewHTTPServer`
  rejects a nil classroom service before route registration.
- Settings modal secondary text follows the theme; settings gear remains
  available when the campus list is empty; `localStorage` failures no longer
  crash preference init/updates.
- Go module metadata marks directly imported Prometheus packages as direct
  dependencies; `go.sum` matches `go mod tidy`.

### Dependencies

- Frontend test toolchain adds `@testing-library/react`, `@testing-library/dom`,
  and `jsdom` for hook lifecycle tests (dev-only; production bundle unchanged).
- Frontend tooling now uses Vite 6.4.3 and ESLint 9.39.4 flat config; patched
  Babel runtime and ESLint transitive dependency versions clear the previous
  production and high-severity development audit findings.

## [0.1.4] - 2026-07-03

### Added

- SPA fallback route: unknown non-API paths serve `index.html`; unknown `/api/*` paths return JSON 404.
- Error responses from `/api/get_data` include a `log_id` field for correlating with server logs.
- Graceful shutdown waits for in-flight background classroom refreshes to finish.
- Frontend auto-reloads classroom data when the cached payload expires (`expires_at`).
- Error boundary around the lazy-loaded classroom table with a visible failure message.
- This changelog, plus `scripts/release.sh` for cutting releases; stable release notes are now taken from the matching changelog section.

### Changed

- Documentation reorganized: `README.md` is a short overview, with detailed guides under `docs/` (deployment, upgrading, operations, development, release).
- CI reorganized: pull requests are validated by `ci.yml`; pushes to `main` by the release quality gate. The quality gate now also enforces `gofmt`, `go vet`, and the race detector.
- Logging migrated to `log/slog` with structured JSON output; every request-scoped record carries a `log_id`.
- Backend refactored from package-level globals to a `ClassroomService` struct with a stateless `JWClient` interface; tests now use interface mocks instead of function-pointer swapping.
- `service/realtime_data.go` split into focused files (token manager, refresh coordinator, JW client, classroom builder, error classification, runtime status, crypto, URL utilities).
- Frontend selection state moved from prop drilling to a `useReducer` + Context store; display preferences initialize from `localStorage` on first render.
- Dark-mode disabled-button styling moved from inline styles to CSS; `isDark` prop removed from `ClassTimePicker`.

### Fixed

- Invalid Ant Design button `type="outline"` replaced with `type="default"` in building and class-time pickers.

### Dependencies

- gin v1.9.1 → v1.12.0; gin-contrib/static v0.0.1 → v1.1.6.

## [0.1.3] - 2026-06-16

### Added

- Classroom detail modal improvements.

### Fixed

- Backend refresh stability improvements.

## [0.1.2] - 2026-06-16

### Changed

- Frontend bundles split for faster loading.

### Fixed

- Token is refreshed on JW business-level auth errors, not only HTTP 401/403.
- Installer and release documentation aligned with actual behavior.

## [0.1.1] - 2026-06-14

### Changed

- Frontend UI refresh.
- Vulnerable Go dependencies updated.

### Added

- Existing-certificate VPS deployment documentation.

## [0.1.0] - 2026-06-14

### Added

- Initial release: Go/Gin backend querying the BUPT JW `todayClassrooms` endpoint with automatic HTTP login, same-day in-memory cache, and an embedded React/Ant Design frontend.
- Xitucheng and Shahe campus support with building and class-period filters.
- One-command installer (`install.sh`) configuring systemd and Nginx on Debian/Ubuntu.
- Release pipeline publishing Linux amd64/arm64 tarballs with checksums and build provenance attestations.

[Unreleased]: https://github.com/ming-kang/BUPT_EC/compare/v0.1.5...HEAD
[0.1.5]: https://github.com/ming-kang/BUPT_EC/compare/v0.1.4...v0.1.5
[0.1.4]: https://github.com/ming-kang/BUPT_EC/compare/v0.1.3...v0.1.4
[0.1.3]: https://github.com/ming-kang/BUPT_EC/compare/v0.1.2...v0.1.3
[0.1.2]: https://github.com/ming-kang/BUPT_EC/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/ming-kang/BUPT_EC/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/ming-kang/BUPT_EC/releases/tag/v0.1.0
