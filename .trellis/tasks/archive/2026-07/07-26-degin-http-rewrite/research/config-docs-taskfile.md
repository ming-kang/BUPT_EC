# Research: GIN_MODE 清除面、构建命令统一面、Taskfile 落地

- **Query**: GIN_MODE 全仓出现点与连带改动；构建/测试/检查命令全仓清单；go-task/task 调研；Taskfile.yml 草案；CHANGELOG 格式
- **Scope**: mixed（内部代码/文档/CI 检索 + Taskfile 外部事实）
- **Date**: 2026-07-27
- **关联需求**: PRD R3（删 GIN_MODE）、R7（Taskfile + 文档统一）、AC 最后一条（CHANGELOG 记录 GIN_MODE 移除与 LogID→X-Log-Id）

---

## 1. GIN_MODE 全仓出现点与连带改动

### 1.1 Go 代码（删除即编译错，必须一次改完）

| 位置 | 内容 | 连带改动 |
|---|---|---|
| `config/config.go:18` | `GinModeKey = "GIN_MODE"` 常量 | 删除 |
| `config/config.go:22` | `DefaultGinMode = "debug"` 常量 | 删除 |
| `config/config.go:41` | `RuntimeConfig.GinMode string` 字段 | 删除 |
| `config/config.go:71` | `GinMode: resolve(GinModeKey)` | 删除 |
| `config/config.go:81-83` | 空值回填默认 `debug` | 删除 |
| `config/config.go:98-100` | `validate()` 中三值校验，错误文案 `"GIN_MODE must be debug, release, or test"` | 删除整个 if 块 |
| `main.go:40` | `gin.SetMode(runtimeConfig.GinMode)` | 删除；同时移除 `gin` import（本任务去 Gin 的一部分） |
| `handler_test.go:22` | `gin.SetMode(gin.TestMode)` | 随 handler_test 去 Gin 重写一并消失 |

注意：`config.Load` 只 `resolve` 已知 key（config.go:57-77），删除后旧部署 env 文件里残留的 `GIN_MODE=release` 行会被**静默忽略，不报错**——升级兼容天然安全，无需迁移逻辑，但要在 CHANGELOG 里向用户说明"该变量现在被忽略"。

### 1.2 config 测试

| 位置 | 内容 | 连带改动 |
|---|---|---|
| `config/config_test.go:24` | want 中 `GinMode: DefaultGinMode` | 删字段 |
| `config/config_test.go:35` | dotenv fixture 行 `"GIN_MODE=release"` | 删行（precedence 用例仍可用 `APP_ADDR`/`LOG_CALLER` 证明覆盖语义，用例本身保留） |
| `config/config_test.go:41,51,57,63` | fixture/want 中 GinMode 相关 | 同上 |
| `config/config_test.go:197-234` | `TestLoadGinModeAndLogCaller` 整个函数（表驱动，含 `invalid mode` 期望报错分支 :210,:222） | GinMode 维度删除；LogCaller 维度需保留 → 建议改名 `TestLoadLogCaller` 并裁掉 mode 列，而非整函数删除 |

### 1.3 .env.example 与文档

| 位置 | 内容 | 连带改动 |
|---|---|---|
| `.env.example:7-10` | 3 行注释 + `GIN_MODE=debug` | 删 4 行 |
| `docs/development.md:23-24` | 配置示例中 `# Gin runtime mode...` + `GIN_MODE=debug` | 删 2 行 |
| `docs/development.md:29` | "Startup validates credentials, `GIN_MODE`, and `APP_ADDR`, then applies Gin/log settings" | 改写为只提 credentials + APP_ADDR + log settings |
| `docs/deployment.md:85` | 安装器交互提示清单里 "Gin mode (default `release`)" | 删行 |
| `docs/deployment.md:164` | 手动部署 env 示例 `GIN_MODE=release` | 删行 |

### 1.4 install.sh（位置参数重排，牵一发动全身）

