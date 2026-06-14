# Repository Guidelines

## Project Structure & Module Organization
This repository contains a Go 1.25 backend (Go module `BUPT_EC`) and a React/Vite frontend for a BUPT empty-classroom query service. Backend entry points live at `main.go`, `router.go`, `handler.go`, and `init.go`. Domain logic is under `service/` (with JSON shapes in `service/model/`), shared helpers under `utils/`, caching under `cache/`, logging under `logs/`, and runtime configuration loading under `config/`. The frontend lives in `frontend/src/`, with reusable components in `frontend/src/components/`, static assets in `frontend/src/assets/`, and public static files in `frontend/public/`. Deployment automation lives in `scripts/install.sh`; CI and release definitions live under `.github/workflows/`, automated dependency updates under `.github/dependabot.yml`. The release artifacts are `bupt-ec-linux-amd64.tar.gz` and `bupt-ec-linux-arm64.tar.gz`; on a deployed server the systemd unit is `bupt-ec.service` under `/opt/bupt-ec`.

## Build, Test, and Development Commands
- `go run ./` starts the backend locally. `JW_USERNAME` and `JW_PASSWORD` (or `JW_TOKEN`) must be set; the backend reads them from a `.env` file via `godotenv`. Build `frontend/dist/` first if it is missing because the backend embeds it via `//go:embed frontend/dist` in `router.go`.
- `go build -o bupt-ec -v ./` builds the backend binary. Run `cd frontend; pnpm build` first when building locally; CI builds the frontend and downloads the artifact into `frontend/dist/` before compiling Go.
- `go test ./...` runs Go tests. `service/realtime_data_test.go` contains unit tests (`TestEncryptJWPassword`, `TestParseRoom`) that always run plus integration tests (`TestLogin`, `TestQueryOne`, `TestQueryAll`) that need valid `JW_USERNAME`/`JW_PASSWORD` and skip otherwise.
- `cd frontend; pnpm install` installs frontend dependencies from `pnpm-lock.yaml` (lockfile v9, pnpm 9.15.x).
- `cd frontend; pnpm dev` starts the Vite development server; it proxies `/api` to `http://localhost:8080` (see `frontend/vite.config.js`).
- `cd frontend; pnpm build` creates `frontend/dist/` for backend static serving and CI artifacts.
- `cd frontend; pnpm lint` runs ESLint for JS/JSX files.
- Pushing a `v*` tag triggers the `Release` workflow, which builds Linux `amd64`/`arm64` archives plus `bupt-ec-linux-*.tar.gz`, `checksums.txt`, and `install.sh` from the bundled binary (see "Release Process" below).

## Architecture and Data Flow
- Single public endpoint: `GET /api/get_data`, defined in `router.go` and implemented by `handler.go` → `service.GetData` → `service.GetTodayClassrooms`.
- The backend does **not** maintain a local timetable database. `service/realtime_data.go` calls the BUPT JW system on demand:
  - `https://jwglweixin.bupt.edu.cn/sjd/serverconfig.json` resolves the live API base URL (with a fallback default in `DefaultAPIURL`).
  - `POST /login` performs an AES-encrypted password login and stores the returned token in memory.
  - `POST /todayClassrooms?campusId=01|04` fetches Xitucheng (`01`) and Shahe (`04`) classroom rows. Response shapes live in `service/model/realtime_data.go` (`JWClassInfo`, `QueryResponse`, `TodayClassrooms`, etc.).
- Results are normalized into a `TodayClassrooms` payload grouped by `campus` → `buildings` → `rooms` with `free_nodes`/`free_times`, plus a `nodes` list of class periods. See `service/realtime_data.go::buildCampusInfo` and the `parseRoom`/`splitRoomName` helpers that handle rooms like `教学实验综合楼-N104(229)` and merged rooms like `未来学习大楼-202-203(60)`.
- The `TokenManager` (`service/realtime_data.go`) refreshes the token on auth failure and supports an emergency `JW_TOKEN` env override for debugging.

## Caching
- `cache/cache.go` wraps `github.com/patrickmn/go-cache` with a 5-minute fresh TTL and 1-minute janitor interval.
- A single `TODAY_CLASSROOMS_CACHE` key holds the `*model.TodayClassrooms` value for the current day. `service.GetTodayClassrooms` first returns the fresh cache, then attempts a refresh; on failure it falls back to a `stale=true` response until `endOfDay(now)`. Cross-day cache reuse is rejected by `getCachedTodayClassrooms`.
- The cache is process-local; restarting the backend or running multiple instances does not share it.

## Coding Style & Naming Conventions
- Format Go code with `gofmt`; keep package names short and lowercase. Use exported names only for cross-package APIs (e.g., `service.GetData`, `service.GetTodayClassrooms`, `config.GetConfig`).
- The Go module name is `BUPT_EC`; all internal imports use that prefix (e.g., `"BUPT_EC/service"`, `"BUPT_EC/cache"`).
- React components use PascalCase filenames such as `BuildingPicker.jsx`; component-specific styles sit beside them as matching `.css` files. The main result table component is `TodayClassroomTable.jsx` with class `.today-classroom-table`. The frontend `package.json` `name` is `bupt-ec`.
- JavaScript modules use ES modules, React hooks, and 2-space indentation consistent with the existing frontend. ESLint config (`frontend/.eslintrc.cjs`) enforces `eslint:recommended`, `plugin:react/recommended`, and `react-refresh/only-export-components` (allowConstantExport).

