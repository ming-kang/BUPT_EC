# Development

Local development setup, testing, and a tour of the architecture.

## Requirements

- Go 1.26.9+ (per `go.mod`)
- Node.js 22 LTS
- pnpm 9.15.x — `corepack enable && corepack prepare pnpm@9.15.0 --activate`
- A valid BUPT teaching affairs account (only for integration tests and running against the real JW system; unit tests run without one)

## Configuration

Create an optional `.env` in the repository root from `.env.example`, or export the same values in the process environment:

```bash
JW_USERNAME=your_username
JW_PASSWORD=your_password
# Optional listen address (default 127.0.0.1:8080 when unset).
APP_ADDR=127.0.0.1:8080
# Optional: include caller source in structured logs.
# LOG_CALLER=1
```

Startup reads configuration once. Process environment values override `.env`; a missing `.env` is allowed, while a present malformed or unreadable file fails safely without printing its contents. Startup requires `JW_USERNAME`/`JW_PASSWORD`, validates `APP_ADDR`, and applies the log settings before constructing dependencies. Changes require a process restart. Never commit real credentials.

## Run locally

The `web/` package embeds the frontend only when built with `-tags embed_assets` (which expects `frontend/dist` copied to `web/dist`). Without the tag — the default for `go run ./`, `go test`, and bare `go build` — the backend compiles on a clean checkout and serves a small placeholder page instead of the UI. `task build` ([Task](https://taskfile.dev)) runs the whole chain:

```bash
task build   # pnpm build → copy frontend/dist to web/dist → go build -tags embed_assets
./bupt-ec    # bupt-ec.exe on Windows
# open http://127.0.0.1:8080/
```

Without `task`, the equivalent is:

```bash
cd frontend && pnpm install && pnpm build && cd ..
rm -rf web/dist && cp -r frontend/dist web/dist
go build -tags embed_assets -o bupt-ec ./
```

This omits the version injection: `task build` also passes `-trimpath -ldflags "-s -w -X main.version=<git describe>"`, so a binary built with the bare command above reports `version: dev` in the startup log and on `/readyz`.

For frontend work with hot reload, run both dev servers (the backend only needs to answer `/api`, so plain `go run ./` is fine here):

```bash
go run ./                # terminal 1: backend on 127.0.0.1:8080
cd frontend && pnpm dev  # terminal 2: Vite dev server, proxies /api to localhost:8080
```

## Tests and checks

```bash
task test                 # go test -race ./...
task installer:check      # installer syntax, behavior, release layout, ShellCheck
task check                # format/vet/dependencies + frontend/installer checks
task vuln                 # govulncheck (needs network; same pinned version as CI)
```

Native equivalents, for environments without `task`:

```bash
go test ./...              # unit tests always run; integration tests skip
go test -race ./...        # what CI and `task test` run
go vet ./...
gofmt -l .                 # must print nothing
GOTOOLCHAIN=go1.26.9 go mod tidy -diff   # go.mod/go.sum match the import graph
go mod verify
find scripts -type f -name '*.sh' -exec bash -c 'for script; do bash -n "$script" || exit 1; done' bash {} +
bash scripts/install_test.sh
bash scripts/release_layout_test.sh
find scripts -type f -name '*.sh' -exec shellcheck {} +
cd frontend && pnpm lint && pnpm test && pnpm build
cd frontend && pnpm size    # gzip budget over dist/, CI runs it after the build
cd frontend && pnpm audit:prod && pnpm audit:dev
```

The reusable quality gate (`.github/workflows/quality.yml`) runs the same
checks, including recursive shell syntax, installer behavior tests, local release-layout/version-injection simulation, and
recursive ShellCheck, plus an embedded-assets
build (`go build -tags embed_assets` after copying the freshly built frontend
to `web/dist`). The other Go steps stay tag-less so a clean checkout without
`frontend/dist` keeps building. `task check` includes `task installer:check`
and frontend type-checking; run `task test` and `task build` separately for Go
race tests and the production build. Run `pnpm size` after a fresh frontend
build when changing dependencies or chunk splitting.

`scripts/install.sh` is the single hand-maintained, self-contained installer
used by `curl | sudo bash` and packaged into release tarballs.
`scripts/compose-release-assets.sh` locks the exact release layout. Installer
tests live in `scripts/installer_test/` (entry point `scripts/install_test.sh`)
and `scripts/release_layout_test.sh`; all are repository-only.

`BUNDLE_REPORT=1 pnpm build` additionally writes a treemap report to
`frontend/bundle-stats.local.html` (gitignored, never inside `dist/`).

Integration tests in `service/integration_test.go` hit the real JW system and compile only with `go test -tags integration ./service`. They require `JW_USERNAME`/`JW_PASSWORD` and skip with a clear message without them.

Unit tests never touch the network: they inject a `mockJWClient` (implementing the `JWClient` interface) into a fresh `ClassroomService` on a fake clock — see `newTestService` and `startRun` in `service/testsupport_test.go`. Refresh behavior is tested by calling `refresh` directly; loop behavior by running `Run` against the mock.

Frontend behavior tests use Vitest and focus on response decoding (`api/decode.test.ts`), the reload schedule, selection state, preference persistence, and a jsdom lifecycle harness for `useTodayClassrooms` (`src/*.lifecycle.test.tsx`, via `@testing-library/react`). Pure helper tests stay on the default `node` environment. Run them with `pnpm test` from `frontend/`. `pnpm audit:prod` rejects moderate-or-higher production dependency advisories; `pnpm audit:dev` rejects high-or-critical findings across the full frontend toolchain.

Backend protocol tests cover AES password encryption known vectors (`service/crypto_test.go`) and offline JWClient request/response fixtures (`service/jw_protocol_test.go`). Real JW integration tests still skip cleanly without credentials.

## Project structure

```text
main.go, router.go, handler.go            HTTP entry points; main.go wires ClassroomService into HTTPServer
web/                     embedded frontend assets (real dist with -tags embed_assets, placeholder otherwise)
service/
  classroom_service.go   ClassroomService: snapshot cache, request path, refresh loop, token state
  jw_client.go           JWClient interface + defaultJWClient (HTTP protocol layer)
  classroom_builder.go   JW rows → campuses/buildings/rooms normalization
  jw_error.go            error classification, SafeErrorMessage, upstream message sanitizer
  jw_http.go             JW outbound HTTP transport (redirect rejection, body limits)
  crypto.go              AES password encryption for the JW login protocol
  urlutil.go             JW API URL validation and building
  model/                 JSON payload shapes
config/                  immutable startup snapshot + dotenv/env validation
logs/                    slog JSON setup + per-request log_id context
frontend/src/            React app in strict TypeScript (Vite + Ant Design)
  api/types.ts           typed mirror of the Go public API structs (update both)
  api/decode.ts          validating decoder for /api/get_data + snapshot usability rule
  api/getData.ts         fetch boundary (timeout, typed ApiError)
  selectionContext.ts    useSelection() context value
  SelectionProvider.tsx  selection state with persisted preferences
  useTodayClassrooms.ts  polling hook driven by reloadSchedule.ts
  components/            UI components (.tsx; pickers, table, modal, ErrorBoundary)
scripts/
  install.sh                   self-contained installer (single source)
  installer_test/, install_test.sh  installer behavior tests (repository-only)
  compose-release-assets.sh    exact release tarball layout
  release_layout_test.sh       stable/main package simulation
  release.sh, extract-changelog.sh
.github/workflows/       ci.yml (PRs), release.yml (v* tags + manual dispatch), quality.yml (reusable gate)
```

## Backend architecture

There is one public API endpoint, `GET /api/get_data`, plus `/healthz` and `/readyz`. The backend keeps no timetable database — it queries the BUPT JW system on demand:

1. `https://jwglweixin.bupt.edu.cn/sjd/serverconfig.json` resolves the live API base URL (validated; falls back to a default on failure).
2. `POST <api>/login` performs an AES-encrypted password login and yields a token, held in memory only.
3. `POST <api>/todayClassrooms?campusId=01|04` fetches classroom rows for Xitucheng (`01`) and Shahe (`04`).

`main.go` is the composition root: it calls `config.Load`, initializes logging, builds `service.NewJWClient` and `service.NewClassroomService`, starts `Run` in a goroutine, and serves `NewHTTPServer(service).Routes()`.

- **`JWClient`** (`jw_client.go`) is the stateless protocol layer — build request, call HTTP, parse and classify the response. Tests substitute `mockJWClient` or a fake `HTTPDoer`.
- **Refresh loop** (`ClassroomService.Run`) is the only JW caller and the sole owner of the API URL and token (plain fields, no locking). Each refresh queries all campuses in parallel; campuses rejected for authentication trigger one fresh login and one retry. At least one successful campus publishes a new snapshot (failed campuses keep prior same-day data, listed in `partial_campuses` with a top-level `error`); all campuses failing keeps the old snapshot and records the error. The loop then sleeps until the next midnight (full), 5 minutes (partial), or the 30s/1m/2m/5m failure ladder, and wakes early when a request nudges it outside the backoff window (5 minutes after full, 30s after partial, the ladder step after failure).
- **Request path** (`GetTodayClassrooms`) only reads the snapshot: fresh data is returned as is (a partial snapshot also nudges the loop); stale data is copied with `stale: true` and the last failure message; with no usable snapshot the request nudges the loop and waits up to `ColdWaitTimeout` (5s) for the next attempt, failing fast with the last error during backoff.
- **Topology**: production is a single process-local instance; horizontal multi-app
  replicas are unsupported. See [operations.md](operations.md#deployment-topology-supported-today).
- **Caching**: one immutable `*model.TodayClassrooms` in an `atomic.Pointer`; fresh for 5 minutes, then served stale until the next **Asia/Shanghai** midnight. Dates and TTLs are stamped at refresh **completion**; cross-day reuse is rejected. Canceling the context passed to `Run` stops the loop and any in-flight JW request. See [operations.md](operations.md#caching-behavior) for the operator view.
- Rooms like `教学实验综合楼-N104(229)` and merged rooms like `未来学习大楼-202-203(60)` are parsed in `classroom_builder.go`.
- Outbound JW HTTP (`service/jw_http.go`) does not follow redirects (custom `token` / login bodies must not leave the intended host). Default `APP_ADDR` is loopback (`127.0.0.1:8080`). Cold-start requests wait at most 5s (`service.ColdWaitTimeout`) for the refresh loop and then return `503` + `Retry-After: 5` while the refresh continues in the background; HTTP `WriteTimeout` (15s) keeps headroom over that bound.

Logging is `log/slog` with a JSON handler; `LOG_CALLER` is resolved by `config.Load` and passed to `logs.Init`, and a custom wrapper adds the per-request `log_id` from the context to every record (`logs/`).

## Frontend architecture

- `useTodayClassrooms.ts` fetches `/api/get_data` through `api/getData.ts` with a single `setTimeout` chain whose delay comes from the pure `nextReloadDelay` in `reloadSchedule.ts`: near `expires_at` when fully fresh (1s floor), ≥15s for stale data, ≥30s for partial data, and 10s/20s/30s/60s backoff after client failures. One positive-only jitter sample (≤10% of base, cap 5s) desyncs tabs, and the result is clamped to `stale_until`. The last good snapshot survives failures but is dropped once its date is no longer the Shanghai business day or `stale_until` has passed. Hidden tabs issue no requests; becoming visible or focused triggers one reload (throttled to 15s for the Nginx rate limit). Requests never overlap, are aborted on unmount, and background reloads never show the full-page spinner.
- `api/decode.ts` validates every field the UI reads before anything renders; malformed payloads become a typed `ApiError`. `api/getData.ts` applies the request timeout with `AbortController` + `setTimeout` and maps transport failures to `ApiError` with a safe message. Class-period “now” and “today” use Asia/Shanghai to match the backend business day.
- Selection state (campus, buildings, class times, display preferences) is plain `useState` in `SelectionProvider`, exposed through `useSelection()`; setters persist preferences to `localStorage`, and changing campus clears buildings and class times.
- `main.tsx` wraps `<App />` in a last-resort `ErrorBoundary` whose fallback is plain inline-styled HTML on purpose: a crash can originate inside `ConfigProvider`/antd itself, so the root fallback must not depend on them.
- The classroom table is lazy-loaded behind `Suspense` and its own inner `ErrorBoundary`, so local degradation is handled before the root boundary takes over.
- Dark mode follows system `prefers-color-scheme` only (bootstrap + React share that source; no conflicting `localStorage.darkMode`).

## Conventions

- Format Go code with `gofmt`; imports use the `BUPT_EC/...` module prefix.
- React components are PascalCase `.tsx` files with a matching `.css` beside them when needed. The frontend is strict TypeScript (`pnpm typecheck`, enforced by Taskfile check and CI); API payload shapes are typed in `src/api/types.ts`, mirrored from `service/model/realtime_data.go` — update both sides together on contract changes. ESLint enforces the configured rules (`pnpm lint`).
- Commits follow Conventional Commits (`feat:`, `fix:`, `chore:`, `ci:`, `docs:`, `refactor:`); keep each commit scoped to one change.
- User-visible changes should update the `[Unreleased]` section of [CHANGELOG.md](../CHANGELOG.md) in the same commit — see [release.md](release.md).

## Maintenance contracts

These are the implementation constraints most likely to be lost during refactors.

| Boundary | Preserve |
| --- | --- |
| Cache publication | `refresh` in [classroom_service.go](../service/classroom_service.go) publishes one immutable snapshot through `atomic.Pointer`; copy before decorating responses (stale flag/error). Only the `Run` goroutine touches token/API URL state. |
| API envelope | `/api/get_data` returns `code: 0` with HTTP 200 for usable full, stale, or partial data; failures use HTTP 503 and `data: null`. Both carry `version` and request-specific `log_id` outside `data`; the ID matches `X-Log-Id`. Only warming/no-cache wait failures get `Retry-After: 5`. Future response caching must account for that per-request ID. See [handler.go](../handler.go). |
| Classroom identity | Building `name` is the selection/filter key; `display_name` is a label. A room matches only when **all** selected periods occur in `free_nodes`. `nodes` belongs to each campus; degraded room parsing may produce capacity `0` (display as unknown). See [classroom_builder.go](../service/classroom_builder.go) and [TodayClassroomTable.tsx](../frontend/src/components/TodayClassroomTable.tsx). |
| JW boundary | Keep HTTPS/BUPT-host validation and redirect rejection. Auth rejection triggers one fresh login and one retry per refresh. `safeRemoteMessage` normalizes Unicode controls/whitespace, redacts secrets, then caps at 256 runes; client messages still come only from `SafeErrorMessage`. See [jw_error.go](../service/jw_error.go). |
| HTTP/static boundary | Unknown `/api/*` and missing assets return real 404s. Keep gzhttp's explicit content-type allowlist and single compression layer; `/healthz` and `/readyz` bypass compression. Hashed assets are immutable; SPA HTML uses `no-cache` and a weak ETag. See [router.go](../router.go) and [operations.md](operations.md#static-asset-caching-spa). |

Frontend compatibility:

- Keep the explicit ES2020/Safari 14 build target; transpilation does not polyfill browser APIs. Vite 7 pairs with plugin-react 5. React 19 with antd 5 needs `@ant-design/v5-patch-for-react-19` in both `main.tsx` and jsdom test setup. Keep the classroom table native to protect the bundle budget.
- The reload scheduler never overlaps requests and always arms a positive delay. Hook tests stub `fetch` and use fake timers. Use `AbortController` + `setTimeout` for timeouts (Safari 14 lacks `AbortSignal.timeout`). Snapshot dates and deadlines always use the Shanghai business day.

Installer/release maintenance:

- Keep `DEPLOYMENT_CONFIG_KEYS` the single registry of persisted settings. Rendering consumes validated `CFG_*`; one-shot checksum/insecure-download switches must never be persisted. Private env stays root-owned `0600`.
- Preserve download/checksum → staging → persistent journal snapshot (`/var/lib/bupt-ec/transaction`, `snapshot.done`) → replacement → activation/health validation → `committed` → journal removal. Rollback restores every snapshot target literally (files and enablement links alike); `recover_interrupted_transaction` runs first on every installer start.
- Keep Installer self-contained and bootstrap the current Installer even for an older target archive. Version/mirror rules and exact release layout are in [upgrading.md](upgrading.md) and [release.md](release.md). Check version injection with a real built binary's `/readyz` or startup log; `go test -ldflags "-X main.version=..."` and `go version -m` cannot verify it.
