# 执行计划：HTTP 层重写（去 Gin）

前置：design.md 已定稿；每步 = trellis-implement 派发（dispatch prompt 以 `Active task: .trellis/tasks/07-26-degin-http-rewrite` 开头）→ 主会话跑门禁 → 主会话按步提交（形成回滚点）。trellis-implement 不得 commit。

## 通用门禁（每步必跑）

```bash
gofmt -l .                 # 输出必须为空
go vet ./...
go test -race ./...
```

前端不受影响，pnpm 门禁只在步骤 4 改 CI/docs 后跑一次确认无联动破坏。

## Step 1：web/ 包 + klauspost/compress 引入（提交 ①）

不改任何现有行为；旧 router.go 的 embed 仍在。

- [ ] 新建 `web/web.go`（`Dist() (fs.FS, bool)` + placeholder HTML）、`web/embed_enabled.go`（`//go:build embed_assets` + `//go:embed all:dist`）、`web/embed_disabled.go`（`//go:build !embed_assets`，fstest.MapFS）。代码草案：research/embed-static.md §4。
- [ ] 根 `.gitignore` 加 `/web/dist/`。
- [ ] `go get github.com/klauspost/compress@v1.19.1`（gzhttp 下一步用，本步只入 go.mod；tidy 会保留因为 Step 2 前无引用——**改为在 Step 2 一起 go get**，本步不动 go.mod，避免 tidy -diff 门禁报未使用依赖）。
- [ ] 验证双构建：`go build ./...`（placeholder 分支）；`cp -r frontend/dist web/dist && go build -tags embed_assets ./... && rm -rf web/dist`（需本地已有 frontend/dist，无则先 `pnpm -C frontend build`）。
- [ ] 补 `web/web_test.go`：默认构建下 `Dist()` 返回含 index.html 的 FS 且 embedded=false。

风险文件：无（纯新增）。回滚：revert 提交 ①。

## Step 2：HTTP 层重写一刀（提交 ②，本任务核心）

编译原子性决定 main.go/router.go/handler.go/logs/根包测试必须同步改，不可再拆。

- [ ] `go get github.com/klauspost/compress@latest`（≥v1.19.1）。
- [ ] router.go 重写：删 embed 指令/embedFileSystem/EmbedFolder/acceptsGzip/appendVaryAcceptEncoding/gzipResponseWriter/gzipMiddleware/writeAPINotFound(gin 版)/apiLogContextMiddleware(gin 版)/RegisterRoutes(gin 版)；新写 `Routes() http.Handler`（design.md §1.1-§1.3 的链与路由表）、`writeJSON`、`isAPIPath`（保留）、apiLogContext/recovery/gzipSkipProbes 中间件、fallback（isAPIPath→404 JSON；favicon 等实存文件 no-cache 服务；否则 serveIndex）、serveIndex（启动预读 + 弱 ETag `W/"..."` + no-cache + http.ServeContent 零 modtime）、`GET /assets/` immutable + FileServerFS。gzhttp 配置照 design.md D1（EnableZstd(false) + ContentTypes 白名单 + MinSize 默认）。
- [ ] handler.go：四个 handler 改 `(w http.ResponseWriter, r *http.Request)` 签名，`c.JSON`→writeJSON，`GetContextFromGinContext`→`r.Context()`。信封字段结构逐字节不变（成功信封无 msg/log_id 字段——不要顺手统一）。
- [ ] main.go：删 `gin.SetMode`（main.go:40，GinMode 字段 Step 3 才删，本步只删调用行会引编译错？——**GinMode 字段在 config 包，main.go:40 删除后 config.GinMode 无人引用但仍编译**，安全）；`gin.New()+Recovery+RegisterRoutes` → `Handler: app.httpServer.Routes()`；main.go:62-66 promhttp 注释措辞更新（"router gzipMiddleware"→"gzhttp wrapper"）；启动日志加 placeholder 警告（web.Dist() 第二返回值）。
- [ ] logs/log_util.go：删 :15 gin import、SetNewContextForGinContext、GetContextFromGinContext。其余不动。
- [ ] 测试翻新（research/http-surface.md §5 对照表）：handler_test.go 删 gin import/SetMode、装配改 `srv.Routes()` 或直接 HandlerFunc；gzip 测试 fixture ≥1KB；`LogID`→`X-Log-Id` 3 处；metrics_endpoint_test.go `newMetricsTestRouter` 返回 http.Handler 且复用生产 wrapper 构造；删 router_test.go。新增测试：assets immutable 头、index ETag/304、/assets/missing 404、POST /api/unknown 404 JSON、placeholder fallback 200 text/html。
- [ ] `go mod tidy`：gin/gin-contrib 及传递依赖消失；`go list -m all | wc -l` 记录数字（预期 ≤15）。
- [ ] 门禁 + 双构建验证（同 Step 1 的双构建命令）+ `go test -tags embed_assets ./...`（web/dist 就位时抽跑一次确认嵌入分支测试也绿，跑完删 web/dist）。

