# Full Repository Strict Code Quality Review

## Goal

Perform an unusually strict, repository-wide code-quality audit that identifies high-conviction structural regressions, correctness risks, boundary leaks, maintainability problems, and code-judo opportunities to delete complexity without changing behavior. Produce a prioritized, evidence-backed review report rather than cosmetic nitpicks.

## Background

- The repository is a Go service with a React/Vite frontend, shell-based deployment/release automation, GitHub Actions quality/release pipelines, and Trellis-managed development metadata.
- The user explicitly selected an entire-repository review rather than a diff-only review.
- Project-owned files over 1,000 lines currently include `scripts/install.sh` (1,218) and `scripts/install_test.sh` (1,104). These require explicit decomposition analysis under the supplied review standard.
- Generated or managed files over 1,000 lines include `frontend/pnpm-lock.yaml`, `.pi/extensions/trellis/index.ts`, and `.kiro/hooks/inject-subagent-context.py`; they are not treated as ordinary hand-maintained application modules, but their generation/ownership justification must be recorded.
- Planning exploration identified high-risk areas that require verification, including refresh/token concurrency, manually spliced JSON, frontend SWR/state reconciliation, installer transactions/config persistence, timeout/config policy drift, and local/CI quality-gate alignment. These are hypotheses until confirmed by direct audit evidence.

## In Scope

- Go application and tests: root entry points, `config/`, `logs/`, `service/`, `service/model/`, and `web/`.
- Frontend source, tests, styles, API models/normalization, state/data hooks, build configuration, and bundle-budget tooling under `frontend/`.
- Project-owned operations and release surfaces: `scripts/`, `Taskfile.yml`, `.github/workflows/`, `.env.example`, `README.md`, `docs/`, and `CHANGELOG.md` where they define or document executable contracts.
- Cross-layer contracts: environment/config ownership, installer-to-runtime configuration, JW-to-service-to-API-to-frontend data flow, timeout/retry budgets, frontend asset embedding, and release artifact composition.
- Test architecture, verification coverage, repository hygiene, file-size/decomposition, canonical helper reuse, and consistency between source, tests, docs, specs, and CI.
- The recent substantive performance change (`daa7b98`) as part of the repository-wide audit, including its fast-path serialization and native frontend component replacements.

## Out of Scope

- Line-by-line quality review of Trellis/agent harness internals under `.trellis/`, `.agents/`, `.kiro/`, and `.pi/`; these are managed development infrastructure rather than the BUPT-EC product. Their project integration and generated-file ownership may still be checked.
- Structural review of generated dependency lockfiles or build artifacts.
- Real JW requests requiring credentials.
- Behavior-changing feature implementation or automatic fixes. Product-code changes require separate approval after findings are presented.
- Purely cosmetic style preferences without measurable maintainability or correctness impact.

## Requirements

### R1: Structural and architectural review

Audit ownership boundaries, module cohesion, abstraction value, state location, orchestration, and coupling. Search for a simpler reframing that can remove concepts, branches, wrappers, flags, or synchronization rather than merely moving complexity.

### R2: Spaghetti and complexity review

Flag scattered special cases, repeated conditionals, ad-hoc fallback paths, thin wrappers, magic behavior, sequential orchestration, and partial-update flows when a direct or atomic design is available.

### R3: Type and boundary review

Review Go interfaces/models, JSON contracts, TypeScript wire/domain boundaries, optionality/casts, config parsing, error normalization, and frontend state ownership. Prefer one canonical owner for parsing, validation, and projection.

### R4: File-size and decomposition review

Measure project-owned source/test files. Any file above 1,000 lines is presumptively blocking unless a strong structural reason exists. Evaluate whether decomposition reduces concepts and risk rather than splitting mechanically.

### R5: Correctness and concurrency review

Inspect cache freshness, refresh singleflight/backoff, token recovery, lifecycle/shutdown, manually assembled JSON, HTTP middleware, SWR retry/visibility behavior, selection reconciliation, installer commit/rollback, and release orchestration for race, drift, or partial-state risks.

### R6: Canonical reuse and cross-layer consistency

Search before alleging duplication. Compare implementation with `.trellis/spec/backend/`, existing helpers, tests, docs, Taskfile, and CI. Flag duplicated policy or logic only after identifying its canonical owner and consumers.

### R7: Verification

Run the broad repository checks that do not require private credentials, including Go race tests/build/vet/module checks, frontend lint/typecheck/tests/build/size/audits, shell tests/lint, embed build, vulnerability scan when network/tooling permits, and `git diff --check`. Report any unavailable check explicitly.

### R8: Review report

Create `.trellis/tasks/09-01-perf-quality-review/review.md`. For each significant finding include severity, exact `file:line` anchors, evidence, impact, why it violates the quality bar, and the preferred remedy or code-judo restructuring. Separate confirmed defects from design improvements and explicitly state approval status.

## Acceptance Criteria

- [x] Every in-scope subsystem is audited: backend, frontend, operations/release, tests/quality infrastructure, and cross-layer contracts.
- [x] All project-owned files above 1,000 lines are explicitly evaluated; generated/managed exemptions are justified.
- [x] Every blocker/high finding is independently verified against actual code and relevant tests/specs, not copied from exploratory hypotheses.
- [x] Findings are deduplicated and ordered by structural impact and severity; low-value style nits do not obscure major issues.
- [x] Each significant finding has precise anchors, observable impact, and an actionable preferred remedy.
- [x] Obvious code-judo opportunities are described concretely, including which concepts/branches/layers could disappear.
- [x] Relevant verification commands are run, with pass/fail/skip results recorded in `review.md`.
- [x] The report distinguishes correctness defects, maintainability blockers, non-blocking improvements, and uncertain/deferred risks.
- [x] The report ends with a clear approval decision under the supplied strict approval bar.
- [x] No product-code changes are made during this review-only task.

## Key Decisions

- Scope is the entire project-owned repository, not only the latest commit.
- The audit is read-only and report-producing; fixes are a separate approval boundary.
- Managed AI/Trellis harness internals and generated lockfiles are excluded from line-by-line product review, while their ownership and repository-health implications remain visible.
- High-confidence structural findings take precedence over a large volume of minor comments.
