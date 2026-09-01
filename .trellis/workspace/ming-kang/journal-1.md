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


## Session 14: 重构批次4：前端现代化（frontend-modernize）

**Date**: 2026-07-27
**Task**: 重构批次4：前端现代化（frontend-modernize）
**Branch**: `main`

### Summary

四路并行研究后按 8 步提交落地：antd 5.29/React 18.3 升级并清理 4 条 CVE overrides、Vite 7 并显式锁 build.target=safari14、normalizeResponse 去 throw 与 ApiError/log_id 透传、SWR 替换手写数据层（spike 验证可测性后走主路，胶水约 40 行，逃生出口未触发）、antd Table 换原生表格并内联图标（懒加载 chunk 106KB→3.26KB）、antd-vendor 分组+visualizer+CI gzip 体积预算、Modal/campus/裁剪渲染期派生与时钟重同步、ErrorBoundary 与 aria-pressed。总 gzip 293,407→210,143 B（-28.4%），首屏因 antd 归组升 23.6KB 换 vendor hash 稳定（已记录）。测试 69→112，契约/UI 双镜头对抗验证通过，全链路冒烟 11 项 pass。ToggleButtonGroup 完整档经判据评估跳过。

### Git Commits

| Hash | Message |
|------|---------|
| `edb4d35` | (see git log) |
| `c992435` | (see git log) |
| `d574915` | (see git log) |
| `282242c` | (see git log) |
| `2135321` | (see git log) |
| `8689af2` | (see git log) |
| `6510b12` | (see git log) |
| `ebe5a25` | (see git log) |
| `d57c2ca` | (see git log) |
| `8daed66` | (see git log) |
| `84a1dfe` | (see git log) |
| `c3b6791` | (see git log) |

### Status

[OK] **Completed**


## Session 15: 架构简化重构收尾：父任务集成审查与归档

**Date**: 2026-07-27
**Task**: 架构简化重构收尾：父任务集成审查与归档
**Branch**: `main`

### Summary

四个子任务全部归档后的父任务集成审查：三镜头并行（跨子任务验收标准 / 文档与 CHANGELOG 一致性 / 完整构建与全栈冒烟）全部 pass、零 blocker。实测总账：go.mod 声明依赖 43→15 行、module graph 77→40、前端 gzip 293,407→210,143 B（-28.4%）、四子任务合计 147 文件 +9888/-3645。批次④越界检查通过（无过期模型收敛/生命周期重构/TS/Dockerfile/goreleaser）。冒烟覆盖 task build 全链路 + 8 组 HTTP 断言 + 安全红线 grep（响应侧零泄漏）。审查发现 7 处文档缺口已修补（/readyz version 字段与升级校验、静态缓存契约与排障、体积预算 CI 步、根级 ErrorBoundary、native equivalent 版本注入措辞、父 PRD 依赖口径）。遗留：人工视觉回归待用户执行；服务端 WARN 日志仍原样打印 transport 错误（既有行为，非本轮引入）。

### Git Commits

| Hash | Message |
|------|---------|
| `c54463b` | (see git log) |

### Status

[OK] **Completed**


## Session 16: 全面审计：性能与可维护性优化机会

**Date**: 2026-08-07
**Task**: 全面审计：性能与可维护性优化机会
**Branch**: `main`

### Summary

对 07-26 减法重构后的 `main` 做只读审计（交付形态 A，不改产品代码）。产出 `research/audit-{backend,frontend,engineering}.md` 与跨层 `backlog.md`；对照 07-26 基线与批次④遗留项。批次④判定：lifecycle Run/Shutdown 与 TypeScript **继续做**；过期模型/singleflight/React19/goreleaser/install 大改 **降级**；Dockerfile **关闭**。`trellis-check` PASS。归档路径：`archive/2026-08/08-07-project-audit-optimize`。

### Main Changes

- 写入三份审计报告 + 优先级 backlog（建议后续任务：lifecycle-run-shutdown、release-hygiene 等）
- 产品树零 diff

### Git Commits

| Hash | Message |
|------|---------|
| `1403f80` | (see git log) |

### Testing

- [OK] trellis-check PASS；抽样 file:line 锚点与当前源码一致

### Status

[OK] **Completed**

## 2026-08-15 (evening): backlog 实现批次 —— 4 任务连做

### Task

