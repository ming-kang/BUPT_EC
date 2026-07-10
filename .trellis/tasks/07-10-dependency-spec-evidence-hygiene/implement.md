# 依赖、规范与验收证据同步实施计划

## 0. Preconditions

- [ ] metrics child 已提交/归档并记录验证证据。
- [ ] backend jitter child 已提交/归档并记录验证证据。
- [ ] frontend jitter child 已提交/归档并记录验证证据。
- [ ] installer URL child 已提交/归档并记录验证证据。
- [ ] Unicode sanitizer child 已提交/归档并记录验证证据。
- [ ] 读取五个 child 最终代码、tests、spec/changelog diff，不根据规划假设行为。

## 1. Go module metadata

- [ ] 使用 GOTOOLCHAIN=go1.25.12 运行 go mod tidy。
- [ ] 审查 go.mod 只调整 direct/indirect 分类，不升级版本。
- [ ] 审查 go.sum 只补齐 tidy 所需 checksum。
- [ ] 运行 go mod tidy -diff、go mod verify 和 fresh module download/build。

## 2. Reusable quality gate

- [ ] 在 quality.yml Go setup 后增加 go mod tidy -diff。
- [ ] 增加 go mod verify。
- [ ] 保持 frontend→Go→installer/ShellCheck 顺序和现有 pins。
- [ ] 运行 actionlint，复核 ci.yml/release.yml 仍只调用同一 reusable workflow。

## 3. Contract documentation

- [ ] 更新 AGENTS cache/Clock/metrics/backoff/frontend/installer/logging 合同。
- [ ] 更新 docs/development.md 项目结构和本地质量命令。
- [ ] 更新 docs/operations.md metrics、backoff、polling、deadline 和 alert 说明。
- [ ] 更新 docs/deployment.md 与 docs/upgrading.md 镜像 URL/协议/日志边界。
- [ ] 必要时更新 docs/release.md 的新增 module gate。
- [ ] 更新 backend directory/runtime/api/error/logging/quality specs。
- [ ] 核对 README 是否需要 operator-facing 变化。

## 4. Changelog convergence

- [ ] 确认每个 owning child 已在自身 commit 添加用户可见 bullet。
- [ ] 合并 Unreleased 重复分类标题。
- [ ] 删除或重写旧 5s、5/10/20/30、fixed total 30s 等冲突描述。
- [ ] 保留所有独立安全、功能和依赖变化。
- [ ] 运行 scripts/extract-changelog.sh Unreleased 并人工检查输出。

## 5. Negative consistency audit

- [ ] 搜索 CacheStore 和旧 cache.New signature。
- [ ] 搜索固定 total failure 30s 描述。
- [ ] 搜索旧 frontend retry/stale interval。
- [ ] 搜索 metrics double compression 或 login metric 未接线描述。
- [ ] 搜索允许 userinfo/query/non-HTTP mirror 或完整 URL 输出的描述。
- [ ] 搜索 ASCII-only sanitizer 描述。
- [ ] 对每个残留命中判断是历史语境还是当前合同。

## 6. Evidence audit

- [ ] 不修改旧 archive checklist/commit。
- [ ] 检查五个前置 child 的 implement checklist、validation summary 和 task commit。
- [ ] 本任务按实际执行增量勾选并记录每条验证结果。
- [ ] 为父任务记录 child → commit map。
- [ ] archive 前确认 task.json.commit 只能填写已存在的 commit。

## 7. Full validation

~~~powershell
$env:GOTOOLCHAIN='go1.25.12'
gofmt -l .
go mod tidy -diff
go mod verify
go vet ./...
go test -race ./...
go build ./...
go run golang.org/x/vuln/cmd/govulncheck@v1.5.0 ./...

cd frontend
pnpm install --frozen-lockfile
pnpm lint
pnpm test
pnpm build
pnpm audit:prod
pnpm audit:dev
cd ..

bash -n scripts/*.sh
bash scripts/install_test.sh
shellcheck scripts/*.sh
actionlint
bash scripts/extract-changelog.sh Unreleased
git diff --check
git status --short
~~~

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
