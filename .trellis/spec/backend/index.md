# Backend Development Guidelines

These specs describe the Go backend conventions for the BUPT empty-classroom
query service. They are source-backed rules for future AI coding sessions; keep
them synchronized with real code, tests, docs, and CI whenever the project
architecture changes.

## Project Shape

The backend is a small Go 1.25.12+ service built around Gin handlers and a single
`service.ClassroomService` instance. It does not store classroom timetables in a
database. Runtime data comes from the BUPT JW HTTP API, is normalized into
`service/model.TodayClassrooms`, and is cached in process for the current day.

The React/Vite frontend is embedded into the Go binary from `frontend/dist/` by
`router.go`; API contracts therefore need to account for both backend model
types and frontend consumers.

## Guidelines Index

| Guide | Description | Status |
| --- | --- | --- |
| [Directory Structure](./directory-structure.md) | Backend package ownership, entry points, and where new code belongs | Source-backed |
| [Runtime State and Cache](./runtime-state-and-cache.md) | No-database architecture, `ClassroomService` state, cache, refresh, and JW token rules | Source-backed |
| [API Response Contract](./api-contract.md) | `/api/get_data`, health endpoints, SPA fallback, and frontend-facing JSON shape | Source-backed |
| [Error Handling](./error-handling.md) | JW error classification, safe user messages, stale data errors, and API failures | Source-backed |
| [Logging Guidelines](./logging-guidelines.md) | `log/slog`, `log_id` propagation, log outputs, and secret redaction rules | Source-backed |
| [Quality Guidelines](./quality-guidelines.md) | Formatting, tests, CI commands, release hygiene, and review checklist | Source-backed |

## Non-Negotiable Local Rules

- Keep mutable service runtime state on `service.ClassroomService`; do not add
  package-level mutable globals inside `service/`.
- Do not introduce a local timetable database unless the product explicitly asks
  for an architecture change.
- Do not expose raw JW errors, credentials, tokens, or upstream response bodies
  to API clients or logs.
- Run Go quality checks after backend changes: `gofmt`, `go vet ./...`, and the
  relevant `go test` command. CI runs `go test -race ./...`.
- If a user-visible behavior, endpoint, config, or release asset changes, update
  `README.md`/`docs/` and add a `CHANGELOG.md` `[Unreleased]` bullet in the same
  change.