按 08-07 审计 backlog 优先级顺序连做 4 个实现任务（每个独立规划→实现→检查→归档）：

1. **08-15-lifecycle-run-shutdown**（B-01+B-02，M 级复杂任务）
   - StartWarmup/WaitBackground → Run(ctx) error / Shutdown(ctx) error；5 侧门字段收敛为 lifecycleCtx/lifecycleCancel/schedulerDone；二次 Run 返回 ErrAlreadyRunning
   - B-01 取消传播：worker 与 token 共享操作经 context.AfterFunc(lifecycle, cancel) 桥接 SIGTERM 取消（复合 cancel 防泄漏）；单 waiter 隔离保持（WithoutCancel）
   - 独立 explorer 审查发现 2 处边界（已取消 lifecycle 下 token 操作拿满 12s 预算；测试日志残留）已修
   - commit `6c806bf`
2. **08-15-release-hygiene**（E-04+E-05，轻量 PRD-only）
   - nightly 发布去 delete-then-create：tag force-move + gh release upload --clobber，资产 URL 无 404 空窗
   - release.sh 两处 GNU-only sed -i → awk + 三道事后校验（防 macOS 静默损坏）；临时目录端到端演练验证语义一致 + 负路径拦截
   - commit `70c8644`
3. **08-15-frontend-picker-dedup**（F-03+F-06，M 级）
   - 抽 ToggleButtonGroup/ToggleButton 共享组件（aria-pressed 仅在 boolean pressed 时输出——全选/设置按钮零回退）；三 Picker 接入 + CSS 收编
   - F-06：formatShanghaiDateTime 替换 CampusSettingsModal 裸 toLocaleString
   - 既有测试断言零修改通过；体积 210394/230888 B
   - commit `bf9655a`
4. **08-15-api-surface-hygiene**（B-07+B-11，轻量）
   - /readyz 默认只返回 status+version；READYZ_DIAGNOSTICS=1 恢复全量诊断（拒绝 loopback 检测：同机反代下恒 loopback，XFF 可伪造）
   - 成功信封补 log_id 与错误路径对称；gzip 测试改剥离 log_id 比较
   - commit `cf6c238`

### Main Changes

- 4 个任务全部完成归档（archive/2026-08/）；spec（runtime-state-and-cache、api-contract、directory-structure）、docs、CHANGELOG 均同步
- 环境注记：pi 环境无 trellis 子代理可执行文件（spawn pi ENOENT），trellis-implement/check 回退为内联实现 + 原生 subagent 审查

### Testing

- [OK] 每任务 go test -race ./... 全绿（lifecycle 加 -count=3）；gofmt/vet 干净；embed 构建通过；前端 119 测试全绿 + lint + build + 体积预算内
- [SKIPPED] pnpm audit 失败为存量 devDependency 漏洞（vite/postcss/nanoid 链），与本次改动无关

### Status

[OK] **Completed** — backlog 剩余：frontend-typescript（L 级）、classroom-display-contract（需产品确认）、utils-into-service（S）、ETag/冷路径（按需/需批准）


## Session 17: Fix devDependency audit findings via lockfile patch bumps

**Date**: 2026-08-21
**Task**: Fix devDependency audit findings via lockfile patch bumps
**Branch**: `main`

### Summary

Cleared the 3 remaining high-severity pnpm audit findings (brace-expansion 1.1.18, js-yaml 4.3.1, nanoid 3.3.18) via scoped pnpm update within declared semver ranges - lockfile-only change, no overrides, no source changes. All gates green: audit/audit:prod/audit:dev zero vulns, lint clean, 119 frontend tests pass, build ok, bundle 210394 B within budget (unchanged). Independent explorer review PASS on all 5 items. audit:dev no longer needs manual skipping in task check.

### Git Commits

| Hash | Message |
|------|---------|
| `d7af573` | (see git log) |

### Status

[OK] **Completed**

## Session 18: Create TS-migration parent task; complete utils-into-service

**Date**: 2026-08-21
**Task**: 08-21-utils-into-service (completed & archived); 08-21-frontend-typescript (parent shell created, --no-start)
**Branch**: `main`

### Summary

