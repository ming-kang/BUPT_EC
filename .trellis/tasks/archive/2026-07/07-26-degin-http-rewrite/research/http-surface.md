# Research: HTTP 面完整清单与 gin→net/http 迁移映射

- **Query**: 枚举全部路由/中间件/gin API 使用点/响应契约/ServeMux 语义差异/测试装配依赖
- **Scope**: internal
- **Date**: 2026-07-27

## 1. 路由注册、中间件链、http.Server 装配、graceful shutdown

### 1.1 路由清单（router.go:67-101 `RegisterRoutes`）

| 路由 | 注册点 | Handler | 说明 |
|---|---|---|---|
| `GET /healthz` | router.go:71 | `server.Healthz`（handler.go:63） | gzip 中间件显式跳过 |
| `GET /readyz` | router.go:72 | `server.Readyz`（handler.go:67） | gzip 中间件显式跳过 |
| `GET /metrics` | router.go:73 | `server.Metrics`（handler.go:84） | 代理 promhttp，nil 时 404 |
| `GET /api/get_data` | router.go:75-78（`r.Group("/api")` + `apiGroup.GET`） | `server.GetData`（handler.go:42） | 唯一业务 API |
| 静态文件（middleware） | router.go:80 `r.Use(static.Serve("/", EmbedFolder(f, "frontend/dist")))` | gin-contrib/static | 见 1.2 生效范围说明 |
| NoRoute | router.go:82-100 | 匿名闭包 | 双分支：API JSON 404 / SPA fallback |

embed 入口：router.go:17-18 `//go:embed frontend/dist` + `var f embed.FS`；`EmbedFolder`（router.go:33-41）经 `fs.Sub` + `http.FS` 包成 `static.ServeFileSystem`，`embedFileSystem.Exists`（router.go:24-31）用 Open 探测文件存在性。**`frontend/dist` 被 .gitignore 忽略（`git check-ignore frontend/dist` 命中），裸克隆下 `go vet/test ./...` 直接编译失败——这就是 R6 的动因。**

### 1.2 中间件链与精确顺序

装配点 main.go:97-99：

```go
r := gin.New()
r.Use(gin.Recovery())          // main.go:98
app.httpServer.RegisterRoutes(r)
```

RegisterRoutes 内部：`r.Use(gzipMiddleware())`（router.go:68）→ `r.Use(apiLogContextMiddleware())`（router.go:69）→ 注册 4 条路由 → `r.Use(static.Serve(...))`（router.go:80）→ `r.NoRoute(...)`（router.go:82）。

gin 语义下的**两条实际链**（gin 在路由注册时快照当时的全局中间件，static 在 4 条路由之后 Use，因此不在它们的链里；NoRoute 链则始终重建为全量全局中间件）：

- 已注册路由（4 条）：`Recovery → gzip → apiLog → handler`
- 未匹配路径（NoRoute）：`Recovery → gzip → apiLog → static.Serve → NoRoute 闭包`

即：**静态文件服务只发生在"未匹配已注册路由"的路径上**；`static.Serve` 内部 Exists 命中则用 `http.FileServer` 直接响应并 Abort，未命中则 `c.Next()` 落到 NoRoute 闭包。

`apiLogContextMiddleware`（router.go:49-56）：仅当 `isAPIPath`（router.go:43-45，`path == "/api" || strings.HasPrefix(path, "/api/")`）时调用 `logs.SetNewContextForGinContext(c)`（logs/log_util.go:58-62）——生成新 log_id 上下文、`c.Set("ctx", newCtx)`、**响应头 `LogID`**（log_util.go:61）。非 API 流量不动。

### 1.3 http.Server 装配（main.go:103-114）

| 项 | 值 | 锚点 |
|---|---|---|
| Addr | `runtimeConfig.AppAddr`（默认 `127.0.0.1:8080`，config.go:21） | main.go:101,104 |
| Handler | `r`（gin.Engine） | main.go:105 |
| ReadHeaderTimeout | 5s | main.go:106 |
| ReadTimeout | 15s | main.go:107 |
| WriteTimeout | `httpWriteTimeout = 45s`（常量注释：必须 > `service.ClassroomRefreshLimit`） | main.go:83, 111 |
| IdleTimeout | 60s | main.go:112 |
| MaxHeaderBytes | 1<<20 | main.go:113 |

