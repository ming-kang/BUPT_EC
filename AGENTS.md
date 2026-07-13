# Repository Guidelines

## Project Structure & Architecture

This repository is a Go 1.25.12+ service with a React/Vite frontend for BUPT empty-classroom queries. `main.go` composes configuration, HTTP, cache, and JW clients. HTTP routes and handlers live in `router.go` and `handler.go`; core query, refresh, token, and normalization logic lives in `service/`, with API models in `service/model/`. Shared packages are `cache/`, `config/`, `logs/`, and `utils/`.

The frontend is in `frontend/src/`; put reusable UI in `components/` and colocate component CSS. Deployment and release scripts are in `scripts/`; user-facing operational documentation is under `docs/`.

## Build, Test, and Development

Build the embedded frontend before running the Go server:

```bash
cd frontend && pnpm install && pnpm build && cd ..
go run ./
```

Use `go build -o bupt-ec -v ./` for a binary, `go test ./...` for backend tests, and `go test -race ./...` before a substantial change. Run `gofmt -w` on changed Go files and verify `go vet ./...`.

For frontend work, use `pnpm dev`, `pnpm build`, `pnpm lint`, and `pnpm test` from `frontend/`. Vite proxies `/api` to `http://localhost:8080`.

## Style and Testing

Follow `gofmt`; use short lowercase Go package names, imports rooted at `BUPT_EC/...`, and export APIs only when another package needs them. Keep mutable runtime state on `ClassroomService`; do not introduce package-level state. React components use PascalCase filenames (for example, `BuildingPicker.jsx`), hooks and ES modules, and two-space indentation. Component files should export components only.

Add focused Go tests beside implementation as `*_test.go`. Use injected fakes (`mockJWClient`, fresh caches, test clocks) rather than shared state or real JW requests. Frontend tests use Vitest and should cover behavior changes, especially response handling and selection state.

## Commits, PRs, and Configuration

Use one scoped Conventional Commit, such as `fix(installer): validate checksum before extract` or `docs: clarify local setup`. Add user-visible changes to `CHANGELOG.md` under **Unreleased**. PRs need a short description, linked issue when applicable, commands run, and screenshots for UI changes.

Never commit credentials, JW tokens, logs, or private `.env` files. Use `.env.example`; environment values override the optional root `.env`. Keep public behavior, endpoints, configuration, deployment, and release documentation synchronized with code changes.