| 位置 | 内容 | 连带改动 |
|---|---|---|
| `scripts/install.sh:17` | `DEFAULT_GIN_MODE="release"` | 删除 |
| `scripts/install.sh:29` | `CURRENT_GIN_MODE=""` 声明 | 删除 |
| `scripts/install.sh:99` | `CURRENT_GIN_MODE="${GIN_MODE:-}"`（回读现有 env） | 删除 |
| `scripts/install.sh:225-231` | `validate_gin_mode()` 函数 | 删除整个函数 |
| `scripts/install.sh:629` | `render_env_file` 的 `local gin_mode="${11}"` | 删参数 → **后续 `${12}` download_base_url 前移为 `${11}`** |
| `scripts/install.sh:642` | 渲染 `GIN_MODE=$(shell_quote ...)` 到 `/etc/bupt-ec/bupt-ec.env` | 删行 |
| `scripts/install.sh:775,781` | `stage_install` 的 `local gin_mode="${13}"` 与透传 | 删参数并重排后续位置参数 |
| `scripts/install.sh:1128` | `local ... gin_mode ...` 变量声明 | 删除 |
| `scripts/install.sh:1181` | `gin_mode="$(prompt_required "Gin mode" ...)"` 交互提示 | 删除 |
| `scripts/install.sh:1190` | `validate_gin_mode "${gin_mode}"` 调用 | 删除 |
| `scripts/install.sh:1218` | 调用 stage_install 时透传 `"${gin_mode}"` | 删实参 |
| `scripts/install_test.sh:396-399` | `render_env_file ... "127.0.0.1:8080" "release" ""` —— 倒数第二个实参就是 gin_mode | **同步删实参**，否则位置错位、download_base_url 收到 `"release"` |

陷阱：render_env_file / stage_install 全靠位置参数（`${1}`..`${13}`），删中间一个参数必须把两个函数签名、两处调用点、install_test.sh 的 fixture 一次改齐；CI 的 `bash scripts/install_test.sh` 和 `shellcheck scripts/*.sh`（quality.yml:99-103）会兜底。

### 1.5 "Gin" 字样残留（非 GIN_MODE 配置，去 Gin 后需同步改写的文档措辞）

| 位置 | 内容 |
|---|---|
| `README.md:16` | 架构图 "Go / Gin backend" |
| `docs/development.md:78` | "main.go, router.go, handler.go — Gin entry points" |
| `docs/development.md:113` | "applies Gin/log settings" |
| `docs/operations.md:40` | "the Gin gzip middleware is the only compressor"（换 gzhttp 后措辞要改） |
| `AGENTS.md:5` | 无 Gin 字样（已核对），但架构描述提 router.go/handler.go，若 R8 移动到 internal/httpapi 需同步 |
| CHANGELOG 历史条目（如 `CHANGELOG.md:193,231`） | 历史记录，**不改** |

### 1.6 .trellis/spec（主 agent 走 update-spec，不在本任务直接编辑）

- `.trellis/spec/backend/directory-structure.md:84`（配置键清单含 GIN_MODE）、`:109`（校验错误表）、`:114`（systemd 示例 `GIN_MODE=release`）。

### 1.7 CI

CI workflows（ci.yml / quality.yml / release.yml）中**无** GIN_MODE 出现点，无需改。

---

## 2. 构建/测试/检查命令全仓出现点（Taskfile 统一后的文档同步清单）

### 2.1 README / AGENTS

| 位置 | 命令原文 |
|---|---|
| `README.md:58` | `cd frontend && pnpm install && pnpm build && cd ..` + `go run ./`（:59） |
| `AGENTS.md:14-15` | 同上两行 |
| `AGENTS.md:18` | `go build -o bupt-ec -v ./`、`go test ./...`、`go test -race ./...`、`gofmt -w`、`go vet ./...` |
| `AGENTS.md:20` | `pnpm dev`、`pnpm build`、`pnpm lint`、`pnpm test`（frontend/ 下） |

### 2.2 docs/