Per remaining audit backlog, created two Trellis tasks then executed the lightweight one:
1. **08-21-utils-into-service** (B-08, PRD-only lightweight): moved `utils/http.go` + tests into
   `service/jw_http.go` / `service/jw_http_test.go`; deleted `utils/` package. Renames:
   `NewHTTPClient`→`NewJWHTTPClient` (exported for composition root), `HttpGet`/`HttpPostForm`/
   `HttpPostWithHeader`/`CheckRedirect` → unexported; `HTTPDoer` stays exported as the
   `NewJWClient` injection seam. Behavior byte-identical (redirect rejection, 5 MiB cap,
   transport timeouts). Synced AGENTS.md structure sentence, docs/development.md (tree +
   composition root + redirect note), and `.trellis/spec/backend/directory-structure.md`
   (layout tree, signatures block, Correct example, Service Package Ownership now lists jw_http.go).
2. **08-21-frontend-typescript**: parent task created with --no-start; children to be split during
   its planning phase (type generation / core modules / component layer / prop-types removal;
   React 19 at the end).

### Main Changes

- Independent explorer review: 6/6 PASS (behavior diff vs HEAD:utils/http.go byte-identical apart
  from renames + two additive doc comments; no residual utils references repo-wide; exported
  surface minimal; spec/docs consistent; build/vet/gofmt/race green).

### Git Commits

| Hash | Message |
|------|---------|
| `331c6b4` | refactor(service): absorb utils HTTP client into service as jw_http.go |
| `210b0be`/`c01b8ab` | chore(task): archive 08-21-utils-into-service (+ add frontend-typescript parent dir) |

### Testing

- [OK] go test -race ./... all green (-count=1); go vet clean; gofmt clean
- [OK] embed build verified: cp frontend/dist → web/dist + go build -tags embed_assets
- [OK] rg confirms zero BUPT_EC/utils or utils.* helper references in live code/docs/spec

### Status

[OK] **Completed** — backlog 剩余：frontend-typescript（父任务已建，待规划拆子）、classroom-display-contract（需产品确认）、ETag/冷路径（按需/需批准）

## Session 19: Frontend TypeScript migration — full campaign complete

**Date**: 2026-08-21
**Task**: 08-21-frontend-typescript (parent) + children ts-foundation / ts-data-layer / ts-components / ts-cleanup-protypes — ALL COMPLETED & ARCHIVED
**Branch**: `main`

### Summary

Executed the planned F-01 campaign end-to-end in four reviewable batches, each independently reviewed by an explorer subagent against HEAD baselines before commit:

1. **ts-foundation** (`023b73a`): strict tsconfig (noEmit/bundler/allowJs), typescript-eslint additive in flat config, `typecheck` wired into package.json + Taskfile check + quality.yml; `src/api/types.ts` mirroring Go public structs + wire envelope; converted 7 pure helper modules (+tests). Review fixed LOW finding: scoped tseslint configs to ts/tsx so JS keeps espree.
2. **ts-data-layer** (`48f854c`): todayClassroomsResponse/selectionContext/SelectionProvider/useTodayClassrooms (+4 test suites). normalizeResponse(payload: unknown) keeps every runtime guard verbatim; typed discriminated result; SWR fetcher throw ladder line-for-line preserved. SelectionProvider dropped PropTypes early (no TS declarations exist). Review: PASS zero findings.
3. **ts-components** (`ef4ce29`): all 10 components + App/main + vitest setup → .tsx/.ts; src is now 100% TypeScript. ToggleButtonGroup made generic over option value type (formalizes numeric node values the old string-only PropTypes mis-declared). Review caught MEDIUM: index.html still pointed at main.jsx (worked only via Vite extension fallback) — fixed pre-commit.
4. **ts-cleanup-protypes** (`ff1d829`): prop-types dependency removed (retained in lockfile only as eslint-plugin-react transitive); docs synced (development.md tree/conventions/tests section, AGENTS.md style line).

### Key decisions honored from parent design

D1 handwritten types over codegen; D2 strict day-one, no checkJs phase; D3 runtime guards stay verbatim at boundaries (types describe post-validation shapes); D4 dependency-order batches with tests traveling with subjects.

### Git Commits

| Hash | Message |
|------|---------|
| `023b73a` | feat(frontend): introduce strict TypeScript foundation and convert pure helpers |
| `48f854c` | feat(frontend): convert data layer to strict TypeScript |
| `ef4ce29` | feat(frontend): convert all components and entry points to strict TypeScript |
| `ff1d829` | chore(frontend): remove unused prop-types dependency and sync docs |

### Testing

