# Quality Guidelines

## Overview

Backend changes should preserve the current small-service architecture: thin Gin
handlers, injectable service dependencies, no local timetable database, safe JW
error handling, and source-backed API contracts. Keep code readable and verify
with the same commands CI uses.

Primary references:

- `.github/workflows/ci.yml` and `.github/workflows/release.yml` for quality
  gates.
- `service/realtime_data_test.go` for service unit test style.
- `handler_test.go` for HTTP/router test style.
- `docs/development.md`, `docs/release.md`, and `CHANGELOG.md` for workflow and
  release conventions.

## Required Go Patterns

- Format all Go code with `gofmt`. CI expects `gofmt -l .` to print nothing.
- Keep service dependencies injectable. `ClassroomService` accepts a `CacheStore`
  and internally uses a `JWClient`; tests create isolated services with
  `newTestService(t, client)` and a fresh `gocache` instance.
- Use contexts for external work. JW login, API URL fetch, classroom refreshes,
  and HTTP requests all use bounded contexts.
- Keep handlers thin and service logic testable without Gin.
- Keep public JSON structs in `service/model/` with explicit `json` tags.
- Use `errors.As`, `errors.Is`, or `errors.Join` instead of string-matching
  error text.
- Prefer clear exported APIs only at package boundaries; keep helper functions
  unexported unless another package really needs them.

## Testing Requirements

Add or update focused tests when changing behavior, especially for:

- JW response parsing and error classification;
- cache freshness, stale data, and cross-day rejection;
- refresh coordination and concurrency sharing;
- room parsing/building normalization;
- HTTP envelopes, health/readiness behavior, gzip, and SPA fallback.

Local test patterns:

- `service/realtime_data_test.go` defines `mockJWClient` and `newTestService`.
  Follow this pattern for service tests so unit tests do not touch the network.
- Integration tests such as `TestLogin`, `TestQueryOne`, and `TestQueryAll`
  require `JW_TOKEN` or `JW_USERNAME`/`JW_PASSWORD` and must skip cleanly when
  credentials are missing.
- Handler tests can assign the package-level `classroomService` directly and
  use `httptest` plus `gin.New()`.

Avoid tests that only restate the implementation. Prefer tests that protect
contract edges, race-prone behavior, security checks, or user-visible output.

## Verification Commands

Use the smallest reliable set while developing, then run broader checks before
finishing substantial backend changes:

```bash
gofmt -l .
go vet ./...
go test ./...
```

CI and release quality gates also run:

```bash
go test -race ./...
go build -o bupt-ec -v ./
govulncheck ./...
shellcheck scripts/*.sh
cd frontend && pnpm lint
cd frontend && pnpm build
```

The Go build embeds `frontend/dist/` through `router.go`, so local full builds
need `cd frontend && pnpm build` first if `frontend/dist/` is missing.

## Frontend and Cross-Layer Quality

Backend API changes often require frontend changes because the React app reads
the backend payload directly. Before changing `service/model/realtime_data.go`,
`classroom_builder.go`, or `handler.go`, inspect these frontend files:

- `frontend/src/useTodayClassrooms.js` for fetch and normalization.
- `frontend/src/components/BuildingPicker.jsx` for building assumptions.
- `frontend/src/components/ClassTimePicker.jsx` for campus node assumptions.
- `frontend/src/components/TodayClassroomTable.jsx` for `free_nodes` filtering.

Frontend code uses ES modules, React hooks, 2-space indentation, PascalCase
component filenames, matching component CSS files, and shared selection state
through `useSelection()` rather than prop drilling.

## Documentation and Release Hygiene

User-visible changes must update documentation and changelog in the same change:

- Update `README.md` or `docs/` when behavior, endpoints, config, deployment,
  operations, or release process changes.
- Add a bullet to `CHANGELOG.md` under `[Unreleased]` using Keep a Changelog
  categories (`Added`, `Changed`, `Fixed`, `Removed`, `Deprecated`, `Security`,
  or `Dependencies`).
- Commit messages in this repository use Conventional Commit prefixes such as
  `feat:`, `fix:`, `chore:`, `ci:`, `docs:`, and `refactor:`.
- Do not mix module renames, dependency updates, and behavior changes in one
  commit.

Release automation depends on exact asset names and layout. If changing release
assets or installer behavior, update `scripts/release.sh`, `scripts/install.sh`,
`.github/workflows/release.yml`, and `docs/release.md` together.

## Security Checklist

- Never commit real `.env` files, `JW_USERNAME`, `JW_PASSWORD`, `JW_TOKEN`, or
  generated `run_log/` files.
- Keep JW API URL validation restricted to HTTPS BUPT hosts.
- Keep the AES key in `service/crypto.go` aligned with the JW protocol; do not
  change it casually.
- Keep `/etc/bupt-ec/bupt-ec.env` documented as root-owned mode `0600` for
  deployed servers.
- Keep production `APP_ADDR` behind Nginx as `127.0.0.1:8080` unless the deploy
  design changes.

## Review Checklist

- Does the change preserve `ClassroomService` as the owner of mutable runtime
  state?
- Are errors classified internally and converted to safe client messages only at
  the HTTP boundary?
- Are logs structured and correlated with request context when applicable?
- Does any public JSON change update frontend consumers and tests?
- Do same-day cache and stale behavior still reject cross-day reuse?
- Are docs and changelog updated for user-visible behavior?
- Did the author run the relevant Go/frontend/script checks?

## Forbidden Patterns

- Network-dependent unit tests that do not skip without credentials.
- Global mutable state in `service/` that bypasses injected test instances.
- Raw `err.Error()` in API responses.
- Logging secrets or raw upstream payloads.
- Reintroducing local timetable persistence as an incidental implementation
  detail.
