# 后端架构审计结论（2026-07-26）

来源：全量阅读后端 Go 源码的架构审计。总评：工程质量高于平均，问题是"为『每天缓存一份 JSON、两个校区』的服务配置了企业级抽象与依赖"。

## 核心发现

### 去 Gin（单项收益最高）
- 实际只有 4 条路由（router.go:71-78）+ `c.JSON` + Recovery，中间件全部手写。
- `go mod why` 证实 gin 拖入 67 个无关传递模块：quic-go/http3、mongo-driver/bson、sonic+golang-asm、go-yaml、validator 全家桶、ugorji/codec。CHANGELOG 0.1.5 有"因 quic-go 可达被迫升级"的直接运维成本记录。
- 替换映射：`mux.HandleFunc("GET /api/get_data", h)`（Go 1.22+ ServeMux）；`c.JSON` → 6 行 writeJSON；`gin.Recovery()` → 15 行中间件；`static.Serve`+EmbedFolder（router.go:20-41）→ `http.FileServerFS`；gin.Keys 传 ctx → `r.WithContext`（顺带解除 logs→gin 依赖）；`GIN_MODE` 配置项删除（config/config.go:19, 81-83, 98-100）。
- 模块数 77 → ~10。

### 删 cache/ 包
- cache/cache.go:1-51 为**单个 key** 引入停更 8 年的 patrickmn/go-cache（+incompatible，janitor goroutine）。
- TTL 与业务自身过期机制冗余：Date/ExpiresAt/StaleUntil/Stale + go-cache TTL 共 5 套重叠（model/realtime_data.go:68-77）。StaleUntil 恒等于 endOfDay(Date)，realtime_data.go:57 的 `now.Before(cached.StaleUntil)` 生产路径恒真，其后分支是死代码。
- 替代：`atomic.Pointer[model.TodayClassrooms]` + 单一 `freshness(cached, now) (fresh|stale|expired)` 函数。GetTodayClassrooms 从 50 行嵌套分支降到 ~20 行。
- 同时消除 cache→service/model 倒置依赖（cache/cache.go:6）。

### 手写 gzip 中间件的协议问题（router.go:186-221）
1. identity 响应缺 `Vary: Accept-Encoding`（:216 只在压缩时设置）——共享缓存污染风险。
2. 无条件压缩：png/woff2/jpg 二次压缩、小 JSON 也压。需 Content-Type 白名单 + 长度阈值。
3. 与 Range 请求冲突（static.Serve 底层 http.FileServer 支持 Range，:192,198,217 删 Content-Length）。
4. 204/304/空体也写 gzip footer（:213 无条件 defer gz.Close()）；Recovery 在 gzip 外层（main.go:95），panic 路径写坏响应。
- 方案：换 klauspost/compress/gzhttp，或把压缩全部下放给前置 Nginx（部署本来就是 Nginx 反代），净删 ~120 行（含 acceptsGzip 66 行 q-value 解析）。

### 生命周期
- O1: refresh_coordinator.go:141 `context.WithoutCancel` 剥离了应用生命周期取消 → SIGTERM 最坏等 50s（gracefulShutdownTimeout）。修法：service 持有生命周期 ctx，worker ctx 用 `context.WithoutCancel(reqCtx)` + `context.AfterFunc(lifecycleCtx, cancel)`。token_manager.go:297-299 同样问题。
- O2: 生命周期靠 StartWarmup 侧门注入（warmup.go:93-116），5 个字段（backgroundMu/backgroundStopping/warmupStarted/warmupCancel/warmupDone）→ 改 `Run(ctx)/Shutdown(ctx)` 标准形态。
- O3: 冷启动请求阻塞最长 30s（realtime_data.go:80-92），迫使全局 WriteTimeout=45s（main.go:80）→ 有界等待 5s 后 503+Retry-After。
- O7: main.go:89,146 `log.Fatalf` 跳过 defer 与日志刷盘；main.go 用标准 log 而非 slog → 改 `run() error` 形态。

