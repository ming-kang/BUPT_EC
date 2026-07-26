# Research: 手写 gzip 行为矩阵与 gzhttp 对照

- **Query**: 手写 gzipMiddleware/acceptsGzip 完整行为矩阵；gzip 测试断言分级；klauspost/compress gzhttp 选项与默认行为；迁移映射表与 NewWrapper 配置草案
- **Scope**: mixed（内部 router.go/handler_test/metrics_endpoint_test + 外部 gzhttp v1.19.1 源码）
- **Date**: 2026-07-27

## 1. 手写实现行为矩阵（现状）

代码位置：`router.go:106-221`（`acceptsGzip` + `appendVaryAcceptEncoding` + `gzipResponseWriter` + `gzipMiddleware`）。

| # | 维度 | 现状行为 | 证据 |
|---|---|---|---|
| M1 | 触发条件 | 仅看请求头 `Accept-Encoding`，`acceptsGzip` 返回 true 即包 gzip writer；**与响应内容无关** | router.go:207-212 |
| M2 | AE 解析：大小写 | coding 与 q 参数键均大小写不敏感 | router.go:127, 143 |
| M3 | AE 解析：q 值 | 完整解析 q；`q=0` 拒绝；q 解析失败或超出 [0,1]（如 `q=abc`、`q=1.5`）**按 q=0 拒绝**；无 `=` 的参数也按 q=0 拒绝 | router.go:132-152, 167-170 |
| M4 | AE 解析：`*` 通配 | `*` 允许 gzip，但**仅当没有显式 gzip token 时**才生效（显式 `gzip;q=0` 压过 `*;q=1`） | router.go:158-170（注释 103-105） |
| M5 | AE 解析：空白 | token 与 `q = 0.8` 这种 `=` 两侧带空格的写法都容忍（TrimSpace 后解析） | router.go:121, 138-146 |
| M6 | Content-Type 白名单 | **没有**。任何 Content-Type（含 png/woff2/octet-stream）一律压缩——这是 O5 缺陷之一 | router.go:201-221（无任何 CT 检查） |
| M7 | 最小长度阈值 | **没有**。1 字节响应也压缩（gzip 空流/短流反而膨胀） | 同上（无长度检查） |
| M8 | Vary 头 | 仅在**决定压缩后**追加 `Vary: Accept-Encoding`；对既有 Vary 值做大小写不敏感去重（逗号拆分逐项比对） | router.go:173-184, 216 |
| M9 | Content-Length | 包装时 Del 一次，之后每次 `Write`/`WriteString` 再 Del（防止 handler 中途设置） | router.go:192, 197, 217 |
| M10 | WriteHeader 交互 | **不拦截** WriteHeader；`Content-Encoding: gzip` 在 handler 运行前就写入 header（router.go:215），随首次 WriteHeader 一起发出。即使响应最终为空/204/错误页，`defer gz.Close()` 也会输出 ~23 字节 gzip 空流帧——204/304 语义被破坏（现网无此路由触发，属潜在缺陷） | router.go:212-219 |
| M11 | Flush 交互 | `gzipResponseWriter` **未实现** Flush 转发：gin 的 Flush 只刷底层 writer，gzip.Writer 缓冲不会被刷出，流式响应会滞留到 Close（现网无流式端点，未暴露） | router.go:186-199（无 Flush 方法） |
| M12 | 已压缩内容跳过 | **没有**检查响应 `Content-Encoding`/`Content-Range`。若内层 handler 自己压缩会产生双重 gzip——这正是 /metrics 必须配 `DisableCompression: true` 的原因 | router.go:201-221；main.go:62-66 |
| M13 | HEAD 请求 | 不区分方法，HEAD 同样被包 gzip（Content-Encoding 头照设） | router.go:202-219（无方法判断） |
| M14 | Range/206 | 不识别请求 Range 或响应 Content-Range/Accept-Ranges；静态文件 206 partial 响应体会被 gzip，而 Content-Range 字节数指的是未压缩体——语义损坏（潜在缺陷） | router.go:201-221（无相关检查） |
| M15 | /healthz /readyz 跳过 | 中间件内部**按路径硬编码**提前 return | router.go:203-206 |
| M16 | /metrics | 不跳过，由本中间件做外层压缩；内层 promhttp `DisableCompression: true` 保证单层（"router gzipMiddleware is the sole Accept-Encoding owner"） | router.go:73；main.go:62-66；handler.go:84-90 |
| M17 | 压缩级别 | `gzip.NewWriter` = 标准库 DefaultCompression | router.go:212 |
| M18 | 安装位置 | `r.Use(gzipMiddleware())` 全局第一个中间件，先于 apiLogContextMiddleware | router.go:68-69 |

