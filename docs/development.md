# Development

Local development setup, testing, and a tour of the architecture.

## Requirements

- Go 1.25.12+ (per `go.mod`); Go 1.26 users need 1.26.5 or newer
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
# Gin runtime mode. Use release in production.
GIN_MODE=debug
# Optional: include caller source in structured logs.
# LOG_CALLER=1
```

Startup reads configuration once. Process environment values override `.env`; a missing `.env` is allowed, while a present malformed or unreadable file fails safely without printing its contents. Startup validates credentials, `GIN_MODE`, and `APP_ADDR`, then applies Gin/log settings before constructing dependencies. Changes require a process restart. Never commit real credentials.

## Run locally

The Go binary embeds `frontend/dist` (`//go:embed` in `router.go`), so build the frontend once before running or building the backend:

```bash
cd frontend
pnpm install
pnpm build
cd ..
go run ./
# open http://127.0.0.1:8080/
```

For frontend work with hot reload, run both dev servers:

```bash
go run ./                # terminal 1: backend on 127.0.0.1:8080
cd frontend && pnpm dev  # terminal 2: Vite dev server, proxies /api to localhost:8080
```

## Tests and checks

```bash
go test ./...              # unit tests always run; integration tests skip
go test -race ./...        # what CI runs
go vet ./...
gofmt -l .                 # must print nothing
cd frontend && pnpm lint && pnpm test && pnpm build
cd frontend && pnpm audit:prod && pnpm audit:dev
```

Integration tests in `service/realtime_data_test.go` hit the real JW system. `TestLogin` requires `JW_USERNAME`/`JW_PASSWORD`; `TestQueryOne` and `TestQueryAll` accept that pair or `JW_TOKEN`. Without the required credentials they skip with a clear message.

Unit tests never touch the network: they inject a `mockJWClient` (implementing the `JWClient` interface) into a fresh `ClassroomService` with an isolated cache per test — see `newTestService` in `service/realtime_data_test.go`.

Frontend behavior tests use Vitest and focus on API envelope normalization, selection-state transitions, preference persistence, and a jsdom lifecycle harness for `useTodayClassrooms` (`src/*.lifecycle.test.jsx`, via `@testing-library/react`). Pure helper tests stay on the default `node` environment. Run them with `pnpm test` from `frontend/`. `pnpm audit:prod` rejects moderate-or-higher production dependency advisories; `pnpm audit:dev` rejects high-or-critical findings across the full frontend toolchain.

Backend protocol tests cover AES password encryption known vectors (`service/crypto_test.go`) and offline JWClient request/response fixtures (`service/jw_protocol_test.go`). Real JW integration tests still skip cleanly without credentials.

## Project structure

```text
main.go, router.go, handler.go            Gin entry points; main.go wires ClassroomService into HTTPServer
service/
  classroom_service.go   ClassroomService struct, CacheStore interface, constructor
  realtime_data.go       public API: GetTodayClassrooms, QueryOne/All, refresh data flow
  jw_client.go           JWClient interface + defaultJWClient (HTTP protocol layer)
  token_manager.go       TokenManager: token/API-URL caching with singleflight
  refresh_coordinator.go single-flight refresh attempts and backoff
  warmup.go            cancellable startup/midnight scheduler + background drain
  runtime_status.go      RuntimeStatus for /readyz
  classroom_builder.go   JW rows → campuses/buildings/rooms normalization
  jw_error.go            error classification (auth/config/query/parse) + safe messages
  crypto.go              AES password encryption for the JW login protocol
  urlutil.go             JW API URL validation and building
  model/                 JSON payload shapes
cache/                   explicit go-cache constructor (5-minute default TTL)
config/                  immutable startup snapshot + dotenv/env validation
logs/                    slog JSON setup + per-request log_id context
utils/                   HTTP helpers
frontend/src/            React app (Vite + Ant Design)
  selectionContext.js    selection state: reducer + useSelection hook
  SelectionProvider.jsx  context provider
  useTodayClassrooms.js  data fetching + auto-refresh on expires_at
  todayClassroomsResponse.js  API envelope normalization helpers
  components/            UI components (pickers, table, modal, ErrorBoundary)
scripts/                 install.sh, release.sh, extract-changelog.sh
.github/workflows/       ci.yml (PRs), release.yml (main pushes + tags)
```

## Backend architecture

There is one public API endpoint, `GET /api/get_data`, plus `/healthz` and `/readyz`. The backend keeps no timetable database — it queries the BUPT JW system on demand:

1. `https://jwglweixin.bupt.edu.cn/sjd/serverconfig.json` resolves the live API base URL (validated; falls back to a default on failure).
2. `POST <api>/login` performs an AES-encrypted password login and yields a token, held in memory only.
3. `POST <api>/todayClassrooms?campusId=01|04` fetches classroom rows for Xitucheng (`01`) and Shahe (`04`).

All classroom-query runtime state lives on the `ClassroomService` struct. `main.go::Init` is the sole production composition root: it calls `config.Load`, applies Gin/log settings, constructs `cache.New()`, `utils.NewHTTPClient()`, `service.NewJWClient`, and `service.NewClassroomService`, then injects the resulting service into `NewHTTPServer` before route registration:

