# 后端审计结论（2026-08-07）

对照基线：`archive/2026-07/07-26-arch-simplify-refactor/research/audit-backend.md`。  
证据来源：当前 `main` 的 `service/`、根目录 HTTP 层、`go.mod`、`.trellis/spec/backend/*`。  
总评：批次①②③已把「企业级依赖堆叠」压到 stdlib + 7 个直接依赖；剩余债集中在**生命周期取消传播**、**手写刷新协调 vs singleflight 双轨**、**API 热路径重复序列化**，以及少量包边界/测试体积项。过期字段对外契约已稳定，不宜再做破坏性收敛。

## 07-26 对照总表（主要条目）

| 07-26 条目 | 现状 | 本轮状态 |
|---|---|---|
| 去 Gin | `http.ServeMux` + `writeJSON`；`go.mod` 无 gin | **已落地** |
| 删 cache/ + go-cache | `todayCache atomic.Pointer`；无 cache 包 | **已落地** |
| 手写 gzip → gzhttp | `router.go` 用 klauspost/gzhttp；探针旁路 | **已落地** |
| SPA index 一次读取 + ETag | `router.go:163-180` | **已落地** |
| `/assets` immutable Cache-Control | `router.go:143-156` | **已落地** |
| X-Log-Id | `router.go:94` | **已落地** |
| 版本注入 + `/readyz.version` | `main.go:27`、`handler.go:78` | **已落地** |
| 退避阶梯合并 | `refresh_coordinator.go:backoffLadder` 与 warmup 共用 | **已落地** |
| NoopMetrics 默认 | `ClassroomServiceOptions.Metrics` 可空 | **已落地** |
| export_test 白盒缝 | `service/export_test.go` | **已落地** |
| errgroupNoCancel / 死 Login 环等 | 源码已无对应实现 | **已落地** |
| 过期模型收敛（五套 TTL） | go-cache TTL 已消；对外仍 Date/ExpiresAt/StaleUntil/Stale | **仍有效（收窄）** |
| refresh → x/sync/singleflight | 仍手写 `refreshInFlight`/`classroomRefreshAttempt` | **仍有效** |
| Run/Shutdown + AfterFunc | 仍 `StartWarmup`/`WaitBackground`；worker `WithoutCancel` | **仍有效** |
| 冷启动有界等待 5s+503 | 冷路径仍可阻塞至 `ClassroomRefreshLimit` 30s | **仍有效** |
| `/api/get_data` 预序列化+ETag | 每请求 `json.Marshal` | **仍有效** |
| `/readyz` 公网诊断详情 | 仍返回完整 `runtime` | **仍有效** |
| utils 并入 service | `utils/http.go` 仍独立 | **仍有效** |
| 根目录 → internal/httpapi | 根平铺 `main/router/handler` | **仍有效（低优先）** |
| realtime_data_test 拆分 | 约 511 行（已缩小）但仍偏大 | **仍有效（收窄）** |
| gin 传递依赖 CVE 面 | module graph ~40 | **已过时**（问题已消除） |

## 批次④遗留（后端相关）判定

| 项 | 判定 | 依据 |
|---|---|---|
| 缓存新鲜度模型收敛 | **降级** | 对外 JSON 字段已是 API 契约与前端 SWR/`reloadSchedule` 依赖；再删字段属破坏性。仅建议内部抽 `freshness` 枚举，不改 wire format。 |
| refresh_coordinator → singleflight | **降级** | 协调器已带 partial/total backoff、jitter、300ms stale wait、`backgroundStopping` 联锁；`export_test` 锁定行为。换成 `singleflight.Group` 省行数有限、回归面大。与生命周期重构同开时可再评估。 |
| Lifecycle Run/Shutdown + cancel | **继续做** | `WithoutCancel` 使 SIGTERM 仍可能等到刷新预算（与 `gracefulShutdownTimeout` 50s 绑定）；API 形态仍是侧门 `StartWarmup`。 |

---

## 发现清单

### B-01 · 刷新 worker / token 操作剥离生命周期取消

- **状态相对 07-26**：仍有效（原 O1）
- **类别**：可靠性
- **证据**：
  - `service/refresh_coordinator.go:133-139`：`context.WithoutCancel` + 独立 30s timeout
  - `service/token_manager.go:272-274`：`sharedOperationContext` 同样 `WithoutCancel`
  - `main.go:86-88,140-148`：`gracefulShutdownTimeout = 50s`；先 `stopBackground()` 再 `Shutdown`/`WaitBackground`
