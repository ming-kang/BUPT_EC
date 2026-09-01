# Repository Audit Planning Map

## Repository shape

The project contains a Go HTTP service, React/Vite frontend, shell installer/release automation, GitHub Actions pipelines, operational documentation, and Trellis-managed development infrastructure.

Project-owned runtime and operations code is concentrated in root Go files, `config/`, `logs/`, `service/`, `web/`, `frontend/`, `scripts/`, `.github/workflows/`, and `docs/`.

## Size inventory

Project-owned files above the strict 1,000-line threshold:

- `scripts/install.sh`: 1,218 lines.
- `scripts/install_test.sh`: 1,104 lines.

Generated/managed files above the threshold:

- `frontend/pnpm-lock.yaml`: generated dependency lockfile.
- `.pi/extensions/trellis/index.ts`: managed Trellis platform extension.

The audit must assess the two shell files as presumptive decomposition blockers and explicitly document why generated/managed files are exempt from ordinary module-size rules.

## High-risk areas to inspect

### Backend

- Refresh singleflight, backoff, stale/partial outcomes, lock ordering, detached contexts, and lifecycle drain across `service/realtime_data.go`, `refresh_coordinator.go`, and `warmup.go`.
- Token/API URL caching and concurrent auth recovery in `service/token_manager.go`.
- Fast versus slow `/api/get_data` response construction in `handler.go`, `router.go`, and `service/classroom_service.go`, including exact JSON/header parity and tests.
- Runtime configuration ownership, error safety, logging correlation, and embedding boundaries.

### Frontend

- `useTodayClassrooms.ts` SWR cache assumptions, visibility/retry scheduling, and render-state derivation.
- Campus/time selection reconciliation and effect-synchronized state.
- Duplicate wire-value normalization or time parsing.
- Native Panel/Tag/Typography replacements and whether the abstractions are direct and behavior-preserving.
- Bundle budget after the recent size reduction.

### Operations and release

- `scripts/install.sh` mixes prompting, parsing, download verification, rendering, transaction commit, health validation, and rollback in one file.
- Installer environment persistence may drift from runtime-supported variables and installer-only metadata.
- Multi-target commit/rollback, same-filesystem atomic replacements, process-death windows, and recovery behavior.
- Timeout/config constants and comments across backend, frontend, installer, and docs.
- Local Taskfile versus reusable quality workflow responsibilities.

### Quality infrastructure

- Coverage of manually spliced JSON fast paths and cache publication.
- Test architecture around high-concurrency service code and installer transactions.
- Bundle budget baseline and whether current headroom permits meaningful regressions.
- Full CI-equivalent command set, including shellcheck, installer tests, tagged build, audits, and vulnerability scan.

## False-positive controls

Planning observations are hypotheses, not final findings. Every significant report item must be re-verified against current code, tests, repository specs, comments, and command results. Intentional omissions documented by Taskfile/specs must not be reported as drift without demonstrating a real contract mismatch.
