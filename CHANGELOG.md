# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Add user-visible changes to the `[Unreleased]` section as part of the change itself. `scripts/release.sh` turns that section into the next version entry, and the Release workflow publishes it as the GitHub release notes. See [docs/release.md](docs/release.md) for the full release process.

## [Unreleased]

### Dependencies

- Frontend test toolchain adds `@testing-library/react`, `@testing-library/dom`,
  and `jsdom` for hook lifecycle tests (dev-only; production bundle unchanged).

### Security

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

### Fixed

- Gzip negotiation now honors Accept-Encoding q-values (including `gzip;q=0`)
  instead of substring matching.
- Unknown `/api` routes return a correlated `LogID` header and body `log_id`.
- Shared classroom refresh workers preserve the initiator request log_id without
  inheriting client cancellation.
- Startup now resolves configuration once, honors `.env` values for `GIN_MODE`
  and `LOG_CALLER`, and fails safely on malformed/unreadable dotenv files or
  invalid listen addresses; process environment values still take precedence.
- Installer late first-install failures after service start or Nginx reload now
  stop the new unit and reload Nginx after removing targets, and upgrade
  rollback restores prior active/enabled state instead of always restarting.
- Generated Nginx `/api/` `proxy_read_timeout` is 60s so cold classroom refreshes
  within the 30s refresh / 45s Go write budgets are not cut off by the proxy.
- Installer installs and upgrades are now transactional: downloads, checksums,
  extraction, and config rendering finish before existing files change; failed
  Nginx/service/health validation restores the previous installation (or
  removes new first-install files) instead of leaving mixed binary/config state.
- Stable, nightly, and pinned installer commands now select the matching release
  explicitly; the installer remembers the selected channel or tag for later
  upgrades instead of silently falling back to `nightly`.
- Dark mode follows system `prefers-color-scheme` only (no conflicting
  `localStorage.darkMode` bootstrap), so React no longer thrash-overwrites the
  pre-hydration theme.
- Exact path `/api` now returns JSON 404 like other unknown API routes, instead
  of the SPA `index.html`.
- Day-boundary auto-reload considers Asia/Shanghai business date and
  `stale_until`; failed reloads now clear cross-day or expired snapshots instead
  of retaining yesterday's classroom filters and table.
- Background auto-refresh no longer full-page spins or replaces a successful
  same-day classroom snapshot with an empty error envelope. Hard-empty and
  repeated client failures retry after 5s/10s/20s/30s, while a successful fetch
  resets the backoff.
- Partial-campus cache hits no longer skip JW retries for the full 5-minute
  fresh TTL; soft-stale revalidation runs immediately (still single-flight,
  with the same 30s backoff after total or partial refresh outcomes).
- Classroom cache `Date`/`ExpiresAt`/`StaleUntil` are stamped at refresh
  completion (not start), so a JW refresh that straddles Asia/Shanghai midnight
  is labeled for the completion day and never stored with a non-positive
  go-cache TTL.
- HTTP `WriteTimeout` is now longer than the cold classroom refresh budget so
  clients are not cut off when a shared refresh succeeds near the previous 30s
  limit.
- Concurrent auth failures for the same old token now share one login, and
  delayed failures reuse the replacement token instead of triggering another
  login that could invalidate the first. `JW_TOKEN` is invalidated until
  restart only when the rejected token actually came from that override.
- Multi-campus refresh keeps partial results when one campus fails, merging prior
  same-day data when available instead of failing the whole payload; responses
  and readiness diagnostics identify the affected campus IDs, and the frontend
  warning names the affected campus when possible.
- A total refresh failure after a partial cached result now replaces the older
  partial warning, so users and operators see the latest upstream outage state.
- Business “today” and cache day boundaries use Asia/Shanghai (not the host TZ).
- Stale classroom payloads can poll after 5 seconds for an in-flight refresh;
  partial-campus payloads follow the backend's 30-second refresh backoff.
- Startup/midnight warmup is now cancellable and retries a missing cache with a
  bounded 30s/1m/2m/5m schedule instead of waiting until the following day
  after a transient failure or midnight backoff.
- Ended class periods are dropped from the selection so they cannot block room
  filters; empty/malformed period times are not treated as ended.
- Settings modal secondary text follows the theme (readable in dark mode).
- Settings gear remains available when campus list is empty (error/loading).
- `localStorage` failures no longer crash preference init/updates.

### Changed

- Docs (README, deployment/upgrading/operations/development, AGENTS) now describe
  partial-campus soft-stale retry, day-stamped cache metadata, frontend
  keep-last-good reload, and recommend stable tags for production installs
  (the installer keeps `nightly` only as its first-install fallback).
- Tidied repository structure by folding backend startup initialization into
  `main.go` and refreshing ignore rules for local-only project artifacts.
- `/readyz` `cache_stale` means usable cache past the fresh TTL (not merely
  “still within the same calendar day”).
- `/readyz` now separates cache age from completeness with `cache_partial`,
  `partial_campuses`, and a sanitized `last_refresh_warning`.
- Settings label for ended periods: “允许选择已结束节次”.
- Background warmup runs immediately, retries partial cache at a low frequency,
  and schedules complete-cache refreshes after each Shanghai midnight with a
  small jitter. Graceful shutdown stops the scheduler before draining workers.

### Dependencies

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

[Unreleased]: https://github.com/ming-kang/BUPT_EC/compare/v0.1.4...HEAD
[0.1.4]: https://github.com/ming-kang/BUPT_EC/compare/v0.1.3...v0.1.4
[0.1.3]: https://github.com/ming-kang/BUPT_EC/compare/v0.1.2...v0.1.3
[0.1.2]: https://github.com/ming-kang/BUPT_EC/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/ming-kang/BUPT_EC/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/ming-kang/BUPT_EC/releases/tag/v0.1.0