- [OK] Every batch: typecheck(strict)+lint(0 warn)+119 tests+build green; bundle 209,683 B gzip vs 230,888 budget (net −711 B vs pre-campaign)
- [OK] go build/vet/test green throughout; embed_assets build verified twice
- [OK] Zero behavior change: all pre-existing assertions byte-identical except mechanical annotations/casts; no new runtime deps

### Status

[OK] **Completed** — backlog 剩余：classroom-display-contract（需产品确认）、api-etag-preserialize（流量驱动）、cold-path-bounded-wait（需产品批准）；F-02 React 19 现已解锁（可独立立项）

## Session 20: classroom-display-contract — probe-settled F-05 + backend display_name

**Date**: 2026-08-21
**Task**: 08-21-classroom-display-contract (completed & archived)
**Branch**: `main`

### Summary

Pre-task research (two parallel explorers) settled ground truth for F-05/F-07 and audited B-04:
- **F-05 capacity**: JW has no seat field — capacity is the `(N)` suffix of each CLASSROOMS token
  (builder regex, Atoi error discarded). Live probe via integration test + .env credentials:
  **721 tokens, 0 suffix-less, 0 zero-capacity** ⇒ wire guarantees capacity ≥ 1; 0 only appears as
  parse degradation (suffix-less token → 未分组), where the frontend 未知 fallback is correct.
  Closed by specification, not code change.
- **F-07 display_name**: frontend aliasing was label-only (selection values always raw names), so
  moving rules backend-side is semantics-free. Implemented BuildingInfo.display_name following the
  RoomInfo.DisplayName precedent; picker falls back to raw name for pre-deploy same-day caches.
- **B-04 ETag research finding (task stays deferred)**: B-11's per-request body log_id makes
  marshal+hash ETag hit rate 0%; (stale, errKind) keying would serve stale data across refreshes
  (must key snapshot×variant). Only real payoff is 304 bandwidth at scale. If ever built: option A
  = pre-serialize data segment per snapshot×variant at Store time with fresh log_id envelope
  splice (no contract change); option B adds full 304 by moving log_id out of the success body.

### Git Commits

| Hash | Message |
|------|---------|
| `8b4c689` | feat(api): normalize building display_name server-side and pin capacity wire contract |
| `dbe1dca` | chore(task): archive 08-21-classroom-display-contract |

### Testing

- [OK] New: TestBuildingDisplayNameNormalization, TestBuildCampusInfoEmitsBuildingDisplayName,
      stale-cache fallback picker test (120 frontend tests total), live integration guard
      TestJWRoomTokensCarryPositiveCapacitySuffix (ran green against real JW twice)
- [OK] go test -race ./service, vet, gofmt clean; frontend typecheck/lint/120/build green;
      bundle 209,594 B within budget
- [OK] Review PASS after fixing Medium (missing fallback test) + Low ((00) vs Atoi zero check)

### Status

[OK] **Completed** — backlog 剩余：api-etag-preserialize（挂起，研究结论见任务归档）、cold-path-bounded-wait（需产品批准）、React 19（已解锁）、低优先级杂项

## Session 21: cold-path-bounded-wait — 5s bounded wait + Retry-After

**Date**: 2026-08-21
**Task**: 08-21-cold-path-bounded-wait (completed & archived)
**Branch**: `main`

### Summary

B-03 implemented per the approved planning artifacts (prd/design/implement):
- `ClassroomServiceOptions.ColdWaitTimeout` (default `service.DefaultColdWaitTimeout` = 5s, ≤0→default); cold-miss select gains a `time.NewTimer` arm returning new sentinel `ErrRefreshWaitTimeout` with `ObserveCacheServe("wait_timeout")`; refresh deliberately never cancelled (D3, worker runs on WithoutCancel context).
- Handler sets `Retry-After: 5` only when errors.Is matches ErrNoTodayCache/ErrRefreshWaitTimeout; SafeErrorMessage maps both to the existing 暂无 copy.
- httpWriteTimeout 45s→15s; main-package invariant tests rewritten against DefaultColdWaitTimeout (premise changed: handlers no longer hold sockets for the refresh budget).
- Frontend untouched (retry ladder rung 1 = 10s > Retry-After).