- **建议**：service 持有 lifecycle ctx；worker 用 `WithoutCancel(reqCtx)` 保留 log_id，再 `context.AfterFunc(lifecycleCtx, cancel)`（或等价）使 SIGTERM 取消 in-flight JW。
- **收益**：关机尾延迟从「最坏 ~刷新预算」变为可预测；避免 systemd stop 超时杀进程。
- **风险**：中 — 需保证 WaitBackground 与 in-flight 语义、测试时钟场景不抖。
- **工作量**：M
- **契约影响**：无（进程内行为）
- **建议后续任务**：`lifecycle-run-shutdown`（与 B-02 同任务）

### B-02 · `StartWarmup`/`WaitBackground` → `Run`/`Shutdown`

- **状态相对 07-26**：仍有效（原 O2）
- **类别**：可维护性
- **证据**：`service/warmup.go:76-99,162-180`；`service/classroom_service.go:61-66`（`backgroundMu`/`warmupStarted`/`warmupCancel`/`warmupDone`/`backgroundStopping`）；`main.go:101`
- **建议**：标准 `Run(ctx) error` / `Shutdown(ctx) error`（或 `Run` 阻塞到 ctx cancel），收敛五字段侧门注入。
- **收益**：生命周期可读性接近 `http.Server`；降低「谁负责 cancel」认知成本。
- **风险**：中 — 测试缝 `export_test`/warmup 测试需同步。
- **工作量**：M
- **契约影响**：无
- **建议后续任务**：与 B-01 合并为单一实现任务

### B-03 · 冷路径仍可阻塞至 30s（WriteTimeout 被迫 45s）

- **状态相对 07-26**：仍有效（原 O3，部分注释已文档化）
- **类别**：性能 / 可靠性
- **证据**：`service/realtime_data.go:19-22,69-89`；`main.go:80-84,109-112`
- **建议**：无可用缓存时有界等待（如 5s）后 `503` + `Retry-After`；或依赖 warmup 保证几乎总有缓存、冷路径仅运维窗口出现。
- **收益**：降低边缘超时与占满 worker 的尾延迟；WriteTimeout 可下探。
- **风险**：中高 — 改变冷启动用户体验与前端错误信封处理；需 CHANGELOG。
- **工作量**：M
- **契约影响**：需 CHANGELOG（新 503/Retry-After 行为）
- **建议后续任务**：`cold-path-bounded-wait`（独立；先确认产品接受）

### B-04 · `/api/get_data` 每请求 Marshal，无 ETag/预压缩体

- **状态相对 07-26**：仍有效（原 O6）
- **类别**：性能
- **证据**：`handler.go:40-58` + `router.go:30-38` `writeJSON`；共享 payload 见 `realtime_data.go:41-90`；`classroomResponse` 按 stale/error 拷贝改写（`realtime_data.go:297+`）
- **建议**：刷新成功时按 `(stale, errKind)` 组合预序列化（可选预 gzip），响应带 ETag/`Cache-Control`；注意与 SWR 客户端缓存叠加。
- **收益**：高 QPS 下减 CPU；304 降带宽。当前用户量下收益有限。
- **风险**：中 — 组合键漏缓存会回错误 Stale/Error 视图。
- **工作量**：M
- **契约影响**：需 CHANGELOG（新增缓存头；体不变则非破坏）
- **建议后续任务**：`api-etag-preserialize`（可在生命周期之后）

### B-05 · 缓存新鲜度：对外字段保留，内部可抽 `freshness`

- **状态相对 07-26**：仍有效（收窄；原「五套 TTL」中 store TTL 已死）
- **类别**：可维护性
- **证据**：`service/model/realtime_data.go:62-70`；`GetTodayClassrooms` 分支 `realtime_data.go:43-66`；spec `runtime-state-and-cache.md` 已文档化 Fresh/Stale/Expired
- **建议**：**不要**删除 `expires_at`/`stale_until`；可选内部 `type freshness int` + 单函数判定，压缩 `GetTodayClassrooms` 嵌套。
- **收益**：读路径更清晰；降低再引入第三套 TTL 的概率。
- **风险**：低（若只动内部）
- **工作量**：S
- **契约影响**：无（内部）；若动 JSON 则破坏性 — **禁止作为默认方案**
- **建议后续任务**：可附带在任意小后端卫生任务；不单独立项也可

### B-06 · 手写 refresh singleflight 与 token `singleflight.Group` 双轨

- **状态相对 07-26**：仍有效（原 S3）
- **类别**：可维护性
- **证据**：`refresh_coordinator.go:26-29,103-145`；`token_manager.go:48-49,276+`；spec 要求 shared-attempt + suppression metrics
- **建议**：**降级保留手写**；在 `refresh_coordinator.go` 文件头注释说明为何不用 `x/sync/singleflight`（backoff 绝对时间、`ObserveRefreshSuppressed`、与 `backgroundMu` 联锁）。若强行迁移，用 `DoChan` 且外置 backoff 状态机。
- **收益**：迁移省 ~40–60 行；保留则零回归风险。
- **风险**：迁移中高
- **工作量**：M（若做）
- **契约影响**：无
- **建议后续任务**：默认不建；仅当 B-01/B-02 重写协调器时顺带评估

