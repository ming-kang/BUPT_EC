# HTTP 层重写：去 Gin、gzhttp、embed 解耦、Taskfile

父任务：`07-26-arch-simplify-refactor`（批次③后端）。复杂任务：start 前需 design.md + implement.md。
证据来源：父任务 `research/audit-backend.md`（去 Gin 一节、O4、O5、P2）、`research/audit-engineering.md`（go:embed、Taskfile、缓存头）。
依赖顺序：在子任务 2（backend-subtraction）之后执行。

**已确认决策**：gzip 换 `klauspost/compress/gzhttp`（不下放 Nginx，保持单二进制直连场景有压缩、install.sh 不动）。

## Goal

把 HTTP 层从 Gin 迁到 Go 1.22+ 标准库，同步解决 gzip 协议缺陷与 go:embed 裸克隆问题，并引入 Taskfile 统一构建编排。Go 模块数 77 → ~10。

## Requirements

去 Gin：
- R1 `gin` / `gin-contrib/static` 全部移除：路由改 `http.ServeMux`（`GET /api/get_data` 等方法+模式）；`c.JSON` → `writeJSON` helper；`gin.Recovery()` → 自写 recovery 中间件（注意与 gzip 的包装顺序：recovery 在最内层，避免 panic 时向已 Close 的 gzip writer 写入）。
- R2 `logs` 包解除 gin 依赖（log_util.go:15,58-62,98-108）：删除两个 gin 适配函数，log_id 改经 `r.WithContext` 传递；响应头改 `X-Log-Id`。
- R3 删除 `GIN_MODE` 配置项（config.go:19,81-83,98-100）及 .env.example / 文档 / install.sh 对应项；CHANGELOG 记录该配置移除（用户可见变更）。

gzip：
- R4 手写 gzipMiddleware + acceptsGzip（router.go:106-221，约 180 行）替换为 `klauspost/compress/gzhttp`。注意：Vary 全量、Content-Type 白名单、最小长度、Range/HEAD/空体语义均为 gzhttp **新增能力**（现状是无条件全量压缩），属行为修复而非等价替换，差异清单见 design.md §3。/healthz、/readyz 维持永不压缩（路径绕过），/metrics 维持外层单层压缩（promhttp DisableCompression 不变）。

静态服务与 embed：
- R5 静态文件改 `http.FileServerFS(fs.Sub(...))`；SPA fallback 的 index.html 启动时读取一次，带 ETag 与 `Cache-Control: no-cache`；带 hash 的 `/assets/*` 加 `Cache-Control: public, max-age=31536000, immutable`。
- R6 embed 解耦：嵌入下沉到独立包（如 `web/`），构建标签双实现——默认构建返回"frontend not built"提示页的空 FS，`-tags embed_assets` 才真正 `//go:embed dist`。目标：裸克隆 `go vet ./...`、`go test ./...` 可直接运行。release 构建加 `-tags embed_assets`。

构建编排：
- R7 新增 `Taskfile.yml`（Windows 开发友好）：`frontend:build` / `build`（依赖 frontend:build，带 embed_assets tag 与版本注入）/ `test` / `check`（gofmt、vet、lint、audit）。README/AGENTS.md/docs 的构建命令统一指向 task；**CI 保留原生命令**（分步日志与 SHA 钉扎惯例优先，靠 quality.yml 注释锚点防漂移——决策依据 design.md D6，修订自初稿"CI 也统一指向 task"）。
- R8 根包 HTTP 层文件（router.go/handler.go 及其测试）随本次重写评估移入 `internal/httpapi/`（若改动成本可控；否则保持平铺并在 design.md 说明）。

## Out of Scope

- /api/get_data 预序列化 + ETag/304（O6，批次④）。
- 冷启动 30s 阻塞改 503（O3，批次④）。
- vite 预压缩产物（.gz/.br）联动。

## Acceptance Criteria

- [ ] `go.mod` 直接依赖不含 gin、gin-contrib/static；`go list -m all | wc -l` 相比主分支显著下降（预期 ≤ 15）。
- [ ] 4 条路由 + SPA fallback + 404 行为与现状一致：`/api/get_data`（含 log_id 信封）、`/healthz`、`/readyz`、`/metrics`、静态资源、深链接回落 index.html。
- [ ] gzip 行为：Accept-Encoding 协商正确；所有可压缩响应带 `Vary: Accept-Encoding`；png/woff2 不二次压缩；现有 router_test / handler_test / metrics_endpoint_test 语义级断言通过（允许因中间件实现更换调整测试内部装配）。
- [ ] 干净检出（无 frontend/dist）下 `go vet ./...`、`go test ./...` 全绿；`task build` 产出可运行的完整二进制。
- [ ] `go test -race ./...` 全绿。
- [ ] README/AGENTS.md/docs/development.md 构建命令与 Taskfile 一致；CHANGELOG 记录 GIN_MODE 移除与响应头 LogID→X-Log-Id。