## 2. gzip 相关测试断言清单

### router_test.go — TestAcceptsGzip（router_test.go:5-35）

对内部函数 `acceptsGzip` 的纯单元测试，15 个 case（router_test.go:11-25）。**整体为实现级**：函数删除后此测试应整体删除（gzhttp 自带解析）。其中语义会随 gzhttp 变化的 case：

| Case（行号） | 手写 | gzhttp | 定级 |
|---|---|---|---|
| `gzip`/`GZIP`/`gzip;q=1.0`（13,14,18） | 压缩 | 压缩 | 契约语义（由下方集成测试覆盖） |
| `gzip;q=0` 系列（15,16,17） | 不压缩 | 不压缩 | 契约语义（集成测试覆盖） |
| `*;q=1` / `*;q=0` / `gzip;q=0,*;q=1`（19,20,21） | `*` 生效 | **gzhttp 不识别 `*`，一律不压缩** | 实现级，行为将变（见 §4-D6） |
| `gzip;q=abc`（23） | 拒绝 | 拒绝（ParseFloat 失败 → q=0） | 实现级，恰好一致 |
| `gzip;q=1.5`（24） | 拒绝 | **钳制为 1.0 → 压缩** | 实现级，行为将变（可接受） |
| `  gzip ; q = 0.8  `（25） | q=0.8 压缩 | `q =` 不匹配 `q=` 前缀 → 按 q=1 压缩 | 实现级；注意反向 case `gzip; q = 0` 会从"不压缩"变"压缩"（畸形头，可接受） |

### handler_test.go — TestGzipMiddlewareCompressesAPIAndSkipsHealthz（handler_test.go:271-321）

| 断言（行号） | 语义 | 定级 |
|---|---|---|
| 284-286：`/api/test` + AE:gzip → `Content-Encoding: gzip` | API 可压缩响应必须压缩 | **契约级**。但注意 fixture 体积 128B（handler_test.go:276）< gzhttp 默认 MinSize 1024，**必须把测试体加大到 ≥1KB**（或调 MinSize），否则语义级断言直接失败 |
| 287-289：Vary 含 Accept-Encoding | 压缩响应必带 Vary | **契约级**（验收标准明列） |
| 290-301：解压后体与原文一致 | 单层 gzip、内容无损 | **契约级** |
| 303-312：`gzip;q=0` → 无 CE、identity 体 | q=0 协商 | **契约级** |
| 314-320：/healthz + AE:gzip → 无 CE | healthz 不压缩 | **契约级**（跳过手段从"中间件内判路径"变为"该路由不包 wrapper"，属实现级装配调整） |

### metrics_endpoint_test.go

| 断言（行号） | 语义 | 定级 |
|---|---|---|
| 61-64：无 AE → 无 CE + 可解析 Prometheus 文本 | identity 路径 | **契约级** |
| 69-74：AE:gzip → CE=gzip + Vary 含 Accept-Encoding | /metrics 外层压缩 | **契约级** |
| 76-87：wire 体为 gzip 魔数、**解压一次后不再是 gzip**、可解析出 metric family | 单层压缩（O5 核心回归护栏） | **契约级，最重要** |
| 90-96：`gzip;q=0`、`identity, gzip;q=0` → 无 CE | q=0 协商 | **契约级** |
| 99-108：AE:`*` → 允许压或不压，压则单层 | 通配容忍写法 | **契约级**（写法已兼容 gzhttp 的"`*` 不压"行为，无需改） |
| 111-124：healthz/readyz + AE:gzip → 无 CE | 健康端点不压缩 | **契约级** |
| 126-133：metricsHandler 为 nil → 404 | 与 gzip 无关 | 契约级（保持） |
| 31-35：测试用 promhttp `DisableCompression: true` 与 main.go 对齐 | 装配约定 | **契约级**（注释明言 "tests must keep them aligned"） |