### Review findings fixed in-commit
- HIGH: normalizeCacheState allowlist coerced "wait_timeout"→"miss", silently defeating R4 observability — added to enum + Prometheus test asserting the label survives.
- MEDIUM/LOW doc sweep: operations.md budget table rewritten (cold-wait layer added), main.go + ClassroomRefreshLimit comments de-staled, quality-guidelines.md 45s number updated, success-path Retry-After absence asserted.

### Git Commits

| Hash | Message |
|------|---------|
| `28e5f2f` | feat(api): bound cold-start wait at 5s with 503 + Retry-After |
| `04fd7d1` | chore(task): archive 08-21-cold-path-bounded-wait |

### Testing

- [OK] New: TestColdMissWaitTimesOutThenConverges (gated refresh, timeout → convergence), TestColdWaitTimeoutDefaultsToFiveSeconds, Retry-After polarity handler tests ×3, Prometheus wait_timeout label test
- [OK] go test -race ./... -count=1 all green; vet/gofmt clean; embed build green; frontend 120 tests green (untouched)

### Status

[OK] **Completed** — backlog 剩余：React 19（已解锁）、api-etag-preserialize（挂起待流量）、E-03′/E-06/F-04/B-05（条件触发）

## Session 22: React 19 compatibility migration

**Date**: 2026-08-22
**Task**: 08-22-react-19 (completed & archived)
**Branch**: `main`

### Summary

Completed backlog item F-02 as a behavior-preserving compatibility migration:
- React/React DOM 18.3.1 → 19.2.8; `@types/react` / `@types/react-dom` → 19.2.18 / 19.2.4. The pnpm graph resolves one React 19 runtime; lockfile package changes are limited to the expected runtime/types/scheduler replacements, official patch addition, and obsolete `@types/prop-types` removal.
- Kept antd 5.29.3 and added official `@ant-design/v5-patch-for-react-19` 1.0.3. `main.tsx` imports it before rendering so Button waves/static overlays use React 19 `createRoot` compatibility.
- Direct component tests bypass `main.tsx`, so jsdom setup awaits the same patch only when `window` exists. An initial static setup import was rejected because it loaded the full antd graph into pure node suites; the conditional import keeps those suites isolated.
- No component/hook behavior or assertions changed. CHANGELOG, dependency-line spec, bundle baseline, and stale current `.js`/`.jsx` documentation paths were synchronized; unrelated stale success-`log_id` spec semantics remain deferred.

### Git Commits

| Hash | Message |
|------|---------|
| `7f1e63c` | chore(frontend): migrate to React 19 |
| `0706fea` | chore(task): archive 08-22-react-19 |

### Testing

- [OK] Frontend frozen install, strict typecheck, lint, 18 files / 120 tests, production build (1,494 modules), size, audit:prod, and audit:dev.
- [OK] Only the pre-migration `TimeoutNaNWarning` and jsdom pseudo-element `getComputedStyle` messages remain; no React/antd/act/ref/root/unmount warning was introduced.
- [OK] Bundle 224,343 B gzip (+14,749 B), within unchanged 230,888 B budget (6,545 B headroom); `react-vendor` and `antd-vendor` topology preserved.
- [OK] `task check`, `task test`, `task build`, tagless + embed `./...` builds, govulncheck with `GOTOOLCHAIN=go1.25.13`, installer tests, ShellCheck, and diff check.
- [OK] Independent final review PASS. Trellis Pi subagents were unavailable (`spawn pi ENOENT`), so implementation/check used native fallback agents plus main-session verification.

### Status

[OK] **Completed** — backlog 剩余：api-etag-preserialize（挂起待流量）、E-03′/E-06/F-04/B-05（条件触发）


## Session 18: Pause bupt-ec CLI implementation before independent review

**Date**: 2026-09-02
**Task**: Pause bupt-ec CLI implementation before independent review
**Branch**: `main`

### Summary

CLI/metadata/transaction/release implementation is present and implement-agent gates were reported green; independent full-scope review was interrupted and remains required.

### Main Changes

- Added CLI, metadata, Installer transaction, release composition, tests, specs, and docs in the uncommitted worktree.

### Git Commits

(No commits - planning session)

### Testing

- [OK] Main session confirmed git diff --check and Trellis context validation; implement worker reported full gates green.

### Status

[OK] **Completed**

### Next Steps

- Read .trellis/tasks/08-22-bupt-ec-cli/research/pause-handoff-2026-09-01.md and rerun independent full-scope check before commit.