风险文件：router.go（整文件重写）。回滚：revert 提交 ②（web/ 包仍在但无害）。

## Step 3：GIN_MODE 清除（提交 ③）

清单：research/config-docs-taskfile.md §1（file:line 全表）。

- [ ] config/config.go 6 处删除；config_test.go 8 处（`TestLoadGinModeAndLogCaller` → `TestLoadLogCaller`，保 LogCaller 维度）。
- [ ] .env.example:7-10、docs/development.md:23-24,29、docs/deployment.md:85,164。
- [ ] install.sh 12 处，**位置参数重排**：render_env_file `${11}` 删除后 download_base_url 前移，stage_install `${13}` 同步；install_test.sh:396-399 实参对齐。
- [ ] 验证：通用门禁 + `bash scripts/install_test.sh` + `shellcheck scripts/*.sh`。

回滚：revert 提交 ③。

## Step 4：Taskfile + CI + 文档 + CHANGELOG（提交 ④）

- [ ] 新增 `Taskfile.yml`（草案：research/config-docs-taskfile.md §4；含 frontend:install/frontend:build/build/test/check/vuln；build 拷 dist→web/dist 后 `go build -trimpath -tags embed_assets -ldflags "-s -w -X main.version={{.VERSION}}"`）。
- [ ] quality.yml：顶部注释锚点（与 Taskfile 同步）；新增"拷 dist→web/dist + `go build -tags embed_assets ./...`"步（排在 pnpm build 之后）；其余 go 步保持无 tag。
- [ ] release.yml：download-artifact path `frontend/dist`→`web/dist`；go build 加 `-tags embed_assets`。
- [ ] 文档命令统一：README.md:16(架构措辞),58-59、AGENTS.md:14-20、docs/development.md:33,36-61,78,113、docs/operations.md:40,130、docs/release.md:70 —— 构建入口改 `task build`/`task test`/`task check`（保留原生命令作为无 task 时的等价参考）。
- [ ] CHANGELOG Unreleased：`### Changed` X-Log-Id 条目 + `### Removed` GIN_MODE 条目（草案：research/config-docs-taskfile.md §5）。
- [ ] 验证：通用门禁 + 本地 `task check`/`task build` 冒烟（无 task 则 `winget install Task.Task` 或按草案手跑等价命令）+ `pnpm -C frontend lint/test` 确认无联动。

回滚：revert 提交 ④。

## 收尾（Phase 3）

- [ ] trellis-check 全量核对（对照 design.md §3 差异表逐条验证 + 契约不变项抽测）。
- [ ] 对抗验证双镜头（契约不变镜头 / 删除完整性镜头），模式同任务 2。
- [ ] 冒烟：本地 `task build` 产物起服务，curl /api/get_data（gzip 与 identity）、/healthz、/readyz、/metrics、SPA 深链接、/assets 缓存头。
- [ ] update-spec：api-contract.md（X-Log-Id、缓存头、gzhttp 行为）、logging-guidelines.md（删 gin 适配函数引用）、directory-structure.md（web/ 包、GIN_MODE 键表、Routes 签名）、quality-guidelines.md（Taskfile/CI 锚点、embed 双构建门禁）、operations 相关。
- [ ] 按步提交已在各 Step 完成；归档走 /trellis:finish-work 三段式。
- [ ] 提醒用户：CI 改动需推 GitHub 观察 Actions 实跑；quality.yml 新步骤首跑注意 artifact 路径。
