# 技术设计：HTTP 层重写（去 Gin、gzhttp、embed 解耦、Taskfile）

证据基础：`research/http-surface.md`、`research/logs-decoupling.md`、`research/gzip-matrix.md`、`research/embed-static.md`、`research/config-docs-taskfile.md`（全部含 file:line 锚点，本文只引结论）。

## 1. 目标架构

### 1.1 请求处理链（外 → 内）

```
http.Server（超时/优雅停机配置原样保留，main.go:103-150 零改动）
  └─ gzipSkipProbes        ── /healthz、/readyz 按路径绕过压缩（复刻现状 router.go:203-206 的豁免语义，含"无 Vary"）
       └─ gzhttp wrapper   ── klauspost/compress/gzhttp v1.19.1，配置见 §2.2
            └─ apiLogContext ── 仅 isAPIPath 时：GenNewContext + 写 X-Log-Id 头 + r.WithContext
                 └─ recovery ── 最内层：recover → slog.ErrorContext（带 log_id）→ 未写头则 500
                      └─ http.ServeMux
```

recovery 在 gzip **内侧**是有意的行为修复（非现状复刻）：现状 Recovery 在最外层，panic 展开时 `defer gz.Close()` 先输出 gzip 尾帧再补 500，响应已损坏；新顺序让 panic 在到达 gzhttp 的 writer Close 前就转成干净的 500。

### 1.2 ServeMux 路由

```go
mux.HandleFunc("GET /healthz",       s.Healthz)
mux.HandleFunc("GET /readyz",        s.Readyz)
mux.HandleFunc("GET /metrics",       s.Metrics)
mux.HandleFunc("GET /api/get_data",  s.GetData)
mux.Handle    ("GET /assets/",       immutableCache(http.FileServerFS(distFS)))
mux.Handle    ("/",                  fallback)   // 方法无关兜底
```

`fallback` 复刻现状 NoRoute 双分支 + static.Serve 的合并语义：

1. `isAPIPath(path)` → JSON 404 信封 `{"code":404,"msg":"not found","log_id":...}`（log_id 来自 r.Context()，中间件已注入）。
2. 非 API 且 dist 中存在该文件（当前只有 favicon.ico 会命中）→ `Cache-Control: no-cache` + FileServerFS 服务。
3. 其余（`/`、`/index.html`、SPA 深链接）→ serveIndex：启动时读取一次的 index.html 字节 + 弱 ETag + `Cache-Control: no-cache`，经 `http.ServeContent`（modtime 传零值，纯走 If-None-Match→304）。

方法无关的 `"/"` 兜底保证 ServeMux 永不产生 405：`POST /api/get_data` → fallback → JSON 404，与 gin（HandleMethodNotAllowed=false）现状一致。

### 1.3 装配签名

`RegisterRoutes(r *gin.Engine)` → `func (s *HTTPServer) Routes() http.Handler`：内部建 mux、组合全部中间件、完成 index.html 预读与 ETag 计算（出错 fatal——嵌入产物必须含 index.html，placeholder FS 也满足）。main.go 只改 `Handler: app.httpServer.Routes()`。handler 签名全部 `func(w http.ResponseWriter, r *http.Request)`；`c.JSON` → 包内 `writeJSON(w, status, v)` helper（`Content-Type: application/json; charset=utf-8`）。

### 1.4 包布局（R8 决策：保持平铺）

router.go / handler.go 及测试**留在根包**，不迁 `internal/httpapi/`。理由：`version` 变量被 `-X main.version` 注入绑定在 main 包（spec api-contract.md 的 gotcha），Readyz 直接读它；迁包需把 version 穿参传递，本次重写 diff 已经很大，迁移收益（目录整洁）不抵审阅成本。新增包只有 `web/`（嵌入资产）。

## 2. 关键决策

### D1 gzhttp 配置（R4）

```go
gzhttp.NewWrapper(
    gzhttp.EnableZstd(false),        // 保持 gzip-only 行为面；zstd 留待后续一行开启
    gzhttp.ContentTypes([]string{    // 显式白名单：不含 png/woff2/octet-stream
        "application/json", "text/html", "text/plain", "text/css",
        "text/javascript", "application/javascript", "image/svg+xml",
        "application/xml", "application/wasm",
    }),
    gzhttp.MinSize(gzhttp.DefaultMinSize), // 1024，显式写出当文档
)
```

