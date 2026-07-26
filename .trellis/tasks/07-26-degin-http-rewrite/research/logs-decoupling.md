# Research: logs 包 gin 解耦

- **Query**: logs 包 gin 耦合点全量核实、log_id 生命周期追踪、access/recovery 日志、context 化解耦改动清单、LogID→X-Log-Id 影响面、测试依赖
- **Scope**: internal
- **Date**: 2026-07-27

## Findings

### 1. logs 包 gin 耦合点（全量核实）

logs 包共 3 个文件：`logs/log_util.go`、`logs/logger.go`、`logs/init_test.go`。**gin 耦合只存在于 `log_util.go`，且与 PRD 提示的行号完全一致**：

| 位置 | 内容 | 说明 |
|---|---|---|
| `logs/log_util.go:15` | `import "github.com/gin-gonic/gin"` | 唯一 gin import |
| `logs/log_util.go:58-62` | `SetNewContextForGinContext(c *gin.Context)` | gin 适配函数 1：生成新 ctx → `c.Set("ctx", newCtx)` → 写 `LogID` 响应头（第 61 行） |
| `logs/log_util.go:98-108` | `GetContextFromGinContext(c *gin.Context)` | gin 适配函数 2：从 `c.Get("ctx")` 取回 ctx，取不到则 fallback `GenNewContext(c.Request.Context())` |

其余函数全部 gin-free，可原样保留：
- `Init` (`logs/log_util.go:29-56`)、`RandomHex` (:64)、`GenLogID` (:70)、`GenNewContext` (:74-79)、`GetLogIDFromContext` (:81-96)
- `logs/logger.go:12-17` `logIDHandler.Handle`：从 `context.Context` 取 log_id 附加 `slog.String("log_id", ...)`，纯 context，无 gin。
- context key 已是私有类型：`ctxKey int` + `logIDCtxKey` (`logs/log_util.go:19-21`)，解耦后无需改。

PRD 提示的 `log_util.go:102-112` 实际是 `98-108`（`GetContextFromGinContext` 函数体），行号有 4 行偏差，实质一致。

**全仓 gin 引用（Go 代码）**，即去 gin 的完整战场：

| 文件 | gin 引用 |
|---|---|
| `logs/log_util.go` | :15,58,98（本主题） |
| `handler.go` | :13 import；:42,48,57,63-64,67,76,84 handler 签名与 `c.JSON`/`gin.H` |
| `router.go` | :13-14 import（含 `gin-contrib/static`）；:49-56 中间件；:58-65 NoRoute JSON；:67-101 RegisterRoutes；:80 `static.Serve`；:186-199 `gzipResponseWriter` 嵌 `gin.ResponseWriter`；:201-221 gzipMiddleware |
| `main.go` | :19 import；:40 `gin.SetMode(runtimeConfig.GinMode)`；:97-98 `gin.New()` + `gin.Recovery()` |
| `handler_test.go` | :18 import；:22 `gin.SetMode(gin.TestMode)`；:66,106,148,183,224,273 `gin.New()`；:275 `router.GET(..., func(c *gin.Context))` |
| `metrics_endpoint_test.go` | :15 import；:37,43 `newMetricsTestRouter` 返回 `*gin.Engine` |
| `config/config.go` | :18-19 `GinModeKey`；:22 `DefaultGinMode`；:41 `RuntimeConfig.GinMode`；:71,81-83 解析与默认值；:98-100 校验（R3 范围） |
| `go.mod` | :6 `gin-contrib/static v1.1.6`；:7 `gin-gonic/gin v1.12.0`；:24 `gin-contrib/sse`(indirect) |

service/、utils/ 包 Go 代码零 gin 引用（`utils/http_test.go` 的 "origin" 与 `service/token_manager_login_metrics_test.go:250` 的 "login" 为 grep 误命中）。

### 2. log_id 完整生命周期

