# Organize Project Structure

## Goal

Tidy the repository structure and ignore rules without taking on a risky source
tree migration. The result should make project-owned files, local-only files,
and backend entry-point ownership clearer while preserving the current build,
test, release, and embedded-frontend behavior.

## Background and Confirmed Facts

- `.claude/` has been deleted intentionally; this project now uses the tracked
  `.cursor/` and `.trellis/` setup for local AI workflow support.
- `.cursor/` and `.trellis/` were committed in `20ecc33` and should be treated
  as project-owned unless this task explicitly decides otherwise.
- The current dirty `.gitignore` change is part of this cleanup round and should
  be committed together with the final project-structure changes.
- The repository's top-level source ownership is already mostly clean: Go entry
  points at the root, domain logic in `service/`, JSON models in
  `service/model/`, source packages in `cache/`, `config/`, `logs/`, `utils/`,
  docs in `docs/`, and deployment/release automation in `scripts/`.
- `router.go` embeds `frontend/dist` with `//go:embed frontend/dist`. Moving the
  main package into `cmd/` without also relocating `frontend/` would break the
  build because `go:embed` cannot use `..` path elements.
- The frontend tree is clean: build outputs and dependencies are ignored, and CI
  builds `frontend/dist/` before the Go binary is compiled.
- Local-only artifacts observed by structure review are already ignored: `.env`,
  `.tmp/`, `run_log/`, `frontend/node_modules/`, `frontend/dist/`, Trellis
  `__pycache__/`, and `.trellis/.developer`.

## Requirements

- Keep `.cursor/` and `.trellis/` tracked as shared project workflow files.
- Keep the current root Go main package in this round; do not move the binary
  entry point to `cmd/` or relocate `frontend/`.
- Clean `.gitignore` so it has a final newline, removes stale rules, and clearly
  protects local logs, local secrets, temporary files, OS/editor junk, and Go
  coverage/profiling output.
- If root entry-point cleanup is approved, keep it in-place and behavior-neutral
  by merging the tiny `init.go` responsibilities into `main.go` rather than
  changing package paths.
- Update docs/specs that name root entry-point files or project structure so
  they match the final tree.
- Add a `CHANGELOG.md` `[Unreleased]` bullet if the cleanup changes tracked
  project layout or user-facing contributor instructions.

## Out of Scope

- Moving the application to `cmd/bupt-ec/`.
- Relocating `frontend/`.
- Reworking CI artifact paths or release asset layout.
- Replacing Gin/router/static serving behavior.
- Introducing new runtime configuration formats or database/persistence layers.

## Acceptance Criteria

- [x] `.gitignore` is tidy, keeps local secrets/build/log outputs ignored, and
      ends with a newline.
- [x] The repository has no reintroduced `.claude/` workflow files.
- [x] Any root Go file cleanup compiles and preserves `/api/get_data`,
      `/healthz`, `/readyz`, gzip, and SPA fallback behavior.
- [x] Docs and Trellis specs match the final source layout.
- [x] Relevant validation commands pass or are documented if skipped.
- [x] This round is committed together, including the planned `.gitignore`
      cleanup.

## Scope Decision

This round includes the recommended in-place backend entry-point tidy:
`init.go` is merged into `main.go`, while any `cmd/` migration remains deferred.

## Validation Results

- `gofmt -l .` passed with no output after formatting `main.go`.
- `go vet ./...` passed.
- `go test ./...` passed.
- `go build ./...` passed.
- A readonly review subagent found no stale `init.go` references outside this
  task's planning artifacts and no blocking `.gitignore` pattern risks.