- 手写实现实际是"无条件全量压缩"（无白名单、无阈值、空体也发 gzip 帧、HEAD/Range/Flush 均有缺陷——research/gzip-matrix.md §1）。**PRD R4 中"（含 Vary、Content-Type 白名单、最小长度）"描述的是 gzhttp 带来的能力，不是现状等价替换**；本次是行为修复，逐条差异见 §3。
- `text/plain` 无参数条目匹配 `text/plain; version=0.0.4`（/metrics exposition）。
- promhttp `DisableCompression: true` 保留（gzhttp 对已设 Content-Encoding 的响应天然跳过，双保险 + metrics_endpoint_test.go:31-35 契约）；仅更新 main.go:62-66 注释措辞。
- /healthz、/readyz 用路径 shim 完全绕过 wrapper（非依赖 MinSize 侥幸不压），保持"永不压缩、无 Vary"与现状逐字节一致。

### D2 embed 解耦：`web/` 包 + 构建标签（R6）

三文件方案（代码草案见 research/embed-static.md §4）：

- `web/web.go`：`Dist() (fs.FS, bool)` 公共接口 + placeholder HTML 常量。
- `web/embed_enabled.go`：`//go:build embed_assets` + `//go:embed all:dist`。
- `web/embed_disabled.go`：`//go:build !embed_assets`，`fstest.MapFS` 返回单文件 index.html 提示页（"frontend not built, run task build"）。

dist 位置取**方案 B**：Taskfile 在 `frontend:build` 后把 `frontend/dist` 拷到 `web/dist`（根 .gitignore 加 `/web/dist/`）；release.yml 的 download-artifact `path:` 直接指到 `web/dist` 免拷贝。否决方案 A（Go 包放 frontend/ 会让 go 工具链遍历 node_modules）与方案 C（vite outDir 跨界写 Go 包目录）。

main 启动时若 `Dist()` 第二返回值为 false，打日志 "serving placeholder frontend (built without embed_assets)"，防线上误发裸二进制。

### D3 缓存头与 ETag（R5）

| 资源 | Cache-Control | ETag |
|---|---|---|
| `/assets/*`（带 hash） | `public, max-age=31536000, immutable` | 无（hash 即版本） |
| index.html（含 SPA fallback） | `no-cache` | **弱 ETag** `W/"<sha256 前 8 字节 hex>"`，启动时算一次 |
| favicon.ico（public/ 无 hash） | `no-cache` | 无 |

用弱 ETag 的原因：gzhttp 压缩变体默认保留原 ETag，同一强 ETag 对应两种字节流违反 RFC 7232；弱 ETag 语义上允许跨编码等价，`http.ServeContent` 的 If-None-Match 比对本就是弱比较，304 正常工作，不需要 SuffixETag/DropETag。

### D4 logs 解耦（R2）

- 删 `SetNewContextForGinContext`（log_util.go:58-62）、`GetContextFromGinContext`（log_util.go:98-108，PRD 所写 102-112 为 4 行偏差）及 :15 的 gin import。logs 包其余（GenNewContext/GetLogIDFromContext/logger.go）零改动。
- apiLogContext 中间件放**根包**（不放 logs）：`ctx := logs.GenNewContext(r.Context()); w.Header().Set("X-Log-Id", logs.GetLogIDFromContext(ctx)); next.ServeHTTP(w, r.WithContext(ctx))`，仅 isAPIPath 时生效——SPA fallback 必须无该头（handler_test.go:236-237 契约）。
- handler/404 取 ctx 一律 `r.Context()` 直取。service 层零改动（进入 service 已是纯 context）。
- 头名 `X-Log-Id`（CHANGELOG 记录）。前端/nginx/install.sh 均不读旧头名，影响面仅 docs/operations.md:130 与测试断言。

### D5 GIN_MODE 清除（R3）

config.go 6 处 + main.go:40 + config_test 8 处（`TestLoadGinModeAndLogCaller` 不整删，改造成 `TestLoadLogCaller` 保留 LogCaller 维度）+ .env.example + docs 4 处 + install.sh 12 处。install.sh 的 `render_env_file(${11})`/`stage_install(${13})` 是位置参数，删 gin_mode 必须同步重排两个签名、两处调用点与 install_test.sh:396-399 的实参，否则 download_base_url 错位收到 "release"。旧部署 env 残留 `GIN_MODE=` 被 config.Load 静默忽略，升级天然安全（CHANGELOG 说明）。

### D6 Taskfile 与 CI 分工（R7，修订 PRD 措辞）

**CI 保留原生命令，Taskfile 只做本地开发入口；文档命令统一指向 task。** 理由：quality.yml 15 个细粒度 step 的分步日志与失败定位价值高于去重；本仓 CI 惯例是 action 全部钉 SHA，引入 arduino/setup-task 是新的供应链依赖；漂移风险靠 quality.yml 顶部注释锚点（"与 Taskfile.yml 同步"）+ 验收条目控制。Taskfile.yml 草案见 research/config-docs-taskfile.md §4（`build` 的 ldflags 逐字对齐 release.yml:62 再加 `-tags embed_assets`；`test` 不带 tag 与 ldflags——`go test -ldflags -X main.version` 是静默 no-op）。