```
[生成] GenLogID (logs/log_util.go:70-72)：时间戳 "20060102150405" + 大写 hex(9 bytes)=18 字符
   ↑ 由 GenNewContext (logs/log_util.go:74-79) 调用，塞进 context.WithValue(parent, logIDCtxKey, ...)
[入口] apiLogContextMiddleware (router.go:49-56)：仅 isAPIPath (router.go:43-45，"/api" 或 "/api/" 前缀)
   → logs.SetNewContextForGinContext(c) (router.go:52)
       = GenNewContext(c.Request.Context()) → c.Set("ctx", newCtx)   ← 存 gin.Context 键值对，不是 r.WithContext
       + c.Writer.Header().Set("LogID", ...) (logs/log_util.go:61)   ← 响应头在中间件阶段就写死
[取用-handler] GetData (handler.go:42-44)：ctx := logs.GetContextFromGinContext(c)，随后 slog.InfoContext(ctx, "GetData")
[取用-404] writeAPINotFound (router.go:58-65)：同样 GetContextFromGinContext + GetLogIDFromContext
[入 service] GetTodayClassrooms(ctx) (handler.go:46)：service 层收到的就是标准 context.Context，后续全程 context 传递
   → 后台共享刷新 worker 用 context.WithoutCancel(ctx) 保留 log_id (service/refresh_coordinator.go:133-135)
[写日志] logIDHandler.Handle (logs/logger.go:12-17)：每条 slog 记录追加 log_id 字段
[错误信封] GetData 503 分支 (handler.go:47-55)："log_id": logs.GetLogIDFromContext(ctx) (handler.go:51)
[404 信封] router.go:63："log_id": logs.GetLogIDFromContext(ctx)
```

关键结构性事实：**log_id 只在 gin 边界（router.go/handler.go/logs 两个适配函数）依赖 gin.Context 键值存储；一旦进入 service 层即为纯 `context.Context`**。service 层对 logs 的引用仅 `GetLogIDFromContext`/`GenNewContext`（都在测试里：`service/refresh_context_test.go:30,47-48`）和 `logs.Init`（`service/testsupport_test.go:24`），生产代码 `service/refresh_coordinator.go` 只通过 `context.WithoutCancel` 隐式携带，不 import 特定函数。

### 3. access log 与 recovery 日志现状

- **没有 access log 中间件**。全仓无 `gin.Logger()`。唯一请求级日志是 handler 内手写的 `slog.InfoContext(ctx, "GetData")` (`handler.go:44`)。迁移后无需复刻 gin access log。
- **recovery**：`main.go:98` `r.Use(gin.Recovery())`。gin.Recovery 的输出走 gin 自己的 `DefaultErrorWriter`（stderr），**纯文本 + 栈，不经过 slog JSON、不带 log_id**。自写 recovery 中间件改用 `slog.ErrorContext(r.Context(), ...)` 反而是行为改进（panic 日志进 JSON 流水并带 log_id）。无任何测试断言 panic 时的日志格式或 500 响应体（grep 无 recovery/panic 相关 HTTP 测试），行为约束自由度大。
- 中间件顺序现状：`gin.Recovery()` 在 `main.go:98` 最先 Use（最外层），`gzipMiddleware` 在 `RegisterRoutes` 内第一个 Use (`router.go:68`)，即现在是 Recovery 包住 gzip。PRD R1 要求新架构反过来：recovery 在最内层，避免 panic 后向已 Close 的 gzip writer 写入——这是行为设计变更点，不是现状复刻。

### 4. 解耦方案改动清单（context.Context + r.WithContext）

删除 `SetNewContextForGinContext` 与 `GetContextFromGinContext` 后，各调用点改动：

| 调用点 | 现状 | 改动 |
|---|---|---|
| `router.go:49-56` apiLogContextMiddleware | `c.Set("ctx", ...)` + 写 `LogID` 头 | 标准中间件：`ctx := logs.GenNewContext(r.Context()); w.Header().Set("X-Log-Id", logs.GetLogIDFromContext(ctx)); next.ServeHTTP(w, r.WithContext(ctx))`。可继续放在 logs 包（`func WithNewLogID(r *http.Request) *http.Request` 之类），或直接内联在 httpapi 层——logs 包只需保留 GenNewContext/GetLogIDFromContext 即可 |
| `router.go:58-65` writeAPINotFound | `GetContextFromGinContext(c)` | `r.Context()` 直取（中间件已 WithContext）+ `GetLogIDFromContext` |
| `handler.go:43` GetData | `GetContextFromGinContext(c)` | `ctx := r.Context()` |
| `handler.go:51` 错误信封 | 不变 | `GetLogIDFromContext(ctx)` 原样可用 |
| `service/*` | 已是纯 context | **零改动** |
| `logs/logger.go` | 纯 context | **零改动** |

