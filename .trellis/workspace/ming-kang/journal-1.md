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


## Session 7: Make installer updates transactional

**Date**: 2026-07-10
**Task**: Make installer updates transactional
**Branch**: `main`

### Summary

Staged and verified all release candidates before mutation, added atomic file commits with full installation snapshots and automatic rollback, covered first-install and upgrade failure paths with mocked system commands, and synchronized deployment docs plus executable installer specs.

### Main Changes

(Add details)

### Git Commits

| Hash | Message |
|------|---------|
| `17efab4` | (see git log) |
| `75752f5` | (see git log) |

### Testing

- [OK] (Add test results)

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 8: Complete reliability audit hardening

**Date**: 2026-07-10
**Task**: Complete reliability audit hardening
**Branch**: `main`

### Summary

Closed the six-child reliability audit hardening program after a parent-level cross-layer review covering refresh outcomes, Shanghai-day frontend validity, cancellable warmup recovery, concurrent token auth recovery, installer release selection, and transactional installer rollback. Full Go, frontend, shell, documentation, and release-asset checks passed; govulncheck remains covered by CI because it is unavailable locally.

### Main Changes

(Add details)

### Git Commits

| Hash | Message |
|------|---------|
| `363ac0f` | (see git log) |
| `c9a2543` | (see git log) |
| `9447524` | (see git log) |
| `51b3019` | (see git log) |
| `8dd6851` | (see git log) |
| `17efab4` | (see git log) |
| `ec3f2ff` | (see git log) |

### Testing

- [OK] (Add test results)

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 9: 运行时配置与依赖组装

**Date**: 2026-07-10
**Task**: 运行时配置与依赖组装
**Branch**: `main`

### Summary

集中启动配置加载与校验，在 main composition root 显式组装 cache、HTTP、JW client 和 ClassroomService；移除生产路径全局依赖与热路径环境读取，并补齐测试、文档和后端规范。

### Main Changes

(Add details)

### Git Commits

| Hash | Message |
|------|---------|
| `d4bda80` | (see git log) |

### Testing

- [OK] (Add test results)

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 10: Dependency security refresh

**Date**: 2026-07-10
**Task**: Dependency security refresh
**Branch**: `main`

### Summary

Raised the Go security floor to 1.25.12, patched quic-go, refreshed the Vite and ESLint toolchain, added production/full frontend audit gates, synchronized documentation and executable Trellis quality contracts, and archived the completed P0 child task.

### Main Changes

(Add details)

### Git Commits

| Hash | Message |
|------|---------|
| `215d736` | (see git log) |
| `3c98705` | (see git log) |
| `2a66e93` | (see git log) |

### Testing

- [OK] (Add test results)

### Status

[OK] **Completed**

### Next Steps

- None - task complete
