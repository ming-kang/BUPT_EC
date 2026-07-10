# 交付工作流与发布说明合同实施计划

## 1. Baseline

- [ ] 对比 ci/release quality jobs，记录任何非重复差异。
- [ ] 核对 branch protection 依赖的 check/job 名称。
- [ ] 固定 stable/nightly publish action 当前参数和 changelog extraction 测试。

## 2. Reusable quality workflow

- [ ] 新增 `workflow_call` workflow，移动完整门禁和 pins。
- [ ] CI caller 保留 pull_request trigger/permissions，只调用复用 workflow。
- [ ] Release caller 保留 push/concurrency，build jobs `needs` 复用门禁。
- [ ] 删除重复 steps 后全仓搜索旧版本/pin 残留。

## 3. Stable release notes

- [ ] 移除 stable action 的 `generate_release_notes`。
- [ ] 保留 nightly generated notes。
- [ ] 强化 matching changelog section 缺失/空内容测试（如现有脚本未覆盖）。

## 4. Docs/spec/changelog

- [ ] 更新 docs/release、development、AGENTS、quality spec 和 changelog。
- [ ] 明确 stable/nightly notes 差异和 reusable gate 维护入口。

## Validation

```bash
actionlint
pnpm --dir frontend install --frozen-lockfile
pnpm --dir frontend audit:prod
pnpm --dir frontend audit:dev
pnpm --dir frontend lint
pnpm --dir frontend test
pnpm --dir frontend build
gofmt -l .
go vet ./...
go test -race ./...
go build ./...
govulncheck ./...
bash scripts/install_test.sh
shellcheck scripts/*.sh
bash scripts/extract-changelog.sh --help  # or focused existing invocation
git diff --check
```

## Review Gates

- Reusable gate 不得遗漏任何现有检查。
- Caller 不得重复实现 gate。
- stable release 不得同时提供 body 和 generated notes。
- Action pins 与 permissions 必须显式。

## Rollback Points

- Reusable workflow/callers。
- Stable release note parameter。

两者可分提交，便于单独回滚。