注意语义差异：现状 `GetContextFromGinContext` 在中间件未跑过时有 fallback（`logs/log_util.go:100,107` 生成新 ctx，导致该 log_id 不在响应头里）。`handler_test.go:236-237` 断言 SPA fallback **不得**带 LogID 头——新中间件必须保持"仅 /api 路径注入"的条件（`isAPIPath`, router.go:43-45），或用 ServeMux 模式天然限定 `/api/` 前缀。

### 5. 响应头 LogID→X-Log-Id 影响面

- **frontend/**：grep `logid/log_id/LogID`（大小写不敏感）零命中。前端不读该响应头，也不读错误信封 log_id 字段（信封字段本次不改名，仅头改名）。**零影响**。
- **nginx（scripts/install.sh:693-757 render_nginx_site）**：纯 `proxy_pass` + `proxy_set_header`（只设请求头 Host/X-Real-IP/X-Forwarded-*），无 `proxy_hide_header`/`add_header`，响应头透传，头名无关。`scripts/install_test.sh` 也无 LogID 断言。**零影响**。
- **docs**：`docs/operations.md:130`（"in the `LogID` response header"）需改为 `X-Log-Id`；`docs/operations.md:125-139` log_id 追踪章节其余内容仍成立。`README.md:71` 只提 `log_id` 概念，不提头名，可不动。
- **CHANGELOG.md:143** 历史条目提及 `LogID` header（历史记录不改），新条目需按 PRD R3/AC 记录 `LogID→X-Log-Id`。
- **spec 文档**（主 agent 用 update-spec 处理，research 不改）：
  - `.trellis/spec/backend/logging-guidelines.md:14,39,44-45,54,138` — 引用两个 gin 适配函数与 LogID 头
  - `.trellis/spec/backend/error-handling.md:84` — 引用 GetLogIDFromContext（仍有效）
  - `.trellis/spec/backend/directory-structure.md:41` — 引用 SetNewContextForGinContext
- **测试断言**：`handler_test.go:211-216,236-237,258-263` 直接 `Header().Get("LogID")`，改名后需同步为 `X-Log-Id`（注意 `http.Header.Get` 走 CanonicalMIMEHeaderKey，`X-Log-Id`/`X-Log-ID` 规范化后为 `X-Log-Id`，写法建议统一 `X-Log-Id`）。

### 6. 现有测试对 gin 的依赖点

- **`logs/init_test.go`：零 gin 依赖**。仅测 `Init` 的文件目录分支（:19-23, :25-42），删除两个 gin 适配函数后 logs 包测试无需改动；两个函数**没有任何直接单测**，其行为只被根包 handler_test.go 间接覆盖。
- 根包测试（迁移时需重写装配，AC 允许）：
  - `handler_test.go:22` `gin.SetMode(gin.TestMode)`；:66,106,148,183,224 用 `gin.New()` 装配路由（其中 :148,183,224 走 `RegisterRoutes` 全栈，覆盖 log_id 信封/头/SPA fallback）；:273-277 gzip 测试用 gin handler。
  - `metrics_endpoint_test.go:37-49` `newMetricsTestRouter` 返回 `*gin.Engine`，手工挂 gzipMiddleware + 3 条路由。
- `service/refresh_context_test.go:13-78` 只用 `logs.GenNewContext`/`GetLogIDFromContext`，gin-free，不受影响。
- `service/testsupport_test.go:24` `logs.Init(false, false)`，gin-free。

## Caveats / Not Found

- 未发现任何读取 `LogID` 响应头的消费方（前端、脚本、文档示例均不依赖头名），改名风险极低；唯一用户可见面是浏览器 DevTools/curl 手动排障，已由 PRD 要求记入 CHANGELOG。
- gin.Recovery 的日志走 stderr 纯文本这一现状没有测试锁定；自写 recovery 的日志格式可自由设计，但建议带 log_id（当前做不到，是顺带改进）。
- `RandomHex` (`logs/log_util.go:64-68`) 目前仅被 `GenLogID` 使用（全仓无其他调用），解耦时可考虑非导出，但非本任务必需。
