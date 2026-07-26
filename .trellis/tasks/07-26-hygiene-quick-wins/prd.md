# 工程卫生速赢：依赖清理、CI 优化、版本注入

父任务：`07-26-arch-simplify-refactor`（批次①）。轻量任务，PRD-only。
证据来源：父任务 `research/audit-engineering.md`、`research/audit-frontend.md`。

## Goal

零业务逻辑改动的一批卫生修复：删无用依赖、修 .gitignore、CI 提速与加固、发布二进制版本注入、修文档矛盾。全部改动互不冲突、可独立回滚。

## Requirements

依赖与仓库卫生：
- R1 删除 `frontend/package.json` 中未使用的直接依赖 `dayjs`（源码 0 次引用，antd 会自带）。
- R2 `.gitignore`：新增 `/bupt-ec`、`/build/`、`/dist/`；删除死规则 `config/*.json`。
- R3 `git mv accepts_gzip_test.go router_test.go`（测的是 router.go:106,201 的 acceptsGzip/gzipMiddleware）。

CI（.github/workflows/）：
- R4 所有 job 加 `timeout-minutes`（quality 15、release 各 job 10-15）。
- R5 `setup-node` 启用 pnpm 缓存（pnpm 安装提前到 setup-node 之前或改 pnpm/action-setup，钉 SHA 与现有风格一致）。
- R6 删除 `quality.yml:91` 的 apt 安装 shellcheck（runner 已预装）。
- R7 govulncheck 二进制缓存（actions/cache，key 含版本号）。
- R8 quality.yml 构建后上传 `frontend/dist` artifact；release.yml 删除 `build-frontend` job，`build-go` 直接 `needs: quality-gate` 并下载该 artifact。

版本注入：
- R9 `main.go` 加 `var version = "dev"`；启动日志与 `/readyz` 响应带版本字段；release.yml 构建改为 `go build -trimpath -ldflags "-s -w -X main.version=${GITHUB_REF_NAME}"`。

文档与配置矛盾：
- R10 `docs/release.md:8-11` 删除表格中间的空行（GitHub 渲染断裂）；两处补上 `quality.yml (reusable gate)` 的提及。
- R11 `.env.example` 的 `GIN_MODE=release` 改为 `debug` 并加注释（与 config.go 默认值、development.md 示例一致）。
- R12 CHANGELOG.md 在 Unreleased 下记录用户可见变更（/readyz 增加版本字段）。

## Out of Scope

- `.cursor/`/`.pi/` 去重（用户决定都保留）。
- golangci-lint 引入（并入子任务 2 或后续，避免与本批 CI 改动互相干扰）。
- Taskfile（属于子任务 3，与 embed 解耦联动）。

## Acceptance Criteria

- [ ] `pnpm install && pnpm build && pnpm test && pnpm lint` 全绿，且 `pnpm ls dayjs` 显示仅为传递依赖。
- [ ] `git check-ignore bupt-ec build/x dist/x` 均命中。
- [ ] 根目录不再有 `accepts_gzip_test.go`，`go test ./` 全绿。
- [ ] 三个 workflow YAML 通过 actionlint 或至少 YAML 解析；每个 job 均有 timeout-minutes；release.yml 不再有 build-frontend job。
- [ ] 本地 `go build -ldflags "-X main.version=test"` 后 `/readyz` 响应含 `"version":"test"`；未注入时为 `"dev"`。
- [ ] `docs/release.md` 表格在 Markdown 渲染中为完整一张表。
- [ ] `go test -race ./...` 全绿（回归确认）。

## 注意事项

- CI 改动无法在本地完全验证，PR 触发后需观察 Actions 实际运行（可在 PR 描述注明）。
- `/readyz` 加字段是增量、不破坏契约；handler_test.go / metrics_endpoint_test.go 若断言完整 JSON 需同步更新。