### 1.4 Graceful shutdown 流程（main.go:116-150）

1. `server.ListenAndServe()` 在 goroutine 中跑，错误（非 ErrServerClosed）经 `serverErr` chan 上报（main.go:116-124）。
2. `signal.Notify(stop, SIGINT, SIGTERM)`（main.go:126-128）；`select` 等 serverErr 或信号（main.go:131-135）。
3. 先 `stopBackground()`（cancel appCtx，停调度器，main.go:139）→ `server.Shutdown(ctx)`，超时 `gracefulShutdownTimeout = httpWriteTimeout + 5s = 50s`（main.go:87, 140-144）→ `app.classroomService.WaitBackground(ctx)`（main.go:145-147）→ serveErr 非 nil 则 `log.Fatalf`。

迁移影响：这段与 gin 无耦合，Handler 换成 `http.Handler` 组合链即可，其余零改动。main_test.go:8-22 只断言两个超时常量关系，也不受影响。

## 2. gin API 使用点全表 + net/http 等价写法

### 2.1 生产代码

| gin 符号 | 位置 | net/http 等价 |
|---|---|---|
| `gin.SetMode(runtimeConfig.GinMode)` | main.go:40 | 删除（连同 R3 的 GIN_MODE 配置：config.go:18,22,41,71,81-83,98-99） |
| `gin.New()` | main.go:97 | `http.NewServeMux()` |
| `gin.Recovery()` | main.go:98 | 自写 recovery 中间件：`defer func(){ if v := recover(); v != nil { slog.Error(...); 若未写头则 w.WriteHeader(500) } }()`；按 PRD R1 放在**最内层**（直接包 mux，gzip 包在它外面） |
| `gin.HandlerFunc` 返回值 | router.go:49, 201 | `func(next http.Handler) http.Handler` 包装器 |
| `func(c *gin.Context)` handler 签名 | handler.go:42,63,67,84; router.go:50,58,82,202 | `func(w http.ResponseWriter, r *http.Request)` |
| `c.JSON(status, gin.H{...})` | router.go:60-64; handler.go:48-53, 57-60, 64, 76-81 | `writeJSON(w, status, v)` helper：`w.Header().Set("Content-Type", "application/json; charset=utf-8")`（gin c.JSON 的精确 Content-Type，测试只断言含 `application/json`）→ `w.WriteHeader(status)` → `json.NewEncoder(w).Encode(v)` |
| `gin.H` | 同上各处 | `map[string]any` 或具名 struct（信封字段见 §3） |
| `c.Data(200, "text/html; charset=utf-8", data)` | router.go:99 | `w.Header().Set("Content-Type", "text/html; charset=utf-8"); w.WriteHeader(200); w.Write(data)`（R5 改为启动时读一次 + ETag + no-cache） |
| `c.Status(404/500)` | router.go:90, 96; handler.go:86 | `w.WriteHeader(...)` |
| `c.GetHeader("Accept-Encoding")` | router.go:207 | `r.Header.Get("Accept-Encoding")`（gzhttp 内部自理，此处整体删除） |
| `c.Header("Content-Encoding", "gzip")` | router.go:215 | `w.Header().Set(...)`（gzhttp 自理） |
| `c.Writer` / `gin.ResponseWriter` 替换 | router.go:186-199（gzipResponseWriter）, 212-218 | 整体被 `gzhttp.GzipHandler` / `gzhttp.NewWrapper` 取代（R4） |
| `c.Next()` | router.go:54, 204, 208, 219 | 中间件模式改为 `next.ServeHTTP(w, r)` |
| `c.Set("ctx", newCtx)` | logs/log_util.go:60（`SetNewContextForGinContext`，调用点 router.go:52） | `r = r.WithContext(logs.GenNewContext(r.Context()))`（R2：删除该函数，logs 包去 gin） |
| `c.Get("ctx")` | logs/log_util.go:102（`GetContextFromGinContext`，调用点 handler.go:43, router.go:59） | `r.Context()` 直取（log_id 已在 ctx 里）；`GetLogIDFromContext` 保留不动 |
| `c.Writer.Header().Set("LogID", ...)` | logs/log_util.go:61 | 中间件里 `w.Header().Set("X-Log-Id", id)`（R2 改名，用户可见，CHANGELOG） |
| `c.Request` | router.go:51, 83; log_util.go:59, 99, 107 | 直接用 `r` |
| `r.Group("/api")` + `apiGroup.GET("/get_data")` | router.go:75-78 | `mux.HandleFunc("GET /api/get_data", ...)`（无需 group 概念） |
| `r.GET("/healthz" 等)` | router.go:71-73 | `mux.HandleFunc("GET /healthz", ...)` 等三条 |
| `r.NoRoute(...)` | router.go:82 | 注册方法无关的 `mux.Handle("/", fallbackHandler)` 兜底（见 §4.1 论证），fallback 内部先判 `isAPIPath` → JSON 404，否则静态文件/index.html |
| `static.Serve` + `EmbedFolder` + `embedFileSystem` | router.go:13, 20-41, 80 | `http.FileServerFS(subFS)` 或自写 open-if-exists：`fs.Stat(subFS, name)` 命中则交给 FileServerFS，否则 SPA fallback（R5/R6 下沉到 `web/` 包） |
| `gin.Engine` 参数类型 | router.go:67（`RegisterRoutes(r *gin.Engine)`） | 建议改签名为 `func (s *HTTPServer) Routes() http.Handler`（内部建 mux + 包中间件），测试直接拿 handler |