- **`JWClient`** (`jw_client.go`) is the stateless protocol layer — build request, call HTTP, parse and classify the response. `NewJWClient` receives immutable username/password values plus an explicit `utils.HTTPDoer`; tests substitute `mockJWClient` or a fake doer.
- **`TokenManager`** (`token_manager.go`) caches the token and API URL, records whether the current token came from the startup `JW_TOKEN` snapshot or login, and deduplicates login/API-URL work with `singleflight.DoChan`. Each shared operation has its own bounded context detached from the first waiter's cancellation, while every caller can still stop waiting through its own context. On an auth failure, `RefreshAfterAuthFailure` rechecks the failed token inside singleflight: a delayed request reuses any newer token instead of logging in again. The injected override is invalidated only when that actual override token is rejected; expiration of a login-issued token does not change override state.
- **Refresh coordination** (`refresh_coordinator.go`) ensures at most one refresh runs at a time; concurrent requests wait on the same attempt. Internal outcomes explicitly distinguish full success, partial success, and total failure. Partial outcomes set a fixed 30-second soft backoff; consecutive total failures use a 30s → 1m → 2m → 5m base ladder with bounded injectable jitter (±10% of base, absolute cap ±5s). Full success clears the ladder. Partial results are cached with `partial_campuses`, a safe top-level `error`, and prior same-day data for failed campuses when available. Partial payloads still inside the fresh TTL trigger soft-stale revalidation (return data + background refresh) instead of waiting the full 5 minutes. A newer total failure overrides an older partial warning on stale responses. Business time comes from the optional injected `ClassroomServiceOptions.Clock` (shared with `TokenManager`); tests use a thread-safe fake clock rather than replacing an internal `now` function.
- **Topology**: production is a single process-local instance; horizontal multi-app
  replicas are unsupported. See [operations.md](operations.md#deployment-topology-supported-today).
- **Caching**: `cache.New()` creates the single explicit process-local store used by production. One `TODAY_CLASSROOMS_CACHE` key holds today's normalized payload — fully successful data is fresh for ~5 minutes, then served stale until the next **Asia/Shanghai** midnight while refreshes happen in the background. Cache `date` / TTLs are stamped at refresh **completion**. Cross-day reuse is rejected. The context-cancellable warmup scheduler runs immediately at startup, retries a missing cache with a 30s/1m/2m/5m cap, retries partial cache no faster than the fresh TTL, and schedules a new-day refresh after Shanghai midnight plus a small jitter. `main.go` cancels the scheduler before HTTP shutdown and calls `WaitBackground` only after handlers are drained. See [operations.md](operations.md#caching-behavior) for the operator view.
- Rooms like `教学实验综合楼-N104(229)` and merged rooms like `未来学习大楼-202-203(60)` are parsed in `classroom_builder.go`.
- Outbound JW HTTP (`utils/http.go`) does not follow redirects (custom `token` / login bodies must not leave the intended host). Default `APP_ADDR` is loopback (`127.0.0.1:8080`). Cold-path handlers may wait up to the classroom refresh budget; HTTP `WriteTimeout` is set higher so near-limit successes are not cut off.

Logging is `log/slog` with a JSON handler; `LOG_CALLER` is resolved by `config.Load` and passed to `logs.Init`, and a custom wrapper adds the per-request `log_id` from the context to every record (`logs/`).

## Frontend architecture

- `useTodayClassrooms.js` fetches `/api/get_data` and schedules automatic reloads via `reloadSchedule.js`: near `expires_at` when fully fresh, after 5 seconds for ordinary stale data, after 30 seconds for partial-campus data, and with a 5s/10s/20s/30s client-failure backoff. A snapshot is kept only while its date matches the Shanghai business day and `stale_until` remains in the future. Background polls do **not** full-page spin.
- `todayClassroomsResponse.js` normalizes backend envelopes before UI code reads them. Class-period “now” and “today” use Asia/Shanghai to match the backend business day.
- Selection state (campus, buildings, class times, display preferences) lives in a `useReducer` store exposed through `SelectionProvider` / `useSelection()`; preferences persist to `localStorage` in the reducer.
- The classroom table is lazy-loaded behind `Suspense` and an `ErrorBoundary`.
- Dark mode follows system `prefers-color-scheme` only (bootstrap + React share that source; no conflicting `localStorage.darkMode`).

## Conventions

- Format Go code with `gofmt`; imports use the `BUPT_EC/...` module prefix.
- React components are PascalCase `.jsx` files with a matching `.css` beside them when needed; ESLint enforces the configured rules (`pnpm lint`).
- Commits follow Conventional Commits (`feat:`, `fix:`, `chore:`, `ci:`, `docs:`, `refactor:`); keep each commit scoped to one change.
- User-visible changes should update the `[Unreleased]` section of [CHANGELOG.md](../CHANGELOG.md) in the same commit — see [release.md](release.md).
