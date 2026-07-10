# 依赖与工具链安全刷新设计

## Boundaries

本任务只改变依赖解析、开发 lint 配置、CI 审计门禁和工具链文档：

```text
go.mod/go.sum ───────────────┐
                             ├─ safe Go 1.25.12 CI/release ─ govulncheck
GitHub workflow Go versions ┘

frontend/package.json ───────┐
frontend/eslint.config.js ───┼─ pnpm 9.15 lock ─ lint/test/build/audit
frontend/pnpm-lock.yaml ─────┘
```

应用运行时代码、API 数据流、缓存语义和 release asset layout 不变。

## Go Dependency Design

- `go` directive 从 `1.25.0` 提升为 `1.25.12`，表达模块最低安全补丁。
- `.github/workflows/ci.yml` 与 `release.yml` 三个 setup-go 步骤精确使用
  `1.25.12`，确保测试与发布二进制不依赖 runner 中任意旧 1.25 缓存。
- 仅将间接 `quic-go` 提升为 `v0.59.1`。若 `go get` 改动其他模块，逐项
  检查并回退非必要漂移。
- 本机 Go 1.26.4 本身不安全，因此最终漏洞验证显式使用
  `GOTOOLCHAIN=go1.25.12`。普通构建测试仍可运行，但不能用本机标准库
  的 govuln 结果判断仓库修复失败。

## Frontend Dependency Design

### Vite line

选择 Vite `6.4.3`：这是当前审计数据中首个覆盖全部已列 Vite 修复的稳定
安全线，同时仍被 Vitest 3.2.7 与 `@vitejs/plugin-react` 4.7.0 支持。
Vite 5 无法满足最新公告修复版本，Vite 8 则会引入 Rolldown/Node engine
和插件主版本迁移，超出最小安全修复范围。

### ESLint line

选择 ESLint `9.39.x` 与支持 ESLint 9 的插件版本。ESLint 8 的旧依赖链
包含已确认的 high 漏洞且已经结束主线演进。迁移为 `eslint.config.js`：

- 使用 `@eslint/js` recommended；
- 使用 `eslint-plugin-react` recommended + jsx-runtime 规则；
- 保留 Hooks 的 `rules-of-hooks` / `exhaustive-deps` 规则；
- 保留 `react-refresh/only-export-components` warn 和 constant-export 例外；
- 忽略 `dist`；
- 删除 flat config 不再支持的 `--ext` CLI 参数。

这应是配置格式迁移，而不是扩大 lint 规则范围。若新插件默认规则导致
无关代码重写，改为显式等价规则集合。

### Babel runtime

Ant Design 5 依赖的 Babel 7 runtime helper 具有同主版本向后兼容契约。
使用 pnpm override 固定到已修复的 Babel 7 最新补丁，避免为一个传递依赖
升级整个 UI 库。override 必须保持在 7.x，不能强制 Babel 8。

## Audit Policy

- `audit:prod`: `pnpm audit --prod --audit-level moderate`
- `audit:dev`: `pnpm audit --audit-level high`

生产浏览器依赖采用更严格阈值；开发依赖允许暂时存在不可达的低/moderate
公告，但 high/critical 会阻止 PR 和 release。两个 workflow 调用 package
scripts，避免在 YAML 中复制阈值字符串。

## Compatibility and Rollback

- 使用 pnpm 9.15.0 重写 lockfile，随后执行 frozen install 验证。
- Vite 配置和 React 源码预期无需变化；若 Vite 6 暴露构建不兼容，只做
  必要配置适配，不升级应用框架。
- ESLint 迁移失败时可回退 package/config/lockfile 整组改动；Go 安全补丁
  与前端工具链互不依赖，可分别验证和提交。
- Go 与前端依赖更新应形成两个独立 Conventional Commits；CI/docs/task
  收尾可在审计门禁提交中完成。