utils/http_test.go 的 grep 命中（33,35,67,69）是 `origin.Close`/`context` 误报，与 gin 无关。`service/`、`config/`、`logs/init_test.go` 无 gin 引用；**gin 依赖面 = 根包 4 文件 + logs/log_util.go 两个适配函数**。

### 2.2 gzip 中间件现状（将被 gzhttp 整体替换的行为基线）

- `acceptsGzip`（router.go:106-171）：只协商 gzip，大小写不敏感、支持 q 值、格式坏 token 按拒绝、`*` 仅在无显式 gzip token 时放行。测试 router_test.go:5-35 全文枚举了 15 个用例——迁移后此函数与测试一并删除，语义由 gzhttp 承担（gzhttp 有自己的 Accept-Encoding q 值解析）。
- `gzipMiddleware`（router.go:201-221）：**仅跳过 `/healthz`、`/readyz`（router.go:203）；其余一切路径（含 /metrics、含 png/woff2 静态资源）只要客户端接受就压**；无最小长度、无 Content-Type 白名单；`Vary: Accept-Encoding` 仅在实际压缩时追加（router.go:216, `appendVaryAcceptEncoding` router.go:173-184）；写时删 Content-Length（router.go:192,197,217）。
- promhttp 侧 `DisableCompression: true`（main.go:64-66），注释明言 router gzip 是唯一 Accept-Encoding 所有者，避免双重压缩——gzhttp 替换后此约定保持不变（R4）。

## 3. 每条路由的响应契约

LogID 头：所有 `/api` 前缀请求（含未匹配的 /api 404）响应头带 `LogID: <20060102150405+18位大写hex>`（log_util.go:61, 70-72）；非 API 路径**必须没有** LogID 头（handler_test.go:236-238 显式断言）。R2 后改名 `X-Log-Id`。

