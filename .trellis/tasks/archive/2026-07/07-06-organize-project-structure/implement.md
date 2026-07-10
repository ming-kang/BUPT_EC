# Organize Project Structure Implementation Plan

## Pre-Implementation Gate

- Resolve the scope question in `prd.md` before editing source files.
- Do not start a `cmd/` migration in this task.

## Implementation Checklist

1. Rewrite root `.gitignore` according to the approved cleanup scope.
2. Confirm `.claude/` remains absent and no docs/specs reference it as an active
   project workflow directory.
3. If approved, merge `init.go` into `main.go` without behavior changes:
   - move `var classroomService` and `Init()` into `main.go`;
   - delete `init.go`;
   - run `gofmt` on affected Go files.
4. Update source-backed docs/specs that reference the root entry-point set:
   - `AGENTS.md`;
   - `docs/development.md`;
   - `.trellis/spec/backend/index.md` if needed;
   - `.trellis/spec/backend/directory-structure.md`;
   - any other `.trellis/spec/backend/*.md` file that names changed entry-point
     ownership.
5. Add a `CHANGELOG.md` `[Unreleased]` bullet if tracked project layout or
   contributor instructions change.
6. Keep Trellis task files for this planning task in the final commit.

## Validation Plan

Run focused checks for the final scope:

```bash
gofmt -l .
go vet ./...
go test ./...
```

If source files change, also consider:

```bash
go test -race ./...
go build -o bupt-ec -v ./
```

For docs-only or ignore-only changes, document why Go/frontend checks were not
necessary.

## Rollback Points

- `.gitignore` can be reverted independently if an ignore pattern is too broad.
- `init.go` merge can be rolled back by restoring the file and removing the moved
  declarations from `main.go`.
- Avoid moving `frontend/` or CI paths so rollback remains small.
