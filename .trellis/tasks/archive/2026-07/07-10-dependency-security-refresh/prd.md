# 依赖与工具链安全刷新

## Goal

修复 2026-07-10 复查中由 `govulncheck` 与 pnpm audit 确认的依赖和
构建工具链漏洞，使 PR、main 推送和 release 使用明确的安全 Go 补丁版本，
并让前端生产依赖与开发工具链满足可执行的审计阈值，而不改变应用功能、
公开 API、UI 设计或发布资产布局。

## Background

- `go run golang.org/x/vuln/cmd/govulncheck@v1.5.0 ./...` 当前报告：
  - `GO-2026-5676` 可由 `github.com/quic-go/quic-go v0.59.0` 触达，修复版
    为 `v0.59.1`；该间接依赖来自 Gin 的 HTTP/3 依赖链。
  - 本机 Go `1.26.4` 命中标准库 TLS 漏洞 `GO-2026-5856`；安全版本为
    Go `1.25.12` / `1.26.5`。
- `go.mod` 仅声明 `go 1.25.0`，CI/release 三处使用宽泛的
  `go-version: "1.25"`，不能表达本次要求的安全补丁下限。
- `pnpm audit --prod` 报告 `@babel/runtime 7.23.7` 的一个 moderate 漏洞。
- 完整 pnpm audit 报告 11 high、20 moderate、4 low，主要来自 Vite
  `5.0.10`、Rollup `4.9.1`、esbuild `0.19.11` 和 ESLint 8 的旧传递依赖。
- 项目发布工具链为 Go 1.25、Node 22、pnpm 9.15.x；本任务生成锁文件时
  必须使用 pnpm `9.15.0`，而不是本机全局 pnpm 10。

## Requirements

### R1 — Go 安全下限

- 将模块最低 Go 补丁版本和所有 CI/release Go setup 统一为 `1.25.12`。
- 将 `github.com/quic-go/quic-go` 更新到至少 `v0.59.1`，不顺带升级 Gin
  或无关 Go 依赖。
- 使用安全 Go 工具链运行 `govulncheck`，结果不得存在可达漏洞。
- 文档必须说明 Go `1.25.12+`；若使用 Go 1.26，则至少为 `1.26.5`。

### R2 — 前端生产依赖安全

- 消除 `pnpm audit --prod --audit-level moderate` 当前发现的
  `@babel/runtime` 漏洞。
- 优先使用与 Babel 7 调用方兼容的精确 pnpm override，不通过大范围
  Ant Design/React 升级来修复单个传递依赖。
- 不升级 React、React DOM、Ant Design 的主版本，不修改页面行为或样式。

### R3 — 前端开发工具链安全

- 采用 Vite `6.4.3` 安全线；不直接跨到 Vite 8。
- 将 `@vitejs/plugin-react` 更新到兼容 Vite 6 的 4.x 安全版本。
- 将 ESLint 8 更新到 ESLint 9，并将 `.eslintrc.cjs` 等价迁移为 flat
  config；保留现有 React、Hooks 和 react-refresh 规则意图。
- 更新相关 ESLint 插件到支持 ESLint 9 的版本，不把 lint 规则扩张为
  无关的应用重构。
- `pnpm audit --audit-level high` 必须通过；若仍有低/moderate 开发依赖
  公告，必须在任务记录中说明不可达性或另行修复，不能保留 high/critical。

### R4 — 可执行审计策略

- 在 frontend scripts 中提供稳定的生产审计和全工具链审计命令：
  - 生产依赖：moderate 及以上失败；
  - 完整依赖：high 及以上失败。
- PR CI 与 release quality gate 都必须在 frozen-lockfile 安装后执行同一组
  审计脚本。
- 本任务不顺带重构两个 workflow 的重复步骤；复用清理留给独立子任务。

### R5 — 文档与变更记录

- 更新 `AGENTS.md`、`docs/development.md` 及其他实际提到工具链版本的文档。
- 在 `CHANGELOG.md` `[Unreleased]` 的 `Security` / `Dependencies` 下记录
  Go、前端和审计门禁变化。
- `.agents/`、`.codex/` 保持未提交；`.trellis/.template-hashes.json` 继续忽略。

## Acceptance Criteria

- [x] `go.mod`、CI 和 release 均要求 Go `1.25.12`，文档同步。
- [x] `github.com/quic-go/quic-go` 解析为 `v0.59.1` 或更高安全补丁，且
      没有无关 Go 依赖漂移。
- [x] 使用 Go `1.25.12` 执行 `govulncheck ./...` 无可达漏洞。
- [x] 锁文件由 pnpm `9.15.0` 生成，frozen install 可复现。
- [x] Vite 解析为 `6.4.3`，ESLint 解析为 9.x，旧 `.eslintrc.cjs` 被等价
      flat config 替代。
- [x] 生产依赖 audit 在 moderate 阈值通过，完整 audit 在 high 阈值通过。
- [x] 前端 lint、44 个现有 Vitest 行为测试和 production build 全部通过。
- [x] `go test -race ./...`、`go vet ./...`、`gofmt -l .`、完整 Go build
      和 `git diff --check` 通过。
- [x] CI/release 审计步骤、开发文档和 changelog 与实际命令一致。
- [x] 未修改公开 API、业务行为、release 资产名称或安装器逻辑。

## Out of Scope

- React 19、Ant Design 6、Vite 8 或其他大范围前端框架升级。
- Gin 主版本升级、Go 模块重命名或业务代码重构。
- CI/release reusable workflow 去重。
- 安装器信任链、回滚和 Nginx 超时修复；这些属于下一个 P1 子任务。
- 自动依赖机器人；仓库继续采用人工依赖更新策略。