| 路由/场景 | 状态码 | 头 | JSON 信封 | 锚点 |
|---|---|---|---|---|
| `GET /api/get_data` 成功 | 200 | `Content-Type: application/json; charset=utf-8`、`LogID`、（协商后）gzip+Vary | `{"code":0,"data":<TodayClassrooms>}` —— 成功信封**无 msg、无 log_id 字段** | handler.go:57-60 |
| `GET /api/get_data` 失败 | 503 | 同上 | `{"code":503,"msg":<SafeErrorMessage>,"log_id":<id>,"data":null}` | handler.go:48-53 |
| `GET /healthz` | 200 | JSON CT；**永不 gzip** | `{"status":"ok"}` | handler.go:64; router.go:203 |
| `GET /readyz` | 200 或 503（ready = hasJWCredentials && HasUsableTodayCache） | JSON CT；**永不 gzip** | `{"status":http.StatusText(code),"jw_credentials_configured":bool,"runtime":<RuntimeStatus>,"version":version}` | handler.go:67-82 |
| `GET /metrics`（handler 存在） | 200 | promhttp 自设 CT；经中间件 gzip（单层） | Prometheus 文本 | handler.go:84-90; main.go:64-66 |
| `GET /metrics`（handler 为 nil） | 404 | 空体 | — | handler.go:85-87 |
| NoRoute，`isAPIPath` 为真（`/api` 或 `/api/...`） | 404 | JSON CT、`LogID` | `{"code":404,"msg":"not found","log_id":<id>}` | router.go:58-65, 84-87 |
| NoRoute，非 API，静态文件存在 | 200 | FileServer 自定 CT | 文件内容 | router.go:80 |
| NoRoute，非 API，无文件（SPA 深链接） | 200 | `text/html; charset=utf-8`；无 LogID | index.html 全文 | router.go:88-99 |
| NoRoute，index.html 打不开 | 404 空体 | — | — | router.go:89-92 |
| NoRoute，index.html 读失败 | 500 空体 | — | — | router.go:94-97 |

NoRoute 双分支的**精确条件**就是 `isAPIPath(path)`（router.go:43-45）：`path == "/api" || strings.HasPrefix(path, "/api/")`。handler_test.go:240-268 对 `/api/nonexistent` 与 `/api` 两个路径都断言 404 JSON + LogID 头与 body log_id 一致。

## 4. gin vs Go 1.22 ServeMux 语义差异陷阱（逐条评估）

### 4.1 方法不匹配：gin 404 vs ServeMux 405

- gin.New() 默认 `HandleMethodNotAllowed=false`：`POST /api/get_data` 走 NoRoute → isAPIPath → **JSON 404**；`POST /healthz` 走 NoRoute → SPA fallback 200。
- 裸 ServeMux 只注册 `GET /api/get_data` 时，POST 会得 **405 + `Allow: GET`**。
- **但本项目必然注册方法无关的兜底 `mux.Handle("/", fallback)`**（SPA/静态）。ServeMux 规则：405 仅在"无任何 pattern 匹配、但存在同路径异方法 pattern"时返回；`"/"`（无方法）匹配一切方法与路径，因此 `POST /api/get_data`、`POST /healthz` 都会命中 `"/"` 兜底——fallback 里的 isAPIPath 分支使行为与 gin 现状**完全一致**（API JSON 404 / 非 API SPA 200），405 永远不会出现。
- **决策建议**：4 条路由全部用 `GET ` 前缀 pattern + `"/"` 方法无关兜底，无需任何额外处理即保持现状。
- 次要差异：ServeMux 的 `GET` pattern **同时匹配 HEAD**（handler 照常执行，net/http 自动丢弃 body）。gin 现状 HEAD /healthz 走 NoRoute→SPA(200 html)；迁移后 HEAD /healthz 变为 200 空体 + JSON CT。无测试覆盖，属可接受改进，建议 design.md 记一句。

### 4.2 尾斜杠

- gin.New() 默认 `RedirectTrailingSlash=true`：`GET /api/get_data/` → **301 重定向**到 `/api/get_data`（非 GET 则 307）。
- ServeMux 精确 pattern `/api/get_data`（不以 / 结尾）不做重定向：`/api/get_data/` 落到 `"/"` 兜底 → isAPIPath → **JSON 404**。`/healthz/` 同理落 SPA fallback（与 gin 现状一致，因为 gin 对 `/healthz/` 会 301 到 /healthz……注意：gin 对 `/healthz/` 也是 301）。
- 影响：`/api/get_data/`、`/healthz/` 等带尾斜杠请求从 301 变 404/SPA。无测试覆盖、前端不发这种请求。**建议接受该差异，不做 gin 式重定向模拟**，design.md 声明即可。

### 4.3 `"/"` 兜底 pattern 匹配一切

