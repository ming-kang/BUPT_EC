# Development

Local development setup, testing, and a tour of the architecture.

## Requirements

- Go 1.25+ (per `go.mod`)
- Node.js 22 LTS
- pnpm 9.15.x — `corepack enable && corepack prepare pnpm@9.15.0 --activate`
- A valid BUPT teaching affairs account (only for integration tests and running against the real JW system; unit tests run without one)

## Configuration

Create `.env` in the repository root from `.env.example`:

```bash
JW_USERNAME=your_username
JW_PASSWORD=your_password
# Optional debug fallback only. Leave empty for automatic HTTP login.
JW_TOKEN=
# Optional listen address (default 127.0.0.1:8080 when unset).
APP_ADDR=127.0.0.1:8080
# Gin runtime mode. Use release in production.
GIN_MODE=debug
```

Startup validates that either `JW_TOKEN` or both `JW_USERNAME` and `JW_PASSWORD` are set. Never commit real credentials.

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
```

Integration tests (`TestLogin`, `TestQueryOne`, `TestQueryAll` in `service/realtime_data_test.go`) hit the real JW system and require `JW_USERNAME`/`JW_PASSWORD` (or `JW_TOKEN`); without credentials they skip with a clear message.

Unit tests never touch the network: they inject a `mockJWClient` (implementing the `JWClient` interface) into a fresh `ClassroomService` with an isolated cache per test — see `newTestService` in `service/realtime_data_test.go`.

Frontend behavior tests use Vitest and focus on API envelope normalization plus selection-state transitions and preference persistence. Run them with `pnpm test` from `frontend/`.

## Project structure

```text
main.go, router.go, handler.go            Gin entry points; main.go wires ClassroomService into HTTPServer
service/
  classroom_service.go   ClassroomService struct, CacheStore interface, constructor
  realtime_data.go       public API: GetTodayClassrooms, QueryOne/All, refresh data flow
  jw_client.go           JWClient interface + defaultJWClient (HTTP protocol layer)
  token_manager.go       TokenManager: token/API-URL caching with singleflight
  refresh_coordinator.go single-flight refresh attempts, backoff, WaitWarmup
  runtime_status.go      RuntimeStatus for /readyz
  classroom_builder.go   JW rows → campuses/buildings/rooms normalization
  jw_error.go            error classification (auth/config/query/parse) + safe messages
  crypto.go              AES password encryption for the JW login protocol
  urlutil.go             JW API URL validation and building
  model/                 JSON payload shapes
cache/                   go-cache wrapper (5-minute default TTL)
config/                  campus list + env validation
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

All classroom-query runtime state lives on the `ClassroomService` struct. `main.go::Init` creates one service instance, `main()` passes it to `NewHTTPServer`, and `HTTPServer.RegisterRoutes` registers methods from that injected boundary:

- **`JWClient`** (`jw_client.go`) is the stateless protocol layer — build request, call HTTP, parse and classify the response. `defaultJWClient` talks to the real system; tests substitute `mockJWClient`.
- **`TokenManager`** (`token_manager.go`) caches the token and API URL, deduplicates concurrent logins with `singleflight`, honors an emergency `JW_TOKEN` override (applied only while memory token is empty and the override has not been invalidated), and re-logs-in when a query fails with an auth error. Auth failure clears the current token and **invalidates** the env override until process restart so a stale `JW_TOKEN` cannot clobber a good login token.
- **Refresh coordination** (`refresh_coordinator.go`) ensures at most one refresh runs at a time; concurrent requests wait on the same attempt. Failed refreshes set a 30-second backoff. Campus queries run independently: if at least one campus succeeds, the payload is cached (with a top-level `error` when partial) and prior same-day data is merged for failed campuses when available.
- **Caching**: one `TODAY_CLASSROOMS_CACHE` key holds today's normalized payload — fresh for ~5 minutes, then served stale until the next **Asia/Shanghai** midnight while refreshes happen in the background. Cross-day reuse is rejected. Warmup re-runs after each Shanghai midnight. See [operations.md](operations.md#caching-behavior) for the operator view.
- Rooms like `教学实验综合楼-N104(229)` and merged rooms like `未来学习大楼-202-203(60)` are parsed in `classroom_builder.go`.

Logging is `log/slog` with a JSON handler; a custom wrapper adds the per-request `log_id` from the context to every record (`logs/`).

## Frontend architecture

- `useTodayClassrooms.js` fetches `/api/get_data` and schedules an automatic reload near `expires_at` (or every few seconds while `stale` / `error` is set); `todayClassroomsResponse.js` normalizes backend envelopes before UI code reads them. Class-period “now” and “today” use Asia/Shanghai to match the backend business day.
- Selection state (campus, buildings, class times, display preferences) lives in a `useReducer` store exposed through `SelectionProvider` / `useSelection()`; preferences persist to `localStorage` in the reducer.
- The classroom table is lazy-loaded behind `Suspense` and an `ErrorBoundary`.
- Dark mode follows `prefers-color-scheme` and toggles both the Ant Design theme algorithm and a `body.dark` class used by component CSS.

## Conventions

- Format Go code with `gofmt`; imports use the `BUPT_EC/...` module prefix.
- React components are PascalCase `.jsx` files with a matching `.css` beside them when needed; ESLint enforces the configured rules (`pnpm lint`).
- Commits follow Conventional Commits (`feat:`, `fix:`, `chore:`, `ci:`, `docs:`, `refactor:`); keep each commit scoped to one change.
- User-visible changes should update the `[Unreleased]` section of [CHANGELOG.md](../CHANGELOG.md) in the same commit — see [release.md](release.md).
