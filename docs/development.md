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
# Optional debug fallback only. Leave empty for automatic HTTP login.
JW_TOKEN=
# Optional listen address (default 127.0.0.1:8080 when unset).
APP_ADDR=127.0.0.1:8080
# Optional: include caller source in structured logs.
# LOG_CALLER=1
```

Startup reads configuration once. Process environment values override `.env`; a missing `.env` is allowed, while a present malformed or unreadable file fails safely without printing its contents. Startup validates credentials and `APP_ADDR`, then applies `APP_ADDR` and the log settings before constructing dependencies. Changes require a process restart. Never commit real credentials.

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
task installer:generate   # regenerate tracked scripts/install.sh from fragments
task installer:check      # Installer/CLI syntax, behavior, release-layout, ShellCheck
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
bash scripts/generate-install.sh --check
find scripts -type f -name '*.sh' -exec bash -c 'for script; do bash -n "$script" || exit 1; done' bash {} +
bash scripts/install_test.sh
bash scripts/cli_test.sh
bash scripts/release_layout_test.sh
find scripts -type f -name '*.sh' -exec shellcheck {} +
cd frontend && pnpm lint && pnpm test && pnpm build
cd frontend && pnpm size    # gzip budget over dist/, CI runs it after the build
cd frontend && pnpm audit:prod && pnpm audit:dev
```

The reusable quality gate (`.github/workflows/quality.yml`) runs the same
checks, including generated-installer drift, recursive shell syntax, Installer
and CLI behavior tests, local release-layout/version-injection simulation, and
recursive ShellCheck, plus an embedded-assets
build (`go build -tags embed_assets` after copying the freshly built frontend
to `web/dist`). The other Go steps stay tag-less so a clean checkout without
`frontend/dist` keeps building. `task check` includes `task installer:check`
and frontend type-checking; run `task test` and `task build` separately for Go
race tests and the production build. Run `pnpm size` after a fresh frontend
build when changing dependencies or chunk splitting.

Installer source is modular in `scripts/installer/`, while
`scripts/generate-install.sh` emits the deterministic tracked and
self-contained `scripts/install.sh` artifact used by tests and releases. Edit
fragments, run `task installer:generate`, then run `task installer:check`; do
not hand-edit the generated artifact. `scripts/bupt-ec-cli.sh` is an independent
thin operations product packaged into each tarball; it must not become an
Installer runtime dependency. `scripts/compose-release-assets.sh` injects its
package-only version marker and locks the exact layout. Test modules live in
`scripts/installer_test/`, `scripts/cli_test.sh`, and
`scripts/release_layout_test.sh`; all are repository-only.

`BUNDLE_REPORT=1 pnpm build` additionally writes a treemap report to
`frontend/bundle-stats.local.html` (gitignored, never inside `dist/`).

Integration tests in `service/integration_test.go` hit the real JW system and compile only with `go test -tags integration ./service`. `TestLogin` requires `JW_USERNAME`/`JW_PASSWORD`; `TestQueryOne` and `TestQueryAll` accept that pair or `JW_TOKEN`. Without the required credentials they skip with a clear message.

Unit tests never touch the network: they inject a `mockJWClient` (implementing the `JWClient` interface) into a fresh `ClassroomService`, which owns its own in-memory day cache — see `newTestService` in `service/testsupport_test.go`.

Frontend behavior tests use Vitest and focus on API envelope normalization, selection-state transitions, preference persistence, and a jsdom lifecycle harness for `useTodayClassrooms` (`src/*.lifecycle.test.tsx`, via `@testing-library/react`). Pure helper tests stay on the default `node` environment. Run them with `pnpm test` from `frontend/`. `pnpm audit:prod` rejects moderate-or-higher production dependency advisories; `pnpm audit:dev` rejects high-or-critical findings across the full frontend toolchain.

Backend protocol tests cover AES password encryption known vectors (`service/crypto_test.go`) and offline JWClient request/response fixtures (`service/jw_protocol_test.go`). Real JW integration tests still skip cleanly without credentials.

## Project structure

