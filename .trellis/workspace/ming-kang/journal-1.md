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


## Session 11: 重构批次1：工程卫生速赢（hygiene-quick-wins）

**Date**: 2026-07-26
**Task**: 重构批次1：工程卫生速赢（hygiene-quick-wins）
**Branch**: `main`

### Summary

完成 arch-simplify-refactor 任务树的子任务1：CI 提速与加固（pnpm 缓存、frontend-dist artifact 复用、timeout、go run govulncheck）、发布二进制版本注入（-trimpath -ldflags -X main.version，/readyz 新增 version 字段）、删除未使用的 dayjs、.gitignore/.env.example/docs 卫生修复、accepts_gzip_test.go 更名 router_test.go。经 3 分区并行实现 + trellis-check + 双镜头对抗验证，go test -race 与 pnpm 全套全绿。spec 沉淀：版本注入 gotcha 与 CI 工作流编辑规则。

### Git Commits

| Hash | Message |
|------|---------|
| `da6fd95` | (see git log) |
| `01bfa3d` | (see git log) |
| `57aff63` | (see git log) |
| `9ad81ec` | (see git log) |
| `af371b8` | (see git log) |
| `6b14d10` | (see git log) |

### Status

[OK] **Completed**


## Session 12: 重构批次2：后端减法（backend-subtraction）

**Date**: 2026-07-26
**Task**: 重构批次2：后端减法（backend-subtraction）
**Branch**: `main`

### Summary

完成 arch-simplify-refactor 子任务2：删除 cache/ 包与停更 8 年的 go-cache（换 atomic.Pointer 单值存储，Date 跨天守卫保留）、NoopMetrics 默认注入（删 14 处防御检查）、死代码清理（QueryResponse/LogIDKey/forceRefresh/Login/reflect 判空×3）、退避阶梯合一、循环变量清理、RuntimeStatus omitzero（wire 等价）、白盒测试收敛至 export_test.go 种子函数、1438 行测试拆 11 文件 + integration tag。生产代码净删 249 行。经 trellis-check 零缺陷核对 + 契约/完整性双镜头对抗验证，全部契约逐字节不变。插曲：Step 4 代理 OOM 中断但工作已完成，逐项核验后照常提交。

### Git Commits

| Hash | Message |
|------|---------|
| `6f49267` | (see git log) |
| `2962903` | (see git log) |
| `4bc181f` | (see git log) |
| `a70d8ef` | (see git log) |
| `8d7687f` | (see git log) |
| `7b91a29` | (see git log) |
| `8f60ae6` | (see git log) |

### Status

[OK] **Completed**


## Session 13: 重构批次3：HTTP 层重写（去 Gin、gzhttp、embed 解耦、Taskfile）

**Date**: 2026-07-27
**Task**: 重构批次3：HTTP 层重写（去 Gin、gzhttp、embed 解耦、Taskfile）
**Branch**: `main`

### Summary

去 Gin 换 net/http ServeMux：五路并行研究后按 4 步提交落地——web/ 构建标签双实现（裸克隆可编译）、HTTP 层重写一刀（gzhttp 压缩、X-Log-Id、静态缓存契约、recovery 最内层）、GIN_MODE 全仓清除（install.sh 位置参数重排）、Taskfile+CI embed 门禁+文档统一。module graph 76→40，净删 gin 及 ~30 传递模块。双镜头对抗验证裁决 15 条行为差异（B1-B15 入 design），真实二进制冒烟 11 项断言通过，spec 5 文档同步。

### Git Commits

| Hash | Message |
|------|---------|
| `01f97e2` | (see git log) |
| `8dccb36` | (see git log) |
| `d37eb09` | (see git log) |
| `256a091` | (see git log) |
| `35fb0cd` | (see git log) |
| `3223e53` | (see git log) |
| `85dea6e` | (see git log) |
| `10f61de` | (see git log) |

### Status

[OK] **Completed**
