# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Add user-visible changes to the `[Unreleased]` section as part of the change itself. `scripts/release.sh` turns that section into the next version entry, and the Release workflow publishes it as the GitHub release notes. See [docs/release.md](docs/release.md) for the full release process.

## [Unreleased]

### Security

- Default listen address is `127.0.0.1:8080` when `APP_ADDR` is unset (no longer
  binds all interfaces via `:8080`).
- JW outbound HTTP no longer follows redirects, so custom `token` headers and
  login form bodies cannot be sent to unvalidated hosts.

### Fixed

- Day-boundary auto-reload considers Asia/Shanghai business date and
  `stale_until`, so yesterday’s “fresh” payload is not held until `expires_at`
  after midnight.
- Background auto-refresh no longer full-page spins or replaces a successful
  classroom snapshot with an empty error envelope; last-good data is kept with
  a soft warning until the next successful fetch (hard empty only when there is
  no prior good data).
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
- Invalid `JW_TOKEN` no longer overrides a successfully re-logged-in token after
  an auth failure (override is invalidated until process restart).
- Multi-campus refresh keeps partial results when one campus fails, merging prior
  same-day data when available instead of failing the whole payload.
- Business “today” and cache day boundaries use Asia/Shanghai (not the host TZ).
- Stale/partial classroom payloads auto-refresh every few seconds instead of
  waiting a minimum of 60 seconds past `expires_at`.
- Ended class periods are dropped from the selection so they cannot block room
  filters; empty/malformed period times are not treated as ended.
- Settings modal secondary text follows the theme (readable in dark mode).
- Settings gear remains available when campus list is empty (error/loading).
- `localStorage` failures no longer crash preference init/updates.

### Changed

- Tidied repository structure by folding backend startup initialization into
  `main.go` and refreshing ignore rules for local-only project artifacts.
- `/readyz` `cache_stale` means usable cache past the fresh TTL (not merely
  “still within the same calendar day”).
- Settings label for ended periods: “允许选择已结束节次”.
- Background warmup re-runs after each Shanghai midnight for long-lived processes.

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
