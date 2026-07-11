# 依赖、规范与验收证据同步实施计划

## 0. Preconditions

- [x] metrics child 已提交/归档并记录验证证据。
- [x] backend jitter child 已提交/归档并记录验证证据。
- [x] frontend jitter child 已提交/归档并记录验证证据。
- [x] installer URL child 已提交/归档并记录验证证据。
- [x] Unicode sanitizer child 已提交/归档并记录验证证据。
- [x] 读取五个 child 最终代码、tests、spec/changelog diff，不根据规划假设行为。

## 1. Go module metadata

- [x] 使用 GOTOOLCHAIN=go1.25.12 运行 go mod tidy。
- [x] 审查 go.mod 只调整 direct/indirect 分类，不升级版本。
- [x] 审查 go.sum 只补齐 tidy 所需 checksum。
- [x] 运行 go mod tidy -diff、go mod verify 和 fresh module download/build。

## 2. Reusable quality gate

- [x] 在 quality.yml Go setup 后增加 go mod tidy -diff。
- [x] 增加 go mod verify。
- [x] 保持 frontend→Go→installer/ShellCheck 顺序和现有 pins。
- [x] 运行 actionlint，复核 ci.yml/release.yml 仍只调用同一 reusable workflow（本地无 actionlint；人工确认 callers 未变，CI 跑 shellcheck/actionlint）。

## 3. Contract documentation

- [x] 更新 AGENTS cache/Clock/metrics/backoff/frontend/installer/logging 合同。
- [x] 更新 docs/development.md 项目结构和本地质量命令。
- [x] 更新 docs/operations.md metrics、backoff、polling、deadline 和 alert 说明（已与前置 child 对齐；本任务复核）。
- [x] 更新 docs/deployment.md 与 docs/upgrading.md 镜像 URL/协议/日志边界（已由 installer child 写入；本任务复核无残留冲突）。
- [x] 必要时更新 docs/release.md 的新增 module gate。
- [x] 更新 backend directory/runtime/api/error/logging/quality specs。
- [x] 核对 README 是否需要 operator-facing 变化（无需额外改动）。

## 4. Changelog convergence

- [x] 确认每个 owning child 已在自身 commit 添加用户可见 bullet。
- [x] 合并 Unreleased 重复分类标题。
- [x] 删除或重写旧 5s、5/10/20/30、fixed total 30s 等冲突描述。
- [x] 保留所有独立安全、功能和依赖变化。
- [x] 运行 scripts/extract-changelog.sh Unreleased 并人工检查输出。

## 5. Negative consistency audit

- [x] 搜索 CacheStore 和旧 cache.New signature。
- [x] 搜索固定 total failure 30s 描述。
- [x] 搜索旧 frontend retry/stale interval。
- [x] 搜索 metrics double compression 或 login metric 未接线描述。
- [x] 搜索允许 userinfo/query/non-HTTP mirror 或完整 URL 输出的描述。
- [x] 搜索 ASCII-only sanitizer 描述。
- [x] 对每个残留命中判断是历史语境还是当前合同。

## 6. Evidence audit

- [x] 不修改旧 archive checklist/commit。
- [x] 检查五个前置 child 的 implement checklist、validation summary 和 task commit。
- [x] 本任务按实际执行增量勾选并记录每条验证结果。
- [x] 为父任务记录 child → commit map。
- [x] archive 前确认 task.json.commit 只能填写已存在的 commit（archive 后由 chore commit 记录）。

## 7. Full validation

见下方 Verification 结果。

## Review Gates

- 本任务不得修改 production behavior；发现不一致必须退回 owning child。
- 不把 dependency version bump 混入 tidy commit。
- current docs/spec 必须以测试和代码为准，不以旧 task 文档为准。
- 旧 archive 不得事后补证据。
- .agents/、.codex/、模板哈希、dist、logs 和临时产物不得提交。
- user-visible changelog bullet 必须仍可追溯到 owning behavior commit。

## Rollback Points

- go.mod/go.sum tidy metadata。
- reusable workflow tidy/verify gate。
- docs/spec convergence。
- changelog consolidation。
- evidence metadata 只记录真实 commit，不允许通过回滚删除失败历史。

## Child → commit map (behavior)

| Child | Behavior commit | Archive commit |
| --- | --- | --- |
| 07-10-metrics-endpoint-login-observability | `efd0d31` | `4a1d4db` |
| 07-10-adaptive-refresh-backoff-jitter | `ea76336` | `e5ac060` |
| 07-10-frontend-reload-deadline-jitter | `c584b97` | `75b1cd4` |
| 07-10-installer-mirror-url-safety | `30cbbad` | `e0afe5d` |
| 07-10-upstream-message-unicode-safety | `66987d4` | `af288e0` |
| 07-10-dependency-spec-evidence-hygiene | `f3a0d29` | `f6ea24f` |

### Historical evidence note

Older 07-10 archives outside this parent may lack verification summaries or
`task.json.commit` fields. Those archives are left unchanged; this parent and
its six children form the complete, non-retroactive evidence chain for the
2026-07-10 review remediation.

### Verification (2026-07-11)

```text
GOTOOLCHAIN=go1.25.12
gofmt -l .                         # empty
go mod tidy -diff                  # clean
go mod verify                      # all modules verified
go vet ./...                       # ok
go test -race ./...                # ok
go build ./...                     # ok
govulncheck@v1.5.0 ./...           # 0 vulns (must pin GOTOOLCHAIN; host 1.26.x alone fails)
bash -n scripts/*.sh               # ok
bash scripts/install_test.sh       # installer behavior tests passed
shellcheck / actionlint            # not on PATH (Windows); quality.yml still runs both in CI
bash scripts/extract-changelog.sh Unreleased  # non-empty; single ### headings
pnpm install --frozen-lockfile     # ok
pnpm lint / test (69) / build      # ok
pnpm audit:prod / audit:dev        # no known vulnerabilities
git diff --check                   # ok (CRLF note on task.json only)
rg CacheStore --glob '*.{md,go,yml}'  # no hits
.agents/ .codex/ template-hashes / frontend/dist not tracked
```