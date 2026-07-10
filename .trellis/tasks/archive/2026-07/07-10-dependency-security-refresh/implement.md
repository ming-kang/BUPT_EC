# 依赖与工具链安全刷新实施计划

## 1. Baseline and search

- [x] 记录 Go/pnpm audit 基线和当前解析版本。
- [x] 全仓搜索 Go `1.25`、Node/pnpm、Vite/ESLint 和 audit 命令，列出所有
      需要同步的 workflow/docs/spec/changelog 位置。

## 2. Go security patch

- [x] 将 `go.mod` 最低版本改为 `1.25.12`。
- [x] 仅升级 `github.com/quic-go/quic-go` 到 `v0.59.1` 并检查 go.sum diff。
- [x] 将 CI/release 三个 setup-go 版本改为 `1.25.12`。
- [x] 使用 `GOTOOLCHAIN=go1.25.12` 运行 gofmt/vet/race/build/govulncheck。

## 3. Frontend toolchain patch

- [x] 用 pnpm 9.15.0 更新 package manifest：Vite 6.4.3、plugin-react 4.7.0、
      ESLint 9.39.x、兼容插件、Babel 7 runtime override、packageManager 和
      audit scripts。
- [x] 将 `.eslintrc.cjs` 替换为等价 `eslint.config.js`，调整 lint script。
- [x] 用 pnpm 9.15.0 生成 lockfile，并检查没有 React/AntD 主版本漂移。
- [x] 运行 frozen install、lint、test、build、production audit 和 full audit。

## 4. CI and documentation

- [x] 在 PR CI 与 release quality gate 的 frozen install 后执行
      `audit:prod` 和 `audit:dev`。
- [x] 更新 `AGENTS.md`、`docs/development.md` 及所有实际工具链版本说明。
- [x] 更新 `.trellis/spec/backend/quality-guidelines.md` 的安全门禁合同。
- [x] 在 `CHANGELOG.md` `[Unreleased]` 记录 Security/Dependencies 变化。

## 5. Full review gate

- [x] 检查 `git diff`，确认只有依赖、lint config、workflow、docs/spec、task
      和 changelog 变化。
- [x] 运行 `git diff --check`，确认 `.agents/`、`.codex/` 未纳入。
- [x] 运行 `trellis-check` 全量检查并记录任何环境限制。

## Validation Commands

```powershell
$env:GOTOOLCHAIN = "go1.25.12"
gofmt -l .
go vet ./...
go test -race ./...
go build -v ./
go run golang.org/x/vuln/cmd/govulncheck@v1.5.0 ./...

corepack pnpm@9.15.0 install --frozen-lockfile
corepack pnpm@9.15.0 lint
corepack pnpm@9.15.0 test
corepack pnpm@9.15.0 build
corepack pnpm@9.15.0 audit:prod
corepack pnpm@9.15.0 audit:dev

git diff --check
```

## Validation Results

- Go `1.25.12 windows/amd64`: `gofmt -l .`、`go mod tidy -diff`、
  `go mod verify`、`go vet ./...`、`go test -race ./...` 和完整构建通过。
- `govulncheck@v1.5.0 ./...`: 0 个可达漏洞；仍有 1 个 required-module
  公告，但项目未导入或调用其受影响包/符号。
- pnpm `9.15.0`: frozen install、ESLint 9 lint、44 个 Vitest 测试和 Vite
  6.4.3 production build 通过。
- `audit:prod` 与 `audit:dev`: 均报告 `No known vulnerabilities found`。
- `actionlint v1.7.12`、`git diff --check` 和全仓旧版本引用搜索通过；变更集
  不包含 Go/React 业务源码、安装器或 release asset layout 改动。
- 首轮 `go mod tidy -diff` 发现并移除了旧 quic-go `v0.59.0` 的两条过期
  `go.sum` 校验记录；修正后复验干净。

## Rollback Points

- Go patch: `go.mod`, `go.sum`, setup-go version lines.
- Frontend patch: `package.json`, `pnpm-lock.yaml`, ESLint config as one unit.
- Audit gate: package scripts and both workflow steps as one unit.
