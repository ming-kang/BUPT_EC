# Repository Guidelines

## Project Structure & Module Organization
This repository contains a Go 1.25 backend (Go module `BUPT_EC`) and a React/Vite frontend for a BUPT empty-classroom query service. Backend entry points live at `main.go`, `router.go`, and `handler.go`; `main.go` owns process startup, constructs the single `service.ClassroomService` instance, and injects it into `HTTPServer` before route registration. Domain logic is under `service/`, split into focused files: `classroom_service.go` (struct + `CacheStore`), `realtime_data.go` (public API + refresh data flow), `jw_client.go` (`JWClient` interface + HTTP protocol layer), `token_manager.go`, `refresh_coordinator.go`, `runtime_status.go`, `classroom_builder.go`, `jw_error.go`, `crypto.go`, `urlutil.go`, with JSON shapes in `service/model/`. Shared helpers are under `utils/`, caching under `cache/`, logging under `logs/`, runtime configuration under `config/`. The frontend lives in `frontend/src/` with components in `frontend/src/components/`; selection state lives in `frontend/src/selectionContext.js` + `SelectionProvider.jsx`. Deployment automation is `scripts/install.sh`; release helpers are `scripts/release.sh` and `scripts/extract-changelog.sh`; CI definitions live under `.github/workflows/` (`ci.yml` for PRs, `release.yml` for main pushes and tags). Dependencies are managed manually (no Dependabot). On a deployed server the systemd unit is `bupt-ec.service` under `/opt/bupt-ec`.

User-facing documentation lives in `README.md` (overview) and `docs/` (`deployment.md`, `upgrading.md`, `operations.md`, `development.md`, `release.md`). Keep these in sync when behavior, endpoints, configuration, or the release process change.

## Build, Test, and Development Commands
- `go run ./` starts the backend locally. `JW_USERNAME` and `JW_PASSWORD` (or `JW_TOKEN`) must be set; the backend reads them from a `.env` file via `godotenv`. Build `frontend/dist/` first if it is missing because the backend embeds it via `//go:embed frontend/dist` in `router.go`.
- `go build -o bupt-ec -v ./` builds the backend binary. Run `cd frontend; pnpm build` first when building locally; CI builds the frontend and downloads the artifact into `frontend/dist/` before compiling Go.
- `go test ./...` runs Go tests (CI runs `go test -race ./...`). Unit tests in `service/realtime_data_test.go` and `handler_test.go` always run and never touch the network; integration tests (`TestLogin`, `TestQueryOne`, `TestQueryAll`) need valid `JW_USERNAME`/`JW_PASSWORD` and skip otherwise.
- `gofmt -l .` must print nothing and `go vet ./...` must pass; both are enforced in CI.
- `cd frontend; pnpm install` installs frontend dependencies from `pnpm-lock.yaml` (lockfile v9, pnpm 9.15.x).
- `cd frontend; pnpm dev` starts the Vite development server; it proxies `/api` to `http://localhost:8080` (see `frontend/vite.config.js`).
- `cd frontend; pnpm build` creates `frontend/dist/`; `cd frontend; pnpm lint` runs ESLint.
- `cd frontend; pnpm test` runs the focused Vitest behavior tests.

## Architecture and Data Flow
- Public endpoint `GET /api/get_data` is defined by `HTTPServer.RegisterRoutes` in `router.go` and implemented by `handler.go::HTTPServer.GetData` → injected `classroomService.GetTodayClassrooms`. Operational endpoints are `/healthz` and `/readyz` (runtime status). Unknown non-API paths fall back to the embedded `index.html` (SPA fallback in `router.go`); unknown `/api/*` paths return JSON 404.
- All mutable runtime state lives on the `ClassroomService` struct — there are no package-level mutable globals in `service/`. The struct owns the `TokenManager`, a `CacheStore`, the campus list, a `JWClient`, refresh-coordination state, and `RuntimeStatus`.
- The backend does **not** maintain a local timetable database. It calls the BUPT JW system on demand through the stateless `JWClient` interface (`jw_client.go`):
  - `FetchAPIURL`: `https://jwglweixin.bupt.edu.cn/sjd/serverconfig.json` resolves the live API base URL (validated by `urlutil.go`, fallback in `DefaultAPIURL`).
  - `Login`: AES-encrypted password login (`crypto.go`); `TokenManager` tracks whether the cached token came from login or `JW_TOKEN`. Concurrent auth failures for the same rejected token share one bounded singleflight login, while delayed failures reuse an already-installed replacement. `JW_TOKEN` is invalidated until restart only when the rejected token actually came from that override.
  - `QueryCampus`: `POST /todayClassrooms?campusId=01|04` for Xitucheng (`01`) and Shahe (`04`). Response shapes live in `service/model/realtime_data.go`.