装配代码（metrics_endpoint_test.go:37-49 用 gin.New + gzipMiddleware()）全部是**实现级**，随 ServeMux 重写。

主观注意：/metrics gzip case 依赖 exposition 文本 ≥ MinSize 才触发压缩。测试观察了 login/refresh/cache 三组直方图族（metrics_endpoint_test.go:52-56），exposition 通常远超 1KB，但实现时应实测确认，不够就再多 observe 或调 MinSize。

## 3. gzhttp 外部调研（klauspost/compress v1.19.1）

- 最新稳定版：**v1.19.1**（2026-07-20 发布，proxy.golang.org @latest 确认）。纯 Go、零第三方依赖，只新增 1 个模块。
- 关键版本事实：**v1.19.0 起 gzhttp 加入 zstd 支持且默认启用**（v1.18.0 的 compress.go 无 zstd 协商代码，v1.19.0 有）。README：客户端 gzip/zstd 都接受且 q 相同时**优先 zstd**。
- 包为 nytimes/gziphandler 的活跃 fork。

### NewWrapper 选项与默认值（gzhttp/compress.go@v1.19.1，行号为该文件）

| 选项 | 默认 | 说明 |
|---|---|---|
| `MinSize(n)` | **1024**（`DefaultMinSize`，compress.go:78-84） | 响应体缓冲到 minSize 才启动压缩；不足则原样输出 |
| `CompressionLevel(n)` | `gzip.DefaultCompression`（klauspost gzip，compress.go:582） | 与标准库默认级别对等 |
| `ContentTypes([]string)` / `ExceptContentTypes([]string)` / `ContentTypeFilter(fn)` | `DefaultContentTypeFilter`（compress.go:588, 1186-1204） | 默认排除：`video/*`、`audio/*`、`image/jp*` 前缀，及 CT 中含 `compress/zip/snappy/lzma/xz/zstd/brotli/stuffit` 字样。**注意：image/png、font/woff2、application/octet-stream 不在默认排除内，默认仍会被压缩** |
| `SetContentType(b)` | true（compress.go:589） | CT 缺失时用 `http.DetectContentType` 嗅探补齐 |
| `EnableZstd(b)` / `EnableGzip(b)` | 均 true（compress.go:592, 599） | |
| `PreferZstd(b)` | true（compress.go:598） | q 相同时选 zstd |
| `ZstdCompressionLevel(n)` | SpeedFastest=1（compress.go:593） | |
| `KeepAcceptRanges()` | false（默认压缩时**删除 Accept-Ranges**，compress.go:246-248） | |
| `SuffixETag(s)` / `DropETag()` | 都不启用：**压缩响应默认保留原 ETag 不变**（compress.go:250-275） | |
| `RandomJitter(...)` | 关闭 | BREACH 缓解，本项目无用户注入内容，不需要 |
| `AllowCompressedRequests(b)` | false | RFC 7694 压缩请求体，不需要 |

### 默认行为要点

| 维度 | gzhttp 行为 | 证据（compress.go） |
|---|---|---|
| Vary | wrapper 入口**无条件** `Header().Add("Vary", "Accept-Encoding")`——identity 响应、q=0 客户端也带；用 Add 不去重（本项目无 handler 自设 Vary，无叠加风险） | :612 |
| AE 解析 | 只匹配字面 `gzip`/`zstd` token；**不支持 `*` 通配**；q 缺省 1.0；q<0 钳 0、q>1 钳 1；q 解析失败 → 0 | :1052-1069, 1143-1177 |
| HEAD | **一律不压缩**（nginx bug 规避） | :1004-1008 |
| Content-Length | 判定用：CT 通过且 `cl >= minSize` 可提前触发压缩；压缩启动时 Del Content-Length | :168-177, 240-243 |
| 已有 Content-Encoding | 响应已设 CE → 跳过压缩原样透传（天然防双重压缩） | :166, 447 |
| Range | 响应有 `Content-Range` → 不压缩；压缩时删 `Accept-Ranges`（除非 KeepAcceptRanges）。与 `http.FileServer` 的 206 语义正确配合 | :166, 246-248, 433-447 |
| WriteHeader | 缓存 status code，直到决定压缩/直通才真正下发（1xx 直通）；204/304 无体 → 不产生 gzip 空流帧 | :385-398, 412-422, 447-449 |
| Flush | 实现 http.Flusher：先刷压缩 writer 再刷底层；未达 minSize 判定点时按"已达"处理；未写过则不刷 | :495-547 |
| 空响应 | 从未 Write → Close 走 startPlain，不输出任何压缩帧 | :425-455, 447 |
| 逃生门 | 响应头设 `No-Gzip-Compression`（`HeaderNoCompression`）可按请求禁压，输出前自动删除该头 | :54-58, 166, 530 |

