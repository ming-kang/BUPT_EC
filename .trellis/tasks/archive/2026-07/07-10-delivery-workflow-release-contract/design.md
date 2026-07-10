# 交付工作流与发布说明合同设计

## Reusable Workflow Shape

新增 repository-local workflow，例如：

```text
.github/workflows/quality-gate.yml
  on: workflow_call
  permissions: contents: read
  jobs:
    quality: <all existing checks>

ci.yml
  on: pull_request
  jobs.quality -> uses quality-gate.yml

release.yml
  on: push main/tags
  jobs.quality-gate -> uses quality-gate.yml
  build/publish needs quality-gate
```

复用 workflow 自己 checkout 和 setup toolchains；caller 不传 secrets。job 名称应稳定，便于
branch protection 和 release needs 引用。

## Quality Contract

顺序保持：frontend frozen install → production/full audit → lint/test/build → Go setup →
gofmt/vet/race/build/govulncheck → installer tests → ShellCheck。

pnpm、Go、Node、govulncheck 和 Actions pins 只在复用文件维护一份。

## Release Notes Contract

### Stable

```yaml
with:
  body_path: release-notes.md
  # no generate_release_notes
```

`release-notes.md` 必须来自匹配 changelog section。

### Nightly

可以继续 `generate_release_notes: true`，因为 rolling prerelease 不承诺 changelog 区段原样。

## Compatibility and Rollback

- GitHub UI 中 job nesting/name 可能变化，但 required check 名称需在实施前核对。
- 若 reusable workflow 权限/needs 不兼容，可回滚 caller 重构而独立保留 stable notes 修复。
- 不修改 build artifacts 和 release upload steps。