| 位置 | 命令原文 |
|---|---|
| `docs/development.md:36-41` | `cd frontend` / `pnpm install` / `pnpm build` / `cd ..` / `go run ./` |
| `docs/development.md:47-48` | `go run ./` + `cd frontend && pnpm dev` |
| `docs/development.md:54-61` | `go test ./...`、`go test -race ./...`、`go vet ./...`、`gofmt -l .`、`GOTOOLCHAIN=go1.25.12 go mod tidy -diff`、`go mod verify`、`cd frontend && pnpm lint && pnpm test && pnpm build`、`cd frontend && pnpm audit:prod && pnpm audit:dev` |
| `docs/development.md:67` | `go test -tags integration ./service` |
| `docs/development.md:33` | "`//go:embed` in `router.go`" 描述（R6 embed 下沉后要改成新包名 + `-tags embed_assets` 说明） |
| `docs/release.md:70` | 质量门命令的叙述性清单（audits/lint/test/build、go mod tidy -diff、gofmt、vet、-race、govulncheck、shellcheck） |
| `docs/release.md:99` | pnpm 9.15.x（corepack） |

### 2.3 CI workflows

| 位置 | 命令原文 |
|---|---|
| `.github/workflows/quality.yml:31` | `pnpm install --frozen-lockfile` |
| `quality.yml:35,39,43,47,51` | `pnpm audit:prod` / `pnpm audit:dev` / `pnpm lint` / `pnpm test` / `pnpm build` |
| `quality.yml:67-71` | `go mod tidy -diff`（失败提示 `GOTOOLCHAIN=go1.25.12 go mod tidy`） |
| `quality.yml:74` | `go mod verify` |
| `quality.yml:78-83` | `gofmt -l .` 非空即失败 |
| `quality.yml:86,89,92` | `go vet ./...`、`go test -race ./...`、`go build ./...` |
| `quality.yml:97` | `go run golang.org/x/vuln/cmd/govulncheck@v1.5.0 ./...` |
| `quality.yml:100,103` | `bash scripts/install_test.sh`、`shellcheck scripts/*.sh` |
| `.github/workflows/release.yml:62-63` | `go build -trimpath -ldflags "-s -w -X main.version=${version}" -o build/... -v ./`（版本 = tag 或 `nightly-<short sha>`，:56-60） |

R6 落地后 **quality.yml:92 `go build ./...` 和 release.yml:62 都必须加 `-tags embed_assets`**（release 构建靠下载的 frontend-dist artifact，release.yml:45-49），而 quality.yml 的 `go vet`/`go test` 保持无 tag——这正是验收"干净检出全绿"的 CI 化。

### 2.4 scripts/

`scripts/install.sh`、`scripts/release.sh` 中**没有** go/pnpm 构建命令（installer 只下载 release 产物），Taskfile 不需覆盖。

### 2.5 .trellis/spec（update-spec 渠道处理）

- `.trellis/spec/backend/quality-guidelines.md:93-122`（两组命令清单）、`:144-147`（ldflags 版本注入）、`:232`
- `.trellis/spec/backend/index.md:38-39`
- `.trellis/spec/backend/api-contract.md:224-232`：**重要备忘**——`go test -ldflags "-X main.version=..."` 是静默 no-op（测试二进制 main 包是合成的），所以 Taskfile 的 `test` 任务不要带版本注入。

---

## 3. Taskfile（go-task/task）落地调研

> 证据说明：本会话无 WebSearch 工具。版本号来自本机 `winget search Task.Task` 实时查询（2026-07-27）；其余为知识截止 2026-01 的 Taskfile 官方文档事实，均为长期稳定结论，实施时如引入 CI action 需再核对最新 SHA。

### 3.1 版本与 schema

- 当前稳定版：**v3.52.0**（winget 源 `Task.Task`，2026-07-27 查询）。本机未安装 task。
- Schema：`version: '3'`，自 2020 年起唯一现行大版本（v2 早已废弃），近年新特性（`vars` 求值增强、`requires`、通配任务等）都在 v3 内向后兼容。写 `version: '3'` 即可，不需要锁小版本。

### 3.2 Windows 行为（本仓库开发机是 Windows，这是选 Taskfile 的核心理由）