### 重复/死代码清理
- S3: refresh_coordinator.go 手写 singleflight（refreshInFlight/refreshAttempt/done chan，~60 行），而 token_manager.go:76-86 已在用 x/sync/singleflight。统一用 `singleflight.Group.DoChan`，保留 300ms stale 等待语义。
- S4: errgroupNoCancel（realtime_data.go:302-317）= 手写 WaitGroup 包装；Go 1.22+ 循环变量重绑定（realtime_data.go:188 等）可删。
- S5: 三份相同 reflect 判空：handler.go:29-40、service/dependency.go:5-16、utils/http.go:109-120 → 删。
- S6: forceRefresh/loginPerformed/EnsureToken 的 for 重试环（token_manager.go:60-99）唯一调用链是测试专用 Login → 删。
- S7: 死代码：model.QueryResponse、logs.LogIDKey、jw_error.go:66-69 ctx 死分支；QueryOne/QueryAll/Login 仅测试可达。
- S9: 两套相同退避阶梯（refresh_coordinator.go:41-46 与 warmup.go:30-41）→ 统一 backoffLadder。
- S11: NoopMetrics 已存在但 20+ 处 `!= nil` 判空 → 构造函数默认注入 Noop，删 observeXxx 包装。
- S12: now() 判空、runtime_status.go:89-92 `copy := t` 遮蔽内建 + cloneTime（改 time.Time + omitzero）、addQuery 单调用点内联、hasJWCredentials 闭包改 bool。

### 性能
- O5: router.go:82-100 每次 SPA 404 都 f.Open+io.ReadAll index.html → 启动时读一次 + ETag + http.ServeContent。
- O6: /api/get_data 全用户共享 payload，每请求重新 Marshal+gzip，无 ETag/Cache-Control → 刷新时预序列化（可预压缩），带 ETag/304。注意 classroomResponse（realtime_data.go:344-357）按场景改写 Stale/Error，需按 (stale, errKind) 组合缓存。

### 包结构
- P1: cache→service/model 倒置（随删 cache 消解）。
- P2: logs→gin 依赖（log_util.go:15,62-66,102-112）→ logs 只留框架无关函数。
- P3: utils/ 杂物箱：单使用者（jw_client）、HttpGet 命名违例、ctx 非首参、返回 header 全被丢弃、query 参数死参 → 并入 service。
- P4: ClassroomService 4 组状态 + 4 锁，startClassroomRefresh 嵌套持锁 backgroundMu→refreshMu → 拆 refreshCoordinator/backgroundRunner。
- P5: 根目录平铺 main/router/handler + 5 个测试文件 → 可移 internal/httpapi/；config.Campuses 硬编码是领域常量应移 service。

### 测试债（重构前置）
- O8: 白盒测试直接读写未导出字段（refresh_backoff_test.go:101-107 等、warmup_test.go:92 无同步写 warmupJitter）→ 先抽 export_test.go 种子函数（seedCache/forceFailure），warmupJitter 改 Options 字段。
- O9: realtime_data_test.go 1438 行混单测与需凭据集成测试 → 拆分 + `//go:build integration`。
- O10: JW 超时双层设置（jw_client.go:41-42,63-64,108-109 与 token_manager.go:297-299）→ 超时归调用方。

### 其他
- O4/S10: jw_error.go:91-184 Unicode 归一化与 slog JSON 转义重叠，保留脱敏+截断即可（若删需确认日志无非 JSON 消费者）。
- O11: /readyz 公网暴露诊断详情 → 公网只回状态码。
- O12: 响应头 `LogID` 会被规范化成 `Logid` → 改 `X-Log-Id`；错误信封 code 语义域混用。

## 建议执行顺序（原审计）
1. gzip 修复或下放 Nginx（−120 行，低风险）
2. 删 cache 包 → atomic.Pointer（−80 行 −1 依赖，低风险）
3. Noop 默认 + 死代码 + errgroupNoCancel + 退避合并（−150 行，极低风险）
4. 生命周期 Run/Shutdown + AfterFunc（结构改善，中风险）
5. 去 Gin（−70 行 −67 模块，中风险）
6. 过期模型收敛 + singleflight（−100 行，中高风险，需先做 O8 测试收敛）
7. utils 并入 service、internal/httpapi、预序列化+ETag（中风险）