## 4. 映射表：现状 → gzhttp

| # | 现状行为（§1 编号） | gzhttp 对应 | 结论 |
|---|---|---|---|
| D1 | M1/M6/M7 无 CT 白名单、无阈值，全量压缩 | 默认 MinSize=1024 + DefaultContentTypeFilter | **行为改变（即修复）**：小响应、音视频不再压缩。但默认过滤器**不排除 png/woff2**，验收标准"png/woff2 不二次压缩"要求**显式配置 ContentTypes 白名单**（见配置草案）。另：Windows 下 `mime.TypeByExtension(".woff2")` 不保证返回 `font/woff2`，可能落到 octet-stream——白名单方式对两种结果都安全（都不压） |
| D2 | M2/M3 q 值解析 | 语义基本一致；差异：`q=1.5` 手写拒绝 → gzhttp 钳 1.0 压缩；`q = 0`（等号带空格）手写拒绝 → gzhttp 按 q=1 压缩 | **行为改变，可接受**（均为畸形头边角；RFC 客户端不受影响；对应 router_test case 随函数删除） |
| D3 | M4 `*` 通配允许 gzip | gzhttp 不识别 `*`，不压缩 | **行为改变，可接受**：metrics_endpoint_test.go:99-108 已写成两可容忍；真实客户端极少只发 `*` |
| D4 | M8 Vary 仅压缩时加且去重 | 所有经 wrapper 的响应（含 identity）都加 Vary | **行为改变（更正确）**：缓存语义上 identity 变体也应带 Vary。现有测试只断言压缩场景含 Vary，全部兼容 |
| D5 | M9 Content-Length 删除 | 压缩时删除，等价 | 默认即满足 |
| D6 | M10 空体/204 输出 gzip 空流帧、错误响应必压 | 缓冲式判定：空体不压；<1KB 错误 JSON 不压 | **行为改变（即修复）**。现测试的错误路径请求都不带 AE 头，无断言冲突 |
| D7 | M11 Flush 缺陷 | 完整 Flusher 支持 | 默认即满足（修复潜在缺陷） |
| D8 | M12 无已压缩跳过 | 响应已有 CE 即跳过 | 默认即满足。**promhttp `DisableCompression: true` 仍保留**（双保险 + 测试契约 metrics_endpoint_test.go:31-35），只需更新 main.go:62-66 注释中 "router gzipMiddleware" 的措辞 |
| D9 | M13 HEAD 也压 | HEAD 一律不压 | **行为改变（更正确）**，无测试覆盖 HEAD |
| D10 | M14 Range/206 会被错误压缩 | Content-Range 响应直通、压缩时删 Accept-Ranges | 默认即满足（修复缺陷）；R5 静态文件走 `http.FileServerFS` 后此点重要 |
| D11 | M15 healthz/readyz 中间件内判路径 | gzhttp 无路径概念 | **接线改变**：这两条路由注册时**不套 wrapper**（见下）。副作用：healthz/readyz 响应连 Vary 也没有，与现状完全一致 |
| D12 | M17 压缩级别 stdlib 默认 | klauspost gzip DefaultCompression（默认值） | 等价（klauspost 同级别更快） |
| D13 | （新）zstd 默认启用 | 浏览器普遍发 `gzip, deflate, br, zstd`，前端资源将变 zstd 编码 | **行为改变，建议本任务先 `EnableZstd(false)`**：PRD 决策是"gzip 换 gzhttp"，验收措辞全部围绕 gzip；保持行为面最小，zstd 可作后续独立开关（一行改动）。若保留默认开启，需在 CHANGELOG 说明且确认无老旧代理问题 |
| D14 | （新）ETag 交互 | 压缩响应默认保留原 ETag——同一 ETag 对应两种字节流，违反 RFC 7232 精神 | R5 要给 index.html 加 ETag：若走 gzip，需决定 `SuffixETag("-gzip")`（则 If-None-Match 比对逻辑要容忍后缀）或 DropETag 或维持现状。**留给 design.md 决策**，见风险 |