### D7 CI 构建标签接线（R6 的 CI 面）

| 位置 | tag | 理由 |
|---|---|---|
| quality.yml `go vet` / `go test -race` / `go build ./...` | 不加 | 裸克隆可编译门禁（placeholder 分支） |
| quality.yml **新增步**：拷 dist → web/dist 后 `go build -tags embed_assets ./...` | 加 | embed 模式失配必须在 PR 门禁暴露，不能拖到 release |
| release.yml `go build`（:62） | 加 | download-artifact path 同步改 `web/dist` |
| govulncheck | 不加（维持现状） | 无第三方依赖差异，低风险 |

## 3. 显式接受的行为差异（迁移非逐字节等价，逐条拍板）

契约不变项：4 条路由的状态码/信封/Content-Type、API 404 JSON、SPA fallback 200 text/html、/metrics 单层压缩、probe 永不压缩、log_id 信封字段。以下为有意差异：

| # | 场景 | 现状（gin） | 新行为 | 定性 |
|---|---|---|---|---|
| B1 | 响应头名 | `LogID` | `X-Log-Id` | 显式批准，CHANGELOG |
| B2 | `GIN_MODE` | 三值校验生效 | 配置删除，残留被忽略 | 显式批准，CHANGELOG |
| B3 | 尾斜杠 `/api/get_data/` 等 | 301 重定向 | 404 JSON / SPA | 接受：无测试、无调用方 |
| B4 | png/woff2/小于 1KB 响应 | 压缩 | 不压缩 | 接受（即验收要求） |
| B5 | 空体/204/错误短响应 | 输出 gzip 空流帧 | 不压缩 | 修复 |
| B6 | HEAD 请求 | 照压 + 设 CE 头 | 一律不压；HEAD /healthz 从 SPA 200 html 变 200 空体 | 修复/接受 |
| B7 | `Accept-Encoding: *` 及 `q=1.5`、`q = 0` 等畸形头 | 手写语义 | gzhttp 语义（`*` 不压、越界钳制） | 接受：边角；metrics_endpoint_test 已写成两可 |
| B8 | `/assets/不存在` | 回落 index.html 200 | 真 404 | 修复（错资源名不该拿到 HTML） |
| B9 | `/index.html` | 301 → `./` | 200 直接服务 | 接受 |
| B10 | 静态响应缓存头 | 全无 | ETag/Cache-Control 按 D3 | 增强 |
| B11 | panic 日志 | gin stderr 纯文本无 log_id | slog JSON 带 log_id | 增强，无测试锁定 |
| B12 | 路径含 `//`、`..` | 直接进 SPA | ServeMux 先 301 清洗 | 接受（更安全） |

## 4. 测试策略

- **装配翻新**：根包测试已全部是 httptest 模式，仅装配行改 `srv.Routes()`（research/http-surface.md §5 有逐测试对照表）。`gin.SetMode` 删除。
- **删除**：router_test.go `TestAcceptsGzip`（15 case 全部实现级，函数随迁移消失；q=0/识别语义由 metrics_endpoint_test.go:90-108 在 HTTP 层兜底）。
- **必改**：gzip 压缩断言 fixture 从 128B 加大到 ≥1KB（gzhttp MinSize=1024；不为迁就测试改产品配置）；`LogID` 头断言 3 处 → `X-Log-Id`；metrics_endpoint_test 的 `newMetricsTestRouter` 改为复用**生产同款** wrapper 配置（不许测试私配一套 gzip 选项）。
- **新增**：`/assets/*` immutable 头；index.html no-cache + ETag + If-None-Match→304；`/assets/missing` 404；placeholder 模式 SPA fallback 200 text/html（默认构建下天然覆盖——CI 测试恒走 placeholder 分支）；`POST /api/unknown` JSON 404（守住方法无关兜底）。
- /metrics gzip case 依赖 exposition ≥1024 字节，实现时实测确认，不足则测试内多 observe 几组直方图。

## 5. 依赖变化

- 新增：`github.com/klauspost/compress v1.19.1`（纯 Go，零传递依赖）。
- 删除：`gin-gonic/gin`、`gin-contrib/static` 直接依赖及 ~30 个传递模块（sonic/quic-go/mongo-driver/validator 等）。`go mod tidy` 后 `go list -m all` 预期 ≤15。

## 6. 回滚与提交切分

四个独立提交点（详见 implement.md）：① web/ 包 + 依赖引入（不动现有行为）→ ② HTTP 层重写一刀（编译原子性决定不可再拆，含测试翻新）→ ③ GIN_MODE 清除（config/install.sh/docs）→ ④ Taskfile + CI + 文档命令统一 + CHANGELOG。任一步出问题 `git revert` 单提交即回滚，前序提交独立成立。
