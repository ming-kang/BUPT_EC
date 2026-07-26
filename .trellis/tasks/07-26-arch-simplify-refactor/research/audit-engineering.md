# 工程化链路审计结论（2026-07-26）

来源：scripts/、.github/、依赖、配置、仓库杂项的审计。总评：质量意识很高（action 钉 SHA、安装器事务回滚、文档与代码几乎零漂移），问题是手写了大量现成工具能做的事 + 构建顺序耦合只存在于文档。

## 最高优先级

### go:embed 导致裸克隆全挂
- router.go:17 `//go:embed frontend/dist`，dist 不入库。实测干净检出 `go vet ./...` 直接失败（pattern frontend/dist: no matching files found），go test/build/gopls 全挂。
- 修复约束只写在 README/AGENTS.md/docs/development.md 三处散文里。
- 推荐方案 B：embed 下沉独立包 web/embed.go，构建标签双实现（`//go:build embed_assets` 才真嵌入，否则空 FS + "frontend not built" 提示页），release 用 `-tags embed_assets`。

### 无构建编排工具
- 构建 DAG（pnpm build → go build）复制在 README/AGENTS.md/development.md/quality.yml/release.yml 五处。
- 建议 ~30 行 Taskfile（Windows 开发更友好）：frontend:build / build / test / check，CI 与文档统一调 task check。

### agent 目录占入库文件 76%
- 445 个入库文件中 337 个属于 .trellis/(234)、.cursor/(52)、.pi/(51)。
- 实测 `diff -rq .cursor/skills .pi/skills` 逐字节相同。
- 建议：.cursor/.pi 二选一入 gitignore；.trellis/scripts、agents/ 上游分发物移出版本控制（靠 .version 还原），保留 config/spec/tasks。
- 注意：本重构任务本身依赖 Trellis 运行，此项动作需谨慎排序或单独任务。

## CI/CD（.github/）
- quality.yml 已完整 pnpm build 但没上传产物，release.yml 的 build-frontend job 串行重复构建 → quality 上传 artifact，删 build-frontend job。
- 零缓存：setup-node 未开 pnpm 缓存；govulncheck 每次源码编译（quality.yml:85）；apt 安装已预装的 shellcheck（:91）→ 每次 CI 省 1.5-3 分钟。
- 所有 job 无 timeout-minutes（默认 360 分钟）→ 加 15/10。
- Go 静态检查只有 gofmt+go vet → 加 golangci-lint（errcheck/ineffassign/unconvert/misspell/gosec）。
- release 构建无 -trimpath/-s -w/版本注入（release.yml:79-85）；已做 provenance attestation 但不可复现 → `-trimpath -ldflags "-s -w -X main.version=${GITHUB_REF_NAME}"`，main.go 加 `var version = "dev"`，/readyz 与启动日志带版本。
- nightly 删除-重建有 404 空窗（release.yml:171-190）→ 原地覆盖 assets。

## scripts/
- install.sh（1230 行 48 函数）质量高但 5 职责混杂：
  - 短期：render_systemd_service(:650)/render_nginx_site(:693) 抽成 scripts/templates/*.tmpl + envsubst（−100 行）。
  - 中期：releases/<version>/ + current symlink 替代 snapshot/rollback 机制（−200 行 + 400 行测试）。
  - 可选：8 行 Dockerfile（distroless/static）+ compose.yml（Caddy 可免 SSL 交互）。
  - 保留：镜像 URL 校验/脱敏、checksum fail-closed、TTY 交互。
- install_test.sh（1092 行）自建断言/mock 框架 → bats-core（bats-assert/bats-mock），~600 行 + TAP + 并行。POSIX_MODES_SUPPORTED 分支使 Windows 下权限断言静默跳过。
- release.sh:65-67 GNU-sed-only 替换在 macOS 静默产坏结果；:69 的 version 正则脆弱。中期 goreleaser 一次替代 release.yml 两个 job + extract-changelog.sh + release.sh 打包逻辑（~50 行 .goreleaser.yml）。

## 配置
- .env.example 5 个变量 vs 安装器写入生产 env 11 个（6 个安装器私有状态混入应用 env，经 EnvironmentFile 注入 Go 进程）→ 拆 bupt-ec.env 与 installer.state。
- GIN_MODE 默认值三处矛盾：config.go:22=debug、.env.example:8=release、development.md 示例=debug → .env.example 改 debug + 注释。
- config.Campuses 硬编码在 Load 内（config/config.go:73-76）是领域常量 → 移 service。
- main 无 flag：--version/--env-file/--log-dir；.env 与 run_log 路径依赖 WorkingDirectory 巧合。

## 仓库杂项
- .gitignore：`config/*.json` 死规则；漏 `/bupt-ec`、`/build/`（AGENTS.md 教的构建命令产物会被 git add . 误提交）。
- accepts_gzip_test.go 应叫 router_test.go（测的是 router.go:106,201）。
- go.mod 模块名 `BUPT_EC` 非 URL 形式，不可 go install（改 github.com/ming-kang/BUPT_EC 需全仓 import 前缀）。低优先。
- docs/release.md:8-11 表格被空行截断（GitHub 渲染坏）；"Two workflows" 应为三个（漏 quality.yml）。
- 静态资源无 Cache-Control（Vite hash 文件名可 immutable 长缓存，index.html no-cache）；NoRoute 每次重读 index.html。
- 前端预压缩方案：vite-plugin-compression 生成 .gz/.br，Go 侧优先返回预压缩文件（与后端 gzip 改造联动）。

## 原审计第一批建议（低成本互不冲突）
删 dayjs；.gitignore 修复；CI timeout+缓存+删重复构建+删 apt shellcheck；ldflags 版本注入；删 .pi/ 或 .cursor/ 之一；docs/release.md 表格修复；GIN_MODE 统一。做完 CI 省 30-40%，不触业务逻辑。