- Results are normalized into a `TodayClassrooms` payload grouped by `campus` → `buildings` → `rooms` with `free_nodes`/`free_times`, plus a `nodes` list of class periods. See `service/classroom_builder.go` (`buildCampusInfo`, `parseRoom`, `splitRoomName`) for rooms like `教学实验综合楼-N104(229)` and merged rooms like `未来学习大楼-202-203(60)`.
- Refreshes are single-flight (`refresh_coordinator.go`): concurrent requests share one attempt; internal outcomes distinguish full success, partial success, and total failure. Total or partial outcomes set a 30-second backoff. `warmup.go` owns the single context-cancellable scheduler; graceful shutdown cancels it before HTTP drain and then calls `WaitBackground`, which prevents new workers before waiting.

## Caching
- `cache/cache.go` wraps `github.com/patrickmn/go-cache`; `cache.GlobalCache` satisfies the `service.CacheStore` interface directly.
- A single `TODAY_CLASSROOMS_CACHE` key holds the `*model.TodayClassrooms` value for the current **Asia/Shanghai** business day: fully successful data is fresh for ~5 minutes, then served with `stale=true` until next Shanghai midnight while background refreshes run. Partial campus success is cached with `partial_campuses` plus a top-level `error` and soft-stale revalidated inside the fresh TTL (return data + background retry). A later total failure overrides the older partial warning on stale responses. Cache day metadata is stamped at refresh completion. Cross-day cache reuse is rejected by `getCachedTodayClassrooms`.
- Without a usable cache, warmup retries after 30s/1m/2m/5m (cap). Partial cache retries no faster than the fresh TTL; complete cache waits for the next Shanghai midnight plus a 1–5s jitter. All paths still use the refresh coordinator's single-flight/backoff rules.
- The cache is process-local; restarting the backend or running multiple instances does not share it.

## Logging
- The `logs` package configures `log/slog` with a JSON handler writing to stdout, plus `run_log/ec.log` (lumberjack rotation) when `logs.Init(true)` is called from `main.go::Init`.
- Every `/api/*` request gets a `log_id` (set by `logs.SetNewContextForGinContext`); a custom handler wrapper stamps it onto every record. It is also returned in the `LogID` response header and in the body of API error responses.
- Log with `slog.InfoContext(ctx, "msg", "key", value)` style calls so the `log_id` is attached; do not use `log.Printf` in request paths.

## Coding Style & Naming Conventions
- Format Go code with `gofmt`; keep package names short and lowercase. Use exported names only for cross-package APIs (e.g., `service.NewClassroomService`, `service.SafeErrorMessage`, `config.GetConfig`).
- The Go module name is `BUPT_EC`; all internal imports use that prefix (e.g., `"BUPT_EC/service"`, `"BUPT_EC/cache"`).
- React components use PascalCase filenames such as `BuildingPicker.jsx`; component-specific styles sit beside them as matching `.css` files. Shared frontend state goes through `useSelection()` from `selectionContext.js` rather than prop drilling.
- JavaScript modules use ES modules, React hooks, and 2-space indentation. ESLint config (`frontend/.eslintrc.cjs`) enforces `eslint:recommended`, `plugin:react/recommended`, and `react-refresh/only-export-components` — keep component files exporting only components (hooks/constants go in separate `.js` files).