```text
main.go, router.go, handler.go            HTTP entry points; main.go wires ClassroomService into HTTPServer
web/                     embedded frontend assets (real dist with -tags embed_assets, placeholder otherwise)
service/
  classroom_service.go   ClassroomService struct, atomic day-cache slot, Clock, constructor
  realtime_data.go       public API: GetTodayClassrooms, refresh data flow
  jw_client.go           JWClient interface + defaultJWClient (HTTP protocol layer)
  token_manager.go       TokenManager: token/API-URL caching with singleflight
  refresh_coordinator.go single-flight refresh attempts and backoff
  warmup.go            cancellable startup/midnight scheduler + background drain
  runtime_status.go      RuntimeStatus for /readyz
  classroom_builder.go   JW rows → campuses/buildings/rooms normalization
  jw_error.go            error classification + SafeErrorMessage + Unicode-safe upstream sanitizer
  jw_http.go             JW outbound HTTP transport (redirect rejection, body limits)
  crypto.go              AES password encryption for the JW login protocol
  urlutil.go             JW API URL validation and building
  model/                 JSON payload shapes
config/                  immutable startup snapshot + dotenv/env validation
logs/                    slog JSON setup + per-request log_id context
frontend/src/            React app in strict TypeScript (Vite + Ant Design)
  api/types.ts           typed mirror of the Go public API structs (update both)
  selectionContext.ts    selection state: reducer + useSelection hook
  SelectionProvider.tsx  context provider
  useTodayClassrooms.ts  SWR data fetching + auto-refresh on expires_at
  todayClassroomsResponse.ts  API envelope normalization helpers
  components/            UI components (.tsx; pickers, table, modal, ErrorBoundary)
scripts/
  generate-install.sh          deterministic generator for tracked install.sh
  installer/                   ordered installer source fragments
  install.sh                   generated self-contained release artifact
  bupt-ec-cli.sh               packaged thin operations CLI source
  installer_test/              Installer behavior-test modules (repository-only)
  install_test.sh, cli_test.sh Installer and CLI behavior entrypoints
  compose-release-assets.sh    exact tar layout + package-only CLI injection
  release_layout_test.sh       stable/main package simulation
  release.sh, extract-changelog.sh
.github/workflows/       ci.yml (PRs), release.yml (main pushes + tags), quality.yml (reusable gate)
```

## Backend architecture

There is one public API endpoint, `GET /api/get_data`, plus `/healthz` and `/readyz`. The backend keeps no timetable database — it queries the BUPT JW system on demand:

1. `https://jwglweixin.bupt.edu.cn/sjd/serverconfig.json` resolves the live API base URL (validated; falls back to a default on failure).
2. `POST <api>/login` performs an AES-encrypted password login and yields a token, held in memory only.
3. `POST <api>/todayClassrooms?campusId=01|04` fetches classroom rows for Xitucheng (`01`) and Shahe (`04`).

All classroom-query runtime state lives on the `ClassroomService` struct. `main.go::Init` is the sole production composition root: it calls `config.Load`, applies the log settings, constructs `service.NewJWHTTPClient()`, `service.NewJWClient`, and `service.NewClassroomService`, then injects the resulting service into `NewHTTPServer` before route registration:

