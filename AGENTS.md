# Repository Guidelines

Use `pwsh` (PowerShell 7) or Git Bash on Windows.

Read [docs/development.md](docs/development.md) for setup, architecture, maintenance contracts, and known follow-ups. For deployment or release changes, also read [docs/upgrading.md](docs/upgrading.md) and [docs/release.md](docs/release.md); runtime behavior and timeout budgets live in [docs/operations.md](docs/operations.md).

## Structure and conventions

- `main.go` composes configuration and dependencies; `router.go` / `handler.go` own HTTP policy. Query, refresh, token, cache, and JW transport logic lives in `service/`, with API models in `service/model/`; shared packages are `config/` and `logs/`.
- Keep mutable service state on `ClassroomService`. Inject the startup configuration, JW client, and clock; configuration is read once, with process environment taking precedence over optional `.env`. The production design is one process with an in-memory day cache.
- Go follows `gofmt`, short lowercase package names, and `BUPT_EC/...` imports. Export only what another package needs.
- `frontend/src/` uses strict TypeScript, React hooks, ES modules, and two-space indentation. Components are PascalCase files under `components/`, export components only, and colocate CSS. Shared selection state uses `useSelection()`.
- Edit installer fragments in `scripts/installer/`, then regenerate tracked `scripts/install.sh`; it must remain self-contained. `scripts/bupt-ec-cli.sh` is a separate thin operations CLI. Operational documentation lives in `docs/`.

## Build and verification

```bash
task build   # pnpm build → copy frontend/dist to web/dist → go build -tags embed_assets
task test    # go test -race ./...
task check   # format/vet/dependencies + frontend/installer checks
task installer:generate   # regenerate install.sh after editing fragments
task installer:check      # generation, syntax, behavior, release layout, ShellCheck
```

`go run ./` and tag-less builds serve a placeholder UI so clean checkouts compile. Native commands are in the development guide. Run `gofmt` on changed Go files, `go vet ./...`, and relevant tests; use `task test` for substantial backend changes.

Frontend work uses `pnpm dev`, `pnpm lint`, `pnpm typecheck`, `pnpm test`, and `pnpm build` from `frontend/`. Dependency/chunk changes also need `pnpm size` after a fresh build and both dependency audits. `task check` does not run the Go tests, production build, or bundle-size gate.

Tests should cover changed behavior. Use fresh service instances, injected fakes/clocks, and `service/export_test.go` seams; unit tests must not contact JW. Frontend tests use Vitest, with jsdom for component/hook lifecycles.

## Change rules

- Search with `rg` before adding helpers or changing shared values. Fix behavior in its owning layer and keep unrelated refactors separate.
- Keep `service/model/realtime_data.go`, `frontend/src/api/types.ts`, response normalization, and consumers aligned when changing public JSON. Preserve Shanghai-day expiry, partial/stale semantics, and immutable cache publication.
- Use `slog.*Context` for request logs and `service.SafeErrorMessage` for client errors. Keep raw JW bodies and secrets out of responses/logs. Never commit credentials, tokens, runtime logs, or private `.env` files; use `.env.example` for examples.
- Respect intentional dependency major lines and the browser target. Treat major upgrades as separate migrations; avoid blanket `pnpm update --latest`.
- Use scoped Conventional Commits. Update relevant docs and `CHANGELOG.md` under **Unreleased** for user-visible changes. PRs include a short description, relevant issue, checks run, and screenshots for UI changes.