## Testing Guidelines
- Go tests use the standard `testing` package and follow `TestXxx` naming in `*_test.go` files. Place backend tests near the package they verify, for example `service/realtime_data_test.go`.
- Integration-like service tests call `ResetRuntimeStateForTest()` so they start with a clean `TokenManager` and cache. They require valid `JW_USERNAME` and `JW_PASSWORD` (or `JW_TOKEN`) values, otherwise they `t.Skip` cleanly with a clear message.
- The frontend currently has lint/build checks but no test framework configured.

## Commit & Pull Request Guidelines
- History uses Conventional Commit prefixes such as `feat:`, `fix:`, `chore:`, `ci:`, and `docs:`. Keep commit messages concise and scoped to one change.
- Pull requests should include a short description, linked issue when applicable, test/build commands run, and screenshots for visible frontend changes.
- Do not mix module renames, dependency updates, and behavior changes in a single commit.

## Release Process

Releases are cut by pushing a `v*` tag. The `Release` workflow (`.github/workflows/release.yml`) runs three jobs in sequence:

1. `build-frontend` builds the React app with pnpm 9 and Node 22, then uploads `frontend/dist/` as an artifact named `frontend-dist`.
2. `build-go` (matrix `amd64` + `arm64`) downloads the frontend artifact and compiles the Go 1.25 binary for each architecture, uploading each as `bupt-ec-linux-${goarch}`.
3. `release` downloads the binaries, wraps each one with `.env.example`, `README.md`, and `install.sh` into a tarball, generates `checksums.txt`, and publishes a GitHub Release via `softprops/action-gh-release`.

To cut a release:

```bash
git tag v0.1.0
git push origin v0.1.0
```

Then watch the `Actions` tab. The release will appear at `https://github.com/ming-kang/BUPT_EC/releases/tag/v0.1.0` with these assets:

- `bupt-ec-linux-amd64.tar.gz`
- `bupt-ec-linux-arm64.tar.gz`
- `checksums.txt`
- `install.sh`

The release workflow can also be triggered manually from the Actions tab (`workflow_dispatch`) for dry-runs without pushing a tag. The install script (`scripts/install.sh`) downloads the matching tarball, so the assets above must keep their exact filenames and directory layout:

```
bupt-ec-linux-${arch}/
  bupt-ec
  .env.example
  README.md
  install.sh
```

## Toolchain Versions

Pinned via the workflows and `go.mod`; Dependabot (`.github/dependabot.yml`) opens weekly PRs for minor and patch updates.

- Go: 1.25 (per `go.mod` `go` directive and `actions/setup-go` `go-version`)
- Node: 22 LTS (per `actions/setup-node` `node-version` in both workflows)
- pnpm: 9.15.x (per `corepack prepare` in both workflows, lockfile v9 in `frontend/pnpm-lock.yaml`)
- All `actions/*` and `softprops/action-gh-release` are pinned to 40-character commit SHAs (see comments in the workflow YAMLs) for supply-chain safety. Use Dependabot PRs or the `git ls-remote` query below to bump them:

  ```bash
  git ls-remote https://github.com/actions/checkout.git refs/tags/v4
  ```

- Dependabot (`.github/dependabot.yml`) is configured for `github-actions`, `npm` (frontend), and `gomod` ecosystems, all on a weekly schedule grouped into single PRs by minor/patch version.

## Security & Configuration Tips
- Do not commit real `JW_USERNAME`, `JW_PASSWORD`, `JW_TOKEN`, generated logs, or private config data. Use `.env.example` as the template for local secrets.
- `APP_ADDR` controls the listen address, commonly `127.0.0.1:8080` behind Nginx. The server defaults to `:8080`.
- The `logs` package writes to `run_log/ec.log` (relative to the working directory) when `logs.Init(true)` is called from `Init`; the working directory of the systemd unit is `${INSTALL_DIR}`, so the path becomes `/opt/bupt-ec/run_log/ec.log` after install. Keep `run_log/` and `.env` out of version control.
- The AES key for JW password encryption (`tokenPasswordKey` in `service/realtime_data.go`) is compiled into the binary; rotating it requires a new release. Do not log JW passwords or tokens.
- The backend talks directly to the BUPT teaching affairs HTTP endpoints and uses only same-day in-memory cache data; do not reintroduce local timetable databases unless explicitly requested.
- The installer (`scripts/install.sh`) hardens the systemd unit with `NoNewPrivileges`, `PrivateTmp`, `ProtectHome`, `ProtectSystem=full`, and a dedicated `bupt-ec` system user. Keep the env file at `/etc/bupt-ec/bupt-ec.env` mode `0600` and owned by `root`.