- **`JWClient`** (`jw_client.go`) is the stateless protocol layer — build request, call HTTP, parse and classify the response. `NewJWClient` receives immutable username/password values plus an explicit `service.HTTPDoer`; tests substitute `mockJWClient` or a fake doer.
- **`TokenManager`** (`token_manager.go`) caches the token and API URL, records whether the current token came from the startup `JW_TOKEN` snapshot or login, and deduplicates login/API-URL work with `singleflight.DoChan`. Each shared operation has its own bounded context detached from the first waiter's cancellation, while every caller can still stop waiting through its own context. On an auth failure, `RefreshAfterAuthFailure` rechecks the failed token inside singleflight: a delayed request reuses any newer token instead of logging in again. The injected override is invalidated only when that actual override token is rejected; expiration of a login-issued token does not change override state.
- **Refresh coordination** (`refresh_coordinator.go`) ensures at most one refresh runs at a time; concurrent requests wait on the same attempt. Internal outcomes explicitly distinguish full success, partial success, and total failure. Partial outcomes set a fixed 30-second soft backoff; consecutive total failures use a 30s → 1m → 2m → 5m base ladder with bounded injectable jitter (±10% of base, absolute cap ±5s). Full success clears the ladder. Partial results are cached with `partial_campuses`, a safe top-level `error`, and prior same-day data for failed campuses when available. Partial payloads still inside the fresh TTL trigger soft-stale revalidation (return data + background refresh) instead of waiting the full 5 minutes. A newer total failure overrides an older partial warning on stale responses. Business time comes from the optional injected `ClassroomServiceOptions.Clock` (shared with `TokenManager`); tests use a thread-safe fake clock rather than replacing an internal `now` function.
- **Topology**: production is a single process-local instance; horizontal multi-app
  replicas are unsupported. See [operations.md](operations.md#deployment-topology-supported-today).
- **Caching**: `ClassroomService` holds today's normalized payload in a single in-memory slot (`atomic.Pointer`; no cache library, no store key) — fully successful data is fresh for ~5 minutes, then served stale until the next **Asia/Shanghai** midnight while refreshes happen in the background. Cache `date` / TTLs are stamped at refresh **completion**. Cross-day reuse is rejected. The context-cancellable warmup scheduler runs immediately at startup, retries a missing cache with a 30s/1m/2m/5m cap, retries partial cache no faster than the fresh TTL, and schedules a new-day refresh after Shanghai midnight plus a small jitter. `main.go` cancels the service lifecycle before HTTP shutdown and calls
`Shutdown` only after handlers are drained; the lifecycle cancel also stops
in-flight JW refreshes/logins promptly. See [operations.md](operations.md#caching-behavior) for the operator view.
- Rooms like `教学实验综合楼-N104(229)` and merged rooms like `未来学习大楼-202-203(60)` are parsed in `classroom_builder.go`.
- Outbound JW HTTP (`service/jw_http.go`) does not follow redirects (custom `token` / login bodies must not leave the intended host). Default `APP_ADDR` is loopback (`127.0.0.1:8080`). Cold-start requests wait at most 5s (`service.DefaultColdWaitTimeout`) for the in-flight refresh and then return `503` + `Retry-After: 5` while the refresh continues in the background; HTTP `WriteTimeout` (15s) keeps headroom over that bound.

Logging is `log/slog` with a JSON handler; `LOG_CALLER` is resolved by `config.Load` and passed to `logs.Init`, and a custom wrapper adds the per-request `log_id` from the context to every record (`logs/`).

## Frontend architecture

- `useTodayClassrooms.ts` fetches `/api/get_data` through `swr` and schedules automatic reloads via pure helpers in `reloadSchedule.ts`: near `expires_at` when fully fresh (1s floor), ≥15s for ordinary stale data, ≥30s for partial-campus data, and 10s/20s/30s/60s client-failure backoff. SWR's `refreshInterval` drives the poll (through a never-falsy wrapper — a falsy interval would end the polling chain permanently) and a custom `onErrorRetry` drives the backoff, so the same pure schedule covers both paths. Each schedule samples `random` once, applies **positive-only** bounded jitter (≤10% of base, cap 5s), then clamps to remaining `stale_until` so the final delay never exceeds the hard display deadline. A snapshot is kept only while its date matches the Shanghai business day and `stale_until` remains in the future; failures keep the last good snapshot because SWR stores data and errors on separate tracks and the hook merges them during render. Hidden tabs issue no requests (timers may re-arm, but polls are skipped and an armed retry gives up when it wakes hidden); resume after expiry clears stale campuses before a single background reload. Background polls do **not** full-page spin.
- `todayClassroomsResponse.ts` normalizes backend envelopes before UI code reads them. Class-period “now” and “today” use Asia/Shanghai to match the backend business day.
- Selection state (campus, buildings, class times, display preferences) lives in a `useReducer` store exposed through `SelectionProvider` / `useSelection()`; preferences persist to `localStorage` in the reducer.
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
| Cache publication | `publishTodayCache` in [classroom_service.go](../service/classroom_service.go) publishes the model and pre-serialized JSON together through one atomic entry. Both remain immutable; copy before decorating responses. The fast path is only for fresh, complete, error-free same-day data. |
| API envelope | `/api/get_data` returns `code: 0` with HTTP 200 for usable full, stale, or partial data; failures use HTTP 503 and `data: null`. Both carry `version` and request-specific `log_id` outside `data`; the ID matches `X-Log-Id`. Only warming/no-cache wait failures get `Retry-After: 5`. Future response caching must account for that per-request ID. See [handler.go](../handler.go). |
| Classroom identity | Building `name` is the selection/filter key; `display_name` is a label. A room matches only when **all** selected periods occur in `free_nodes`. `nodes` belongs to each campus; degraded room parsing may produce capacity `0` (display as unknown). See [classroom_builder.go](../service/classroom_builder.go) and [TodayClassroomTable.tsx](../frontend/src/components/TodayClassroomTable.tsx). |
| JW boundary | Keep HTTPS/BUPT-host validation and redirect rejection. Auth recovery rechecks the rejected token inside singleflight and retries the query once; a canceled waiter must not cancel shared work. `safeRemoteMessage` normalizes Unicode controls/whitespace, redacts secrets, then caps at 256 runes; client messages still come only from `SafeErrorMessage`. See [jw_error.go](../service/jw_error.go). |
| HTTP/static boundary | Unknown `/api/*` and missing assets return real 404s. Keep gzhttp's explicit content-type allowlist and single compression layer; `/healthz` and `/readyz` bypass compression. Hashed assets are immutable; SPA HTML uses `no-cache` and a weak ETag. See [router.go](../router.go) and [operations.md](operations.md#static-asset-caching-spa). |

Frontend compatibility:

- Keep the explicit ES2020/Safari 14 build target; transpilation does not polyfill browser APIs. Vite 7 pairs with plugin-react 5. React 19 with antd 5 needs `@ant-design/v5-patch-for-react-19` in both `main.tsx` and jsdom test setup. Keep the classroom table native to protect the bundle budget.
- SWR's polling interval function must have stable identity and return a positive delay; a falsy value stops the polling chain. Give each hook test a fresh `SWRConfig` cache and retain production deduping. Fake timers need an explicit timeout-signal stub. Snapshot dates and deadlines always use the Shanghai business day.

Installer/release maintenance:

- Keep `DEPLOYMENT_CONFIG_KEYS` and the CLI's private-config registry aligned, including `LOG_CALLER` / `READYZ_DIAGNOSTICS`. Rendering consumes validated `CFG_*`; one-shot checksum/insecure-download switches must never be persisted. Private env stays root-owned `0600`; public metadata is `0644` and contains only `RELEASE_VERSION` / `APP_ADDR`.
- Preserve download/checksum → staging → snapshot → same-filesystem replacement → activation/health validation. `transaction_targets` owns the eight file/link roles for both commit and rollback; restore prior existence, enablement, and activity. Keep failed-rollback recovery files. Health retries suppress individual curl errors and report failure only after exhaustion.
- Keep Installer self-contained and bootstrap the current Installer even for an older target archive. Version/mirror rules and exact release layout are in [upgrading.md](upgrading.md) and [release.md](release.md). Check version injection with a real built binary's `/readyz` or startup log; `go test -ldflags "-X main.version=..."` and `go version -m` cannot verify it.

## Known follow-ups

Remaining findings checked against source on 2026-09-08; scope them separately when working in the affected area:

- [useTodayClassrooms.ts](../frontend/src/useTodayClassrooms.ts): `AbortSignal.timeout` lacks a Safari 14 fallback, and generic transport errors bypass `ApiError`. Add a compatible timeout boundary and safe errors; also correct the stale 45s server-timeout comment against the [timeout budget](operations.md#timeout-budget-api-cold-path).
- [todayClassroomsResponse.ts](../frontend/src/todayClassroomsResponse.ts): nested campus/building/room data is cast after shallow validation; e.g. `campuses: [null]` passes the boundary. Validate fields used by rendering in one decoder.
- [warmup.go](../service/warmup.go): `Shutdown` before `Run` leaves refresh-worker admission open while waiting. Close admission under `backgroundMu` independently of whether a lifecycle context exists.
- [40-transaction.sh](../scripts/installer/40-transaction.sh): temporary backups and an `EXIT` trap handle returned failures, but provide no durable recovery after SIGKILL/power loss. A future recovery change needs a persistent journal and interruption tests.
- The same rollback code treats a missing unit file as a first install, which can delete a pre-existing enablement link restored from the snapshot. Preserve those states independently and add a damaged-installation fixture.