### B-07 · `/readyz` 仍向公网返回完整 runtime 诊断

- **状态相对 07-26**：仍有效（原 O11）
- **类别**：可靠性
- **证据**：`handler.go:65-79`（`runtime` 全量序列化）
- **建议**：公网只返回 status/code（或精简字段）；详细诊断绑 loopback / 鉴权 / 独立 admin 路径。
- **收益**：减少信息面（凭证配置布尔、缓存状态等）。
- **风险**：低–中 — 运维脚本若依赖字段需改文档。
- **工作量**：S
- **契约影响**：需 CHANGELOG（字段删减）
- **建议后续任务**：`readyz-public-surface`（轻量）

### B-08 · `utils/` HTTP 客户端仍为杂物箱边界

- **状态相对 07-26**：仍有效（原 P3，已部分净化）
- **类别**：可维护性
- **证据**：`utils/http.go`（`HttpGet`/`HttpPostForm`/`NewHTTPClient`，~94 行）；仅 JW 路径使用；命名仍非 Go 惯用
- **建议**：迁入 `service`（如 `jw_http.go`）或改名为惯用标识；保持「禁止跟随重定向」注释契约。
- **收益**：包边界清晰；消除 `utils` 吸引更多无关函数。
- **风险**：低
- **工作量**：S
- **契约影响**：无
- **建议后续任务**：`utils-into-service`（轻量）

### B-09 · 根目录 HTTP 平铺 / 可选 `internal/httpapi`

- **状态相对 07-26**：仍有效（原 P5，低优先）
- **类别**：可维护性
- **证据**：根目录 `main.go`/`router.go`/`handler.go` + 多个 `*_test.go`
- **建议**：在无更大 HTTP 面扩张前保持；若再增路由再搬 `internal/httpapi`。
- **收益**：结构洁癖；当前文件数可控，收益低。
- **风险**：低（import 抖动）
- **工作量**：M
- **契约影响**：无
- **建议后续任务**：**关闭**为独立任务；记入「仅当 HTTP 面扩张时」

### B-10 · `realtime_data_test` 体积与集成边界

- **状态相对 07-26**：仍有效（收窄；原 O9 曾 1438 行，现约 511 行）
- **类别**：DX
- **证据**：`service/realtime_data_test.go` ~511 行；另有 `cache_policy_test.go`/`refresh_backoff_test.go`/`partial_refresh_test.go` 已拆出
- **建议**：若再增长，把需真凭据路径标 `//go:build integration`；其余保持白盒。
- **收益**：CI 默认更快、更稳。
- **风险**：低
- **工作量**：S
- **契约影响**：无
- **建议后续任务**：测试膨胀时再开；当前**降级**

### B-11 · 成功路径 `/api/get_data` 信封未带 `log_id`

- **状态相对 07-26**：新发现（07-26 前端侧提过丢弃；后端成功体仍缺字段）
- **类别**：可维护性 / DX
- **证据**：`handler.go:55-58` 成功只写 `code`/`data`；错误路径有 `log_id`（:46-50）；响应头已有 `X-Log-Id`（`router.go:94`）；前端可回退读头（`frontend/src/useTodayClassrooms.js:141`）
- **建议**：成功信封也带 `log_id`（与错误对称），便于排障与契约一致。
- **收益**：前后端错误/成功诊断路径统一。
- **风险**：极低
- **工作量**：S
- **契约影响**：需 CHANGELOG（加法，非破坏）
- **建议后续任务**：可并入 `readyz-public-surface` 或小 `api-envelope-hygiene`

### B-12 · ClassroomService 多锁分组仍在（可接受）

- **状态相对 07-26**：已过时作为「必须拆包」项（原 P4 部分缓解）
- **类别**：可维护性
- **证据**：`classroom_service.go:52-69` 仍 `refreshMu`/`backgroundMu`/`statusMu`；协调逻辑已文件级拆分 `refresh_coordinator.go`/`warmup.go`
- **建议**：保持现状；勿为拆而拆出第二套可变全局。
- **收益**：无强制动作
- **风险**：—
- **工作量**：—
- **契约影响**：无
- **建议后续任务**：关闭

---

## 建议执行顺序（后端）

1. **B-01 + B-02**（生命周期）— 批次④唯一强烈「继续做」
2. **B-07 + B-11**（小契约卫生）
3. **B-08**（utils 边界）
4. **B-04 / B-03**（性能与冷路径）— 有流量或产品批准后再做
5. **B-05 / B-06 / B-09 / B-10** — 降级或顺带
