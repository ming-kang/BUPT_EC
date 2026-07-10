# Organize Project Structure Design

## Architecture Boundaries

The repository should remain a single Go module with one root `package main` in
this task. The root main package is constrained by `router.go`'s
`//go:embed frontend/dist` directive and by current CI/release commands that
build from `./`. Moving the binary entry point to `cmd/bupt-ec/` would require a
separate coordinated migration of `frontend/`, CI artifact paths, docs, and
specs.

## Proposed Scope

Use a low-risk in-place cleanup:

1. Keep `.cursor/` and `.trellis/` tracked as shared project workflow files.
2. Keep `.claude/` absent; optionally add an ignore guard if the final scope
   wants to prevent accidental reintroduction.
3. Tidy root `.gitignore`: restore final newline, remove stale rules, use clear
   directory suffixes, and add common OS/editor plus Go coverage/profiling
   ignores.
4. If approved, merge the small `init.go` responsibilities into `main.go` while
   leaving `router.go`, `handler.go`, tests, frontend, CI, and release scripts in
   their existing locations.
5. Update docs and Trellis specs for any final file layout changes.

## Deferred Migration

The `cmd/bupt-ec/` migration is intentionally deferred. It is only worth doing
if the project grows more binaries or needs a stronger app/library boundary.
That future task must relocate `frontend/` or isolate the embedded filesystem in
a compatible package, then update CI, release artifact paths, README, docs, and
Trellis specs together.

## Compatibility Notes

- Public HTTP routes and response contracts must not change.
- `frontend/dist/` remains generated and ignored; CI remains responsible for
  producing it before Go build.
- Release assets keep their current names and layout.
- `.env`, `run_log/`, `.tmp/`, frontend build/dependency outputs, and Trellis
  local bytecode/developer files remain local-only.

## Trade-Offs

- Minimal hygiene-only cleanup has the least risk but leaves the tiny `init.go`
  root file as-is.
- Merging `init.go` into `main.go` makes root entry-point ownership clearer with
  low risk, but requires doc/spec updates and focused Go verification.
- A full `cmd/` migration gives conventional Go layout but has high churn and is
  not justified for the current single-binary service.