### 建议的 NewWrapper 配置草案

```go
import (
    "github.com/klauspost/compress/gzhttp"
    "github.com/klauspost/compress/gzip" // 仅当需要显式级别常量
)

// newGzipWrapper 返回可复用的压缩包装器。
// 契约：可压缩类型白名单 + 1KB 阈值 + 单层压缩（内层已设 CE 时自动跳过）。
func newGzipWrapper() (func(http.Handler) http.HandlerFunc, error) {
    return gzhttp.NewWrapper(
        // 迁移期保持 gzip-only 行为面；后续要开 zstd 删掉这一行即可。
        gzhttp.EnableZstd(false),
        // 显式白名单：覆盖 API JSON、Prometheus 文本、SPA 静态资源中真正可压缩的类型。
        // 不含 image/png、font/woff2、application/octet-stream —— 满足"png/woff2 不二次压缩"。
        gzhttp.ContentTypes([]string{
            "application/json",
            "text/html",
            "text/plain", // /metrics exposition: text/plain; version=0.0.4（无参数条目匹配任意参数）
            "text/css",
            "text/javascript",
            "application/javascript",
            "image/svg+xml",
            "application/xml",
            "application/wasm",
        }),
        // 默认即 1024，写出来当文档；如需照顾小 JSON 响应可下调，但别为迁就旧测试改产品配置。
        gzhttp.MinSize(gzhttp.DefaultMinSize),
    )
}
```

级别不必显式设置（默认 `gzip.DefaultCompression` 与现状等价）。`ContentTypes` 的无参数条目会匹配任意带参数变体（`text/plain` 匹配 `text/plain; version=0.0.4; charset=utf-8`，compress.go:841-873）。

### 新架构接线方式（ServeMux）

```go
wrap, _ := newGzipWrapper()

mux := http.NewServeMux()
// healthz/readyz：不套 wrapper —— 替代现在 router.go:203-206 的路径判断。
mux.HandleFunc("GET /healthz", s.Healthz)
mux.HandleFunc("GET /readyz", s.Readyz)
// /metrics：外层 gzhttp 压缩 + 内层 promhttp DisableCompression:true（main.go:64-66 不变，仅改注释）。
mux.Handle("GET /metrics", wrap(http.HandlerFunc(s.Metrics)))
// API 与静态/SPA fallback 都套 wrapper。
mux.Handle("GET /api/get_data", wrap(apiLogCtx(http.HandlerFunc(s.GetData))))
mux.Handle("/", wrap(spaAndStaticHandler))
```

按 PRD R1，recovery 中间件应在 wrapper **内侧**（`wrap(recover(h))`），使 panic 在 gzhttp 的 `defer gw.Close()`（compress.go:642-646）执行前被拦截，避免向已关闭的压缩 writer 写入。

## Caveats / Not Found

- 未运行任何 go 命令验证（本仓库尚未引入 klauspost/compress）；gzhttp 结论全部来自 v1.19.1 源码静读（gzhttp/compress.go）与 README。
- mcp deepwiki 工具在本环境不可用，改用源码直读，可信度更高。
- 未核实 /metrics exposition 实际字节数是否恒 ≥1024（三组直方图理论上远超，但需实现时实测）。
- `.trellis/spec/` 下未检索 gzip 相关既有规范条目（本文件聚焦行为矩阵；spec 沉淀由主 agent 决策）。