- ServeMux 中 `"/"` 是 subtree pattern，匹配所有未被更具体 pattern 命中的请求（任意方法）。这正好是 NoRoute+static 合并后的落点：**兜底 handler 必须自带 isAPIPath → JSON 404 分支**，否则 `/api/xxx` 未知路由会掉进 SPA fallback（违反 handler_test.go:240-268）。
- 若需要"仅根路径"语义可用 `/{$}`，本项目不需要。
- 优先级无忧：`GET /api/get_data` 等具体 pattern 严格比 `"/"` 更具体，注册二者不冲突、命中时具体者赢。

### 4.4 Pattern 冲突 panic

- ServeMux 对"互相不可比"的重复/冲突 pattern 在 `Handle` 时 panic。本项目 5 个 pattern（4×`GET /path` + `/`）两两有严格特异序，无冲突。
- 真正的风险在**测试**：同一个 mux 重复注册同 pattern 会 panic。现测试每个用例 `gin.New()` 新建引擎（handler_test.go:66,106,148,183,224,273; metrics_endpoint_test.go:43），迁移后保持"每用例新建 mux/handler"即可。

### 4.5 路径清洗与重定向

- ServeMux 会对含 `//`、`.`、`..` 的路径先 301 重定向到清洗后路径（CONNECT 除外）；gin.New() 默认 `RedirectFixedPath=false`、`RemoveExtraSlash=false`，这类路径直接进 NoRoute→SPA。差异极小且 ServeMux 行为更安全，接受即可。

### 4.6 中间件作用域差异

- gin 的"static 只存在于 NoRoute 链"这种注册序技巧在 net/http 中消失：中间件是全局函数组合。等价组合建议（外→内）：`gzhttp → logCtx(仅 isAPIPath 时注入 log_id+X-Log-Id，本来就按路径条件判断，全局化无副作用) → recovery → mux`。PRD R1 要求 recovery 在最内层（gzip 之内），使 panic 在到达 gzhttp 的 writer Close 前就被转成 500——当前 gin 版顺序（Recovery 最外、gzip 在内，main.go:98 + router.go:68）实际上存在"panic 展开时 `defer gz.Close()`（router.go:213）先写 gzip 尾再由 Recovery 补 500"的隐患，迁移顺序是修复而非等价保留。
- 注意 net/http 下 `w.Header()` 必须在 `WriteHeader` 前设置；LogID 头在中间件里先 Set 再调 next，天然满足。

### 4.7 gzhttp 与手写 gzip 的行为差异（影响测试断言）

- **MinSize 默认 1400 字节**：现有 `TestGzipMiddlewareCompressesAPIAndSkipsHealthz` 用 128 字节 body 断言必压缩（handler_test.go:275-301），gzhttp 默认下不会压。要么 `gzhttp.NewWrapper(gzhttp.MinSize(N))` 调小，要么测试 body 加大到 >1400 —— PRD R4 本就把"最小长度"列为期望特性，建议改测试。
- **Content-Type 白名单**（`gzhttp.ContentTypes`/默认过滤器）：png/woff2 将不再压缩——这是验收标准里的**有意行为变化**（现状是会压）。
- `/healthz`、`/readyz` 的"永不压缩"现由路径硬编码保证（router.go:203）；gzhttp 下其响应远小于 MinSize，天然不压，`TestMetricsEndpointHealthzAndReadyzStayUncompressed`（metrics_endpoint_test.go:111-124）可不改装配语义直接通过；若要严格保留"即使大也不压"的路径豁免，需在这两条路由外单独绕过 gzhttp 包装。
- Vary：gzhttp 对可压缩类型即使未压也可能加 Vary；现测试只在压缩响应上断言 Vary（handler_test.go:287, metrics_endpoint_test.go:72-74），方向兼容。
- `TestAcceptsGzip`（router_test.go 全文件）随 `acceptsGzip` 删除；其 q 值语义（`gzip;q=0`、`identity, gzip;q=0`、`*` 等）已由 metrics_endpoint_test.go:90-108 在 HTTP 层语义级覆盖，gzhttp 自带 q 值解析可通过。

## 5. 根包测试对 gin 的装配依赖与改法

