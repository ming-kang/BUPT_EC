# Journal - ming-kang (Part 1)

> AI development session journal
> Started: 2026-07-06

---



## Session 1: Bootstrap Trellis specs

**Date**: 2026-07-06
**Task**: Bootstrap Trellis specs
**Branch**: `main`

### Summary

Bootstrapped project-local Trellis guidance for the BUPT_EC backend, replacing template placeholders with source-backed directory, runtime/cache, API contract, error handling, logging, and quality specs.

### Main Changes

(Add details)

### Git Commits

| Hash | Message |
|------|---------|
| `20ecc33` | (see git log) |

### Testing

- [OK] (Add test results)

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 2: Align installer release selection

**Date**: 2026-07-10
**Task**: Align installer release selection
**Branch**: `main`

### Summary

Implemented explicit stable/nightly/tag selection, persisted RELEASE_VERSION, added installer behavior tests and CI coverage, and created the reliability hardening task tree.

### Main Changes

(Add details)

### Git Commits

| Hash | Message |
|------|---------|
| `363ac0f` | (see git log) |
| `36bc41e` | (see git log) |

### Testing

- [OK] (Add test results)

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 3: Model refresh outcomes explicitly

**Date**: 2026-07-10
**Task**: Model refresh outcomes explicitly
**Branch**: `main`

### Summary

Implemented full/partial/failed refresh outcomes, partial campus diagnostics, latest-error precedence, tests, docs, and code specs.

### Main Changes

(Add details)

### Git Commits

| Hash | Message |
|------|---------|
| `c9a2543` | (see git log) |
| `ebbaaf5` | (see git log) |

### Testing

- [OK] (Add test results)

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 4: Harden frontend cache validity and retries

**Date**: 2026-07-10
**Task**: Harden frontend cache validity and retries
**Branch**: `main`

### Summary

Added shared Shanghai business-day snapshot validation, hard-empty cross-day handling, bounded client retry backoff, 30-second partial polling, campus-specific warnings, regression tests, docs, and executable specs.

### Main Changes

(Add details)

### Git Commits

| Hash | Message |
|------|---------|
| `9447524` | (see git log) |
| `422f3d7` | (see git log) |

### Testing

- [OK] (Add test results)

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 5: Make warmup lifecycle cancellable

**Date**: 2026-07-10
**Task**: Make warmup lifecycle cancellable
**Branch**: `main`

### Summary

Added a single context-cancellable warmup scheduler, deterministic cache-state retry policy, cross-midnight backoff recovery, safe background worker draining, graceful shutdown ordering, tests, docs, and runtime specs.

### Main Changes

(Add details)

### Git Commits

| Hash | Message |
|------|---------|
| `51b3019` | (see git log) |
| `109dd6a` | (see git log) |

### Testing

- [OK] (Add test results)

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 6: Coordinate token auth recovery

**Date**: 2026-07-10
**Task**: Coordinate token auth recovery
**Branch**: `main`

### Summary

Added token-source tracking, failed-token-aware singleflight auth recovery, detached bounded login/API URL operations, per-waiter cancellation, concurrency regressions, docs, and executable specs.

### Main Changes

(Add details)

### Git Commits

| Hash | Message |
|------|---------|
| `8dd6851` | (see git log) |
| `de1970b` | (see git log) |

### Testing

- [OK] (Add test results)

### Status

[OK] **Completed**

### Next Steps

- None - task complete