## Testing Guidelines
- Go tests use the standard `testing` package and follow `TestXxx` naming in `*_test.go` files placed next to the package they verify.
- Service unit tests create an isolated `ClassroomService` per test via `newTestService(t, client)`, injecting a `mockJWClient` (an implementation of the `JWClient` interface) and a fresh `gocache` instance — no shared globals, no cleanup calls needed. Handler tests inject a deterministic fake through `NewHTTPServer` instead of mutating package-level service state.
- Integration tests require valid `JW_USERNAME`/`JW_PASSWORD` (or `JW_TOKEN`) and otherwise `t.Skip` cleanly with a clear message.
- Frontend behavior tests use Vitest for API envelope normalization and selection-state behavior. Keep additions focused on meaningful regression risk rather than runner-only smoke tests.

## Commit, Changelog & Pull Request Guidelines
- History uses Conventional Commit prefixes such as `feat:`, `fix:`, `chore:`, `ci:`, `docs:`, and `refactor:`. Keep commit messages concise and scoped to one change.
- User-visible changes must add a bullet to the `[Unreleased]` section of `CHANGELOG.md` in the same commit (Keep a Changelog categories: Added/Changed/Fixed/Removed/Deprecated/Security, plus Dependencies). Internal-only changes may skip it. This section becomes the release notes verbatim.
- Pull requests should include a short description, linked issue when applicable, test/build commands run, and screenshots for visible frontend changes.
- Do not mix module renames, dependency updates, and behavior changes in a single commit.

## Release Process
See [docs/release.md](docs/release.md) for the full picture. Key facts:

- Every push to `main` republishes the rolling `nightly` prerelease; pushing a `v*` tag publishes an immutable stable release whose notes come from the matching `CHANGELOG.md` section (extracted by `scripts/extract-changelog.sh`).
- Cut stable releases with `scripts/release.sh vX.Y.Z` — it rolls the changelog, bumps `frontend/package.json`, commits, tags, and pushes. Do not hand-edit tags or release notes.
- PRs are validated by `.github/workflows/ci.yml`; direct pushes to `main` are validated by the `quality-gate` job in `release.yml` (frontend lint/test/build, gofmt, vet, `go test -race`, govulncheck, `bash scripts/install_test.sh`, and shellcheck on `scripts/*.sh`).
- Release assets must keep their exact names and layout (`bupt-ec-linux-${arch}.tar.gz` containing `bupt-ec`, `.env.example`, `README.md`, `install.sh`, plus top-level `checksums.txt` and `install.sh`) because `scripts/install.sh` depends on them.
- Toolchain: Go 1.25, Node 22, pnpm 9.15.x. All GitHub Actions are pinned to 40-character commit SHAs; bump pins by resolving the new SHA with `git ls-remote` and updating both the ref and the comment.

## Security & Configuration Tips
- Do not commit real `JW_USERNAME`, `JW_PASSWORD`, `JW_TOKEN`, generated logs, or private config data. Use `.env.example` as the template for local secrets.
- `APP_ADDR` controls the listen address, commonly `127.0.0.1:8080` behind Nginx. The server defaults to `127.0.0.1:8080` when unset.
- The `logs` package writes to `run_log/ec.log` relative to the working directory; on an installed server that is `/opt/bupt-ec/run_log/ec.log`. Keep `run_log/` and `.env` out of version control.
- The AES key for JW password encryption (`tokenPasswordKey` in `service/crypto.go`) matches the JW system protocol and is compiled into the binary; do not change it, and do not log JW passwords or tokens.
- The backend talks directly to the BUPT teaching affairs HTTP endpoints and uses only same-day in-memory cache data; do not reintroduce local timetable databases unless explicitly requested.
- The installer (`scripts/install.sh`) hardens the systemd unit with `NoNewPrivileges`, `PrivateTmp`, `ProtectHome`, `ProtectSystem=full`, and a dedicated `bupt-ec` system user. Keep the env file at `/etc/bupt-ec/bupt-ec.env` mode `0600` and owned by `root`.
- Installer release selection precedence is explicit `VERSION`, then saved `RELEASE_VERSION`, then the first-install `nightly` default. Documentation commands must pass `VERSION=latest`, `VERSION=nightly`, or a matching fixed tag so the downloaded installer and package cannot diverge.