`gin.CreateTestContext` **未被使用**；所有测试已是 `httptest.NewRecorder + httptest.NewRequest + ServeHTTP` 模式，迁移只动"装配"两三行。

| 测试 | gin 依赖点 | 改法 |
|---|---|---|
| handler_test.go init | `gin.SetMode(gin.TestMode)`（:21-23） | 删除 |
| `TestReadyzRequiresConfiguredCredentialsAndUsableCache` | `gin.New()` + `router.GET("/readyz", httpServer.Readyz)`（:66-71） | handler 签名改 `(w,r)` 后：`mux := http.NewServeMux(); mux.HandleFunc("GET /readyz", srv.Readyz); mux.ServeHTTP(rec, req)`，或直接 `http.HandlerFunc(srv.Readyz).ServeHTTP(rec, req)` |
| `TestReadyzReportsPartialCacheDiagnostics` | 同上（:106-111） | 同上 |
| `TestGetDataReturnsSuccessEnvelope...` | `gin.New()` + `RegisterRoutes(router)` + ServeHTTP（:148-153） | `RegisterRoutes` 改为返回 `http.Handler`（含全部中间件），测试 `h := srv.Routes(); h.ServeHTTP(rec, req)` |
| `TestGetDataReturnsSafeErrorEnvelopeWithLogID` | 同上（:183-188）；断言 `LogID` 头（:211-217） | 同上；头名改 `X-Log-Id`（R2） |
| `TestNoRouteServesSPAFallback` | 同上（:224-229）；断言非 API 无 LogID（:236-238）、API 404 信封（:240-268） | 同上；语义断言不变，头名改 `X-Log-Id` |
| `TestGzipMiddlewareCompressesAPIAndSkipsHealthz` | `gin.New()` + `router.Use(gzipMiddleware())` + `c.String` 闭包（:273-279） | 重写为对 gzhttp 包装后 handler 的语义测试：`h := wrapGzip(mux)`；`c.String` → `io.WriteString(w, ...)`；body 需 >MinSize（见 §4.7） |
| metrics_endpoint_test.go `newMetricsTestRouter` | `*gin.Engine` 返回类型、`gin.New()`、`Use(gzipMiddleware())`、三条 `router.GET`（:37-49） | 返回 `http.Handler`：mux 注册三条 `GET` pattern 后套同一 gzip 包装；`serveMetrics` 已收 `http.Handler`（:136）零改动 |
| router_test.go 全文件 | 纯函数测试 `acceptsGzip`（:5-35） | 随函数删除（语义由 §4.7 所述 HTTP 层测试兜底） |
| main_test.go | 无 gin | 不动 |

额外装配注意：`RegisterRoutes` 现在直接引用包级 `f embed.FS`（router.go:17,80,88）；R6 下沉到 `web/` 包后，SPA fallback 相关测试（`TestNoRouteServesSPAFallback` 期望 200 text/html）在默认构建（无 `embed_assets` tag，空 FS + "frontend not built" 提示页）下也要能过——提示页同样是 200 `text/html` 即可满足现断言，或让 Routes 支持注入 fs.FS 以便测试自备 index.html。

## Caveats / Not Found

- gin 默认 `RedirectTrailingSlash=true` 的 301 行为无任何测试覆盖，迁移后变 404/SPA——需要在 design.md 明确"接受差异"。
- gzhttp 默认 MinSize=1400 与现有 128 字节压缩断言冲突（§4.7），实现前必须定测试策略。
- 现有成功信封 `{"code":0,"data":...}` 不含 log_id/msg 字段；写 writeJSON helper 时不要"顺手统一"信封结构（会破坏 handler_test.go:158-170 的解码断言语义）。
- `logs.Init` 签名 `(isMain, addSource bool)`（logs/log_util.go:29）与 gin 无关，R2 只需删 log_util.go:58-62、98-108 两个函数及 :15 的 import。
- 未验证 gzhttp 对 `gzip;q=0, *;q=1`（显式 gzip 拒绝优先于通配）这一精确用例的行为，实现时建议保留 metrics_endpoint 风格的语义级回归断言；如 gzhttp 语义不同（倾向压缩），需评估是否可接受。
