# Execution Plan: Full Repository Code Quality Audit

## 1. Establish inventory and contracts

- [x] Read all relevant `.trellis/spec/backend/` contracts and cross-layer/reuse guides.
- [x] Enumerate tracked project-owned source, test, script, workflow, and documentation files.
- [x] Record line counts and classify >1,000-line files as project-owned versus generated/managed.
- [x] Capture the current branch, clean-state baseline, recent substantive changes, and excluded generated surfaces.

## 2. Audit backend runtime

- [x] Review `main.go`, `router.go`, and `handler.go` for composition, middleware, response shaping, JSON fast/slow path parity, and thin-boundary discipline.
- [x] Review `config/`, `logs/`, `web/`, and public model ownership.
- [x] Review `service/` cache, refresh coordination, warmup/shutdown, token recovery, JW transport/protocol, errors, metrics, and builder normalization.
- [x] Trace lock ordering, context lifetimes, atomic cache publication, stale/partial variants, and exported seams.
- [x] Compare risky paths with focused tests and benchmarks.

## 3. Audit frontend runtime

- [x] Review the API wire/domain boundary and response normalization.
- [x] Review `useTodayClassrooms`, reload scheduling, visibility/retry behavior, and SWR cache assumptions.
- [x] Review selection/campus/time state ownership and effect-based reconciliation for simpler models.
- [x] Review component/CSS boundaries, Panel/native replacements, modal/table composition, accessibility, and reusable helpers.
- [x] Review type strictness, optionality/casts, duplicate parsing, test structure, dependency boundaries, and bundle configuration.

## 4. Audit operations, release, and docs

- [x] Review `scripts/install.sh` and `scripts/install_test.sh` with special attention to the >1,000-line blocker presumption, transaction stages, rollback, config persistence, and decomposition opportunities.
- [x] Review `scripts/release.sh`, changelog extraction, Taskfile, and GitHub Actions for duplicated policy, atomicity, and command drift.
- [x] Trace `.env.example`/installer/systemd/runtime config and timeout budgets across code and docs.
- [x] Verify release artifact names/layout and documentation consistency.

## 5. Cross-layer and reuse pass

- [x] Trace JW payload → backend model → JSON envelope → frontend normalizer → render behavior.
- [x] Trace installer inputs → persisted env → `config.Load` → service composition.
- [x] Trace frontend build → `web/dist` → tagged build → release package.
- [x] Search for repeated parsing/constants/conditionals and identify the canonical owner before proposing reuse.
- [x] Consolidate local symptoms into structural root causes and code-judo alternatives.

## 6. Verification

Run from repository root unless noted:

```bash
git diff --check
gofmt -l .
go vet ./...
go mod tidy -diff
go mod verify
go test -race ./...
go build ./...

pnpm -C frontend install --frozen-lockfile
pnpm -C frontend lint
pnpm -C frontend typecheck
pnpm -C frontend test
pnpm -C frontend build
node frontend/scripts/check-bundle-size.mjs
pnpm -C frontend audit:prod
pnpm -C frontend audit:dev

rm -rf web/dist && cp -r frontend/dist web/dist
go build -tags embed_assets ./...

bash scripts/install_test.sh
shellcheck scripts/*.sh
go run golang.org/x/vuln/cmd/govulncheck@v1.5.0 ./...
```

- [x] Record exact results, versions, failures, and skipped checks.
- [x] Add targeted read-only probes or temporary commands where broad tests do not exercise a candidate finding; do not commit product changes.

## 7. Produce and verify report

- [x] Write `.trellis/tasks/09-01-perf-quality-review/review.md` with verdict, prioritized findings, code-judo opportunities, size assessment, verification results, scope, and remediation order.
- [x] Ensure every blocker/high finding has exact anchors and independent evidence.
- [x] Remove speculative or duplicate comments and keep minor nits subordinate.
- [x] Run a final Trellis quality review over the report for spec compliance and false-positive control.

## Rollback and Safety

This task is read-only for product code. Verification may generate ignored build outputs (`frontend/dist`, `web/dist`, binaries); remove or leave only ignored outputs as appropriate. If any tracked file outside the task directory changes, stop and restore it before completing the audit.
