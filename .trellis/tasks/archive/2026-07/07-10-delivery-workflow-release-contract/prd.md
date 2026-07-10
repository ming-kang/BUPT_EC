# 交付工作流与发布说明合同

## Goal

复用 PR/main 质量门禁并确保稳定版发布说明严格来自对应 changelog 区段。

## Background

- `.github/workflows/ci.yml` 与 `release.yml:18-91` 复制同一套 Node/pnpm audit/lint/test/
  build、Go format/vet/race/build/govulncheck 和 installer/ShellCheck 步骤。
- 两份清单容易在版本、顺序或新增门禁时漂移，最近依赖审计就需要同步修改两处。
- `release.yml:228-239` 的稳定发布同时传入 `body_path: release-notes.md` 与
  `generate_release_notes: true`，GitHub 可能追加生成内容，违反“对应 changelog 区段
  原样作为稳定版说明”的文档合同。
- nightly 可继续使用生成说明；稳定 tag 与 rolling nightly 的语义不能混淆。

## Requirements

### R1 — 单一可复用质量门禁

- 提取 repository-local reusable workflow，通过 `workflow_call` 同时供 PR CI 和
  release quality gate 调用。
- 复用层包含当前全部检查、版本 pin、顺序和 working-directory 语义，不降低权限或门禁。
- PR 仍只在 pull_request 运行；main/tag release 仍由 release workflow 触发。
- release build/publish jobs 必须明确依赖复用门禁成功。

### R2 — 稳定版说明只来自 changelog

- stable tag 发布保留 `body_path` 并移除/禁用 `generate_release_notes`。
- release notes 继续由 `scripts/extract-changelog.sh` 从匹配版本区段生成，空/缺失区段
  必须 fail closed。
- nightly 可以保留 GitHub generated notes，但文档必须明确它不是稳定版合同。

### R3 — Pin、权限与资产保持

- 所有 GitHub Actions 继续 pin 到 40 字符 SHA，并保留可读版本注释。
- reusable workflow 使用最小 `contents: read` 权限，不隐式请求 secrets/write。
- release asset 名称、架构矩阵、checksum 和 installer 布局不变。

### R4 — 验证和文档

- actionlint 验证 caller/callee、needs、permissions、expressions 和 pins。
- 增加或复用 changelog extraction tests，验证 stable body 没有额外 generated notes 配置。
- 更新 release/development docs、AGENTS、quality spec 和 changelog。

## Acceptance Criteria

- [ ] PR 和 release 只引用同一个 reusable quality workflow。
- [ ] 两个入口执行完全相同的 frontend、Go、audit、installer 和 shell gates。
- [ ] stable release action 只使用 `body_path`，不存在 `generate_release_notes: true`。
- [ ] nightly 行为、push/tag triggers、concurrency 和 release assets 保持。
- [ ] 所有 action refs 仍为 40 字符 SHA，actionlint 通过。
- [ ] changelog extraction、frontend、Go 和 installer gates 通过。
- [ ] 文档准确区分 stable changelog notes 与 nightly generated notes。

## Out of Scope

- 引入第三方 CI 平台或 Dependabot/Renovate。
- 改变 stable/nightly 发布触发策略或资产格式。
- 优化构建缓存、并行度或 release 性能，除非复用所必需。