- Task 内置 [mvdan/sh](https://github.com/mvdan/sh) 解释器执行每条 cmd，**不依赖系统 sh/bash/cmd**。`$(...)`、`test -z`、`|`、`&&`、多行脚本在 Windows 上原生可用。
- 注意点：cmd 内不能用 cmd.exe 内建命令（如 `dir`）；路径写正斜杠；被调用的程序（go、pnpm、git）走 PATH 查找，与平台无关。
- 模板函数 `{{exeExt}}` 可用于二进制名（Windows 下自动 `.exe`）。

### 3.3 安装方式

| 渠道 | 命令 |
|---|---|
| winget（本机可用，已验证有包） | `winget install Task.Task` |
| scoop | `scoop install task`（main bucket） |
| go install | `go install github.com/go-task/task/v3/cmd/task@latest` |
| npm（前端同学顺手） | `pnpm add -g @go-task/cli` |

### 3.4 CI 用不用 task：建议与理由

官方文档推荐的 GitHub Actions 装法是 `arduino/setup-task`（传 `version: 3.x` 与 `repo-token` 防 API 限流）；也可用 `go install`（复用 setup-go 缓存，但每次冷缓存要编译）。

**建议：CI（quality.yml/release.yml）保留原生命令，Taskfile 只服务本地开发；文档命令统一写 `task xxx`，quality.yml 顶部加一行注释声明"与 Taskfile.yml 的 check/test/build 保持同步"。** 理由：

1. quality.yml 现在是 15 个细粒度 step（quality.yml:29-103），每步独立命名、独立日志分组、失败定位精确；折叠成 `task check` 会失去 GitHub UI 的分步可见性和 `::error::` 注解（quality.yml:68）。
2. 本仓库 CI 惯例是所有 action 按 commit SHA 固定（quality.yml:15,23,54,61）；引入 arduino/setup-task 是新的供应链依赖 + 一次网络下载，收益只是"少一处命令重复"。
3. CI 与 Taskfile 的漂移风险靠 PRD 验收条目（"README/AGENTS.md/docs 构建命令与 Taskfile 一致"）+ 注释锚点控制即可；命令本身（gofmt/vet/test -race/build）多年未变。
4. **与 PRD R7 的措辞有出入**：R7 写"quality.yml 与 README/AGENTS.md/docs 的构建命令统一指向 task"。若 design.md 坚持 CI 也走 task，则方案是：pin SHA 的 `arduino/setup-task` + `version: 3.52.0`，并把 quality.yml 的 Go 检查段折叠为 `task check:go`、`task test`、`task build:ci` 三步——这是需要 owner 在 design.md 拍板的决策点，两个方案都可行。

### 3.5 lint 工具现状核对

- **无 `.golangci.yml`**（Glob 全仓无 `.golangci*`），CI 也不跑 golangci-lint → `check` 任务不引入 golangci-lint，保持 gofmt + vet + tidy-diff + verify 与 CI 等价。
- govulncheck 在 CI 固定 `@v1.5.0`（quality.yml:97），需要联网拉漏洞库，**不放进日常 `check`**，可给可选任务 `vuln`。
- 前端 check 面：`pnpm lint` / `pnpm test` / `pnpm audit:prod` / `pnpm audit:dev`（frontend/package.json:10-14，pnpm@9.15.0 via packageManager 字段）。

---

## 4. Taskfile.yml 草案

```yaml
# Taskfile.yml — 本地开发编排（Windows/Linux/macOS 通用，Task v3.x）
# CI (.github/workflows/quality.yml) 保留原生命令；改这里时同步改 CI 与 docs/development.md。
version: '3'

vars:
  BIN: bupt-ec{{exeExt}}
  VERSION:
    sh: git describe --tags --always --dirty 2>/dev/null || echo dev

tasks:
  frontend:install:
    dir: frontend
    cmds:
      - pnpm install --frozen-lockfile

  frontend:build:
    desc: Build frontend into frontend/dist (embedded by -tags embed_assets)
    dir: frontend
    cmds:
      - pnpm build
    sources:
      - src/**/*
      - index.html
      - package.json
      - vite.config.js
      - pnpm-lock.yaml
    generates:
      - dist/**/*

  build:
    desc: Full release-equivalent binary with embedded frontend
    deps: [frontend:build]
    cmds:
      - go build -trimpath -tags embed_assets -ldflags "-s -w -X main.version={{.VERSION}}" -o {{.BIN}} ./

  test:
    desc: Backend tests with race detector (no embed tag; works on clean checkout)
    cmds:
      - go test -race ./...

  check:
    desc: Local quality gate mirroring quality.yml (Go + frontend)
    cmds:
      - |
        unformatted="$(gofmt -l .)"
        if [ -n "$unformatted" ]; then echo "Files need gofmt:"; echo "$unformatted"; exit 1; fi
      - go vet ./...
      - go mod tidy -diff
      - go mod verify
      - task: frontend:install
      - pnpm -C frontend lint
      - pnpm -C frontend test
      - pnpm -C frontend audit:prod
      - pnpm -C frontend audit:dev

  vuln:
    desc: govulncheck (network; same pinned version as CI)
    cmds:
      - go run golang.org/x/vuln/cmd/govulncheck@v1.5.0 ./...
```

草案依据与注意：

- `build` 的 ldflags 逐字对齐 release.yml:62（`-trimpath -ldflags "-s -w -X main.version=..."`），加上 R6 的 `-tags embed_assets`；`VERSION` 动态 var 用 `sh:` 求值，Windows 下经内置解释器执行没问题（git 在 PATH）。
- `test` **故意不带** embed tag 和 ldflags：干净检出可跑（R6 验收），且 `go test -ldflags -X main.version` 本来就是 no-op（.trellis/spec/backend/api-contract.md:231-232）。
- `check` 的 gofmt 块复刻 quality.yml:78-83 的"列出未格式化文件再失败"，sh 语法由内置解释器保证跨平台。
- `frontend:build` 配 `sources/generates` 做指纹跳过，重复 `task build` 不重跑 vite。
- `GOTOOLCHAIN=go1.25.12` 前缀（docs/development.md:58）没写进 check：go.mod toolchain 指令已约束版本，本地一般无需显式；若要严格可在 `env:` 里加。

---

## 5. CHANGELOG.md 现有格式与条目草案

格式（CHANGELOG.md:1-21）：

- Keep a Changelog 1.1.0 + SemVer（:5）；用户可见变更写进 `## [Unreleased]`（:7），`scripts/release.sh` 切版本，release 工作流取该节做 release notes。
- 当前 Unreleased 已有 `### Added`（/readyz version 字段，:11-14）和 `### Changed`（release 二进制版本注入，:16-21）。
- 风格：英文、~80 列折行、面向运维/用户的行为描述（不写内部实现名）；历史上用过 `### Removed`？未出现过，但 Keep a Changelog 标准类别包含 Removed，直接新增该小节即可。
- 先例参考：0.1.5 的 `LogID` header 提及在 :142-143（"return correlated JSON 404 with `LogID` header"）；gin 依赖升级记录在 0.1.4 Dependencies（:193）。

本任务需追加的 Unreleased 条目草案：

```markdown
### Changed

- The request correlation id response header is now `X-Log-Id` (previously
  `LogID`). Body `log_id` fields are unchanged.

### Removed

- The `GIN_MODE` environment variable: the HTTP layer no longer uses Gin, so
  the setting has no effect and is now ignored. Existing `GIN_MODE=` lines in
  `/etc/bupt-ec/bupt-ec.env` are harmless; the installer no longer prompts for
  a Gin mode.
```

（措辞供 design/implement 直接取用，最终以实现为准。）

## Caveats / Not Found

- 无法联网 WebSearch/deepwiki（工具不可用）；Task 版本号 3.52.0 来自本机 winget 实时查询，`arduino/setup-task` 用法为知识截止内的官方文档事实，若 design 决定 CI 装 task，实施前需核对该 action 最新版本与 SHA。
- CI workflows 中确认无 GIN_MODE；install.sh/release.sh 中确认无 go/pnpm 构建命令。
- `.golangci.yml` 确认不存在，check 任务不含 golangci-lint。
