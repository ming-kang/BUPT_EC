# Repository Guidelines

## Project Structure & Architecture

This repository is a Go 1.25.13+ service with a React/Vite frontend for BUPT empty-classroom queries. `main.go` composes configuration, HTTP, and JW clients. HTTP routes and handlers live in `router.go` and `handler.go`; core query, refresh, token, caching, and normalization logic lives in `service/`, with API models in `service/model/`. Shared packages are `config/` and `logs/`; JW outbound HTTP transport lives in `service/` (`jw_http.go`).

The frontend is in `frontend/src/`; put reusable UI in `components/` and colocate component CSS. Deployment and release scripts are in `scripts/`; user-facing operational documentation is under `docs/`.

## Build, Test, and Development

Local entry points live in `Taskfile.yml` ([Task](https://taskfile.dev)):

```bash
task build   # pnpm build → copy frontend/dist to web/dist → go build -tags embed_assets
task test    # go test -race ./...
task check   # gofmt/vet/tidy/verify + frontend lint/test/audits (mirrors CI; skips `pnpm size` on purpose — it needs a fresh production build)
```

Native equivalents without `task`: `cd frontend && pnpm install && pnpm build && cd ..`, then `rm -rf web/dist && cp -r frontend/dist web/dist` and `go build -tags embed_assets -o bupt-ec ./` — without the `-trimpath -ldflags "-s -w -X main.version=..."` that `task build` adds, so the binary reports version `dev`. `go run ./` and bare `go build` compile without the tag and serve a placeholder page instead of the UI (this keeps clean checkouts buildable). Use `go test ./...` for backend tests and `go test -race ./...` before a substantial change. Run `gofmt -w` on changed Go files and verify `go vet ./...`.

For frontend work, use `pnpm dev`, `pnpm build`, `pnpm lint`, and `pnpm test` from `frontend/`. Vite proxies `/api` to `http://localhost:8080`.

## Style and Testing

Follow `gofmt`; use short lowercase Go package names, imports rooted at `BUPT_EC/...`, and export APIs only when another package needs them. Keep mutable runtime state on `ClassroomService`; do not introduce package-level state. The frontend is strict TypeScript (`pnpm typecheck`; wired into CI). React components use PascalCase filenames (for example, `BuildingPicker.tsx`), hooks and ES modules, and two-space indentation. Component files should export components only.

Add focused Go tests beside implementation as `*_test.go`. Use injected fakes (`mockJWClient`, test clocks) and the white-box seams in `service/export_test.go` rather than shared state or real JW requests. Frontend tests use Vitest and should cover behavior changes, especially response handling and selection state.

## Commits, PRs, and Configuration

Use one scoped Conventional Commit, such as `fix(installer): validate checksum before extract` or `docs: clarify local setup`. Add user-visible changes to `CHANGELOG.md` under **Unreleased**. PRs need a short description, linked issue when applicable, commands run, and screenshots for UI changes.

Never commit credentials, JW tokens, logs, or private `.env` files. Use `.env.example`; environment values override the optional root `.env`. Keep public behavior, endpoints, configuration, deployment, and release documentation synchronized with code changes.

<!-- TRELLIS:START -->
# Trellis Instructions

These instructions are for AI assistants working in this project.

This project is managed by Trellis. The working knowledge you need lives under `.trellis/`:

- `.trellis/workflow.md` — development phases, when to create tasks, skill routing
- `.trellis/spec/` — package- and layer-scoped coding guidelines (read before writing code in a given layer)
- `.trellis/workspace/` — per-developer journals and session traces
- `.trellis/tasks/` — active and archived tasks (PRDs, research, jsonl context)

If a Trellis command is available on your platform (e.g. `/trellis:finish-work`, `/trellis:continue`), prefer it over manual steps. Not every platform exposes every command.

If you're using Codex or another agent-capable tool, additional project-scoped helpers may live in:
- `.agents/skills/` — reusable Trellis skills
- `.codex/agents/` — optional custom subagents

Managed by Trellis. Edits outside this block are preserved; edits inside may be overwritten by a future `trellis update`.

<!-- TRELLIS:END -->
