# Repository Guidelines

## Project Structure & Module Organization
This repository contains a Go 1.21 backend and a React/Vite frontend for an empty-classroom query service. Backend entry points live at `main.go`, `router.go`, `handler.go`, and `init.go`. Domain logic is under `service/`, shared helpers under `utils/`, caching under `cache/`, logging under `logs/`, and runtime configuration loading under `config/`. Frontend code lives in `frontend/src/`, with reusable components in `frontend/src/components/`, assets in `frontend/src/assets/`, and public static files in `frontend/public/`. Deployment automation lives in `scripts/install.sh`; CI and release definitions live under `.github/workflows/`.

## Build, Test, and Development Commands
- `go run ./` starts the backend locally. Ensure required environment variables are set first, and build `frontend/dist/` first if it is missing because the backend embeds it.
- `go build -o EmptyClassroom -v ./` builds the backend binary. Run `cd frontend; pnpm build` first when building locally; CI builds/downloads `frontend/dist/` before compiling Go.
- `go test ./...` runs Go tests. Some service tests call external login/query flows and require valid credentials.
- `cd frontend; pnpm install` installs frontend dependencies from `pnpm-lock.yaml`.
- `cd frontend; pnpm dev` starts the Vite development server.
- `cd frontend; pnpm build` creates `frontend/dist/` for backend static serving and CI artifacts.
- `cd frontend; pnpm lint` runs ESLint for JS/JSX files.
- Pushing a `v*` tag runs the release workflow, building Linux `amd64`/`arm64` archives plus `checksums.txt` and `install.sh`.

## Coding Style & Naming Conventions
Format Go code with `gofmt`; keep package names short and lowercase. Use exported names only for cross-package APIs. React components use PascalCase filenames such as `BuildingPicker.jsx`; component-specific styles sit beside them as matching `.css` files. JavaScript modules use ES modules, React hooks, and 2-space indentation consistent with the existing frontend.

## Testing Guidelines
Go tests use the standard `testing` package and follow `TestXxx` naming in `*_test.go` files. Place backend tests near the package they verify, for example `service/realtime_data_test.go`. Integration-like service tests require valid `JW_USERNAME` and `JW_PASSWORD` values from `.env.example`; otherwise they should skip cleanly. The frontend currently has lint/build checks but no test framework configured.

## Commit & Pull Request Guidelines
History uses Conventional Commit prefixes such as `feat:`, `fix:`, `chore:`, and `ci:`. Keep commit messages concise and scoped to one change. Pull requests should include a short description, linked issue when applicable, test/build commands run, and screenshots for visible frontend changes.

## Security & Configuration Tips
Do not commit real `JW_USERNAME`, `JW_PASSWORD`, `JW_TOKEN`, generated logs, or private config data. Use `.env.example` as the template for local secrets. `APP_ADDR` controls the listen address, commonly `127.0.0.1:8080` behind Nginx. The backend talks directly to the BUPT teaching affairs HTTP endpoints and uses only same-day in-memory cache data; do not reintroduce local timetable databases unless explicitly requested.
