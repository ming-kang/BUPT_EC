# Research: metrics 判空与死代码清单（backend-subtraction / R2-R7 精确使用点）

- **Query**: 核实 audit-backend.md S5/S6/S7/S9/S11/S12 各条在当前代码的精确 file:line，供 design.md 引用
- **Scope**: internal
- **Date**: 2026-07-26
- **行号基准**: 本文件所有行号以 2026-07-26 当前工作区为准（已核实 handler.go / main.go 行号与审计一致，无漂移）

## 1. Metrics 接口与构造函数现状（R2 基线）

**注意命名**：接口实际名为 `RuntimeMetrics`（不是 ClassroomMetrics）。

| 项 | 位置 | 内容 |
|---|---|---|
| `RuntimeMetrics` 接口 | `service/metrics.go:7-14` | 6 个方法：ObserveRefresh(outcome, duration)、ObserveRefreshSuppressed()、SetRefreshInFlight(bool)、ObserveCacheServe(state)、ObserveLogin(outcome, source, duration)、ObserveCampusFailure(campusID, kind) |
| `NoopMetrics` | `service/metrics.go:17-24` | 值类型 struct{}，6 个空实现，已存在可直接用 |
| `PrometheusMetrics` | `service/prometheus_metrics.go:10-21` | 私有 registry + 8 个 collector 字段 |
| Options 定义 | `service/classroom_service.go:86` | `Metrics RuntimeMetrics`，注释明确 "nil disables runtime metric emission" |
| 构造函数处理 | `service/classroom_service.go:118` | `metrics: options.Metrics` **原样透传，无默认值**（对比：clock 在 103-106、backoffRandom 在 107-110 都有 nil→默认注入） |
| 传入 TokenManager | `service/classroom_service.go:124` | `metrics: options.Metrics` 同样透传 |
| 生产注入点 | `main.go:52-60` | `NewPrometheusMetrics()` 后经 Options.Metrics 注入；`main.go:66` 用 `runtimeMetrics.Registry()` 建 /metrics handler |

## 2. metrics 判空点与 observeXxx 包装（穷举）

### 2a. `metrics != nil` 判空点 — 共 7 处

| # | 位置 | 形式 | 上下文 |
|---|---|---|---|
| 1 | `service/realtime_data.go:96` | `if s.metrics != nil` | observeCacheServe 包装体内 |
| 2 | `service/realtime_data.go:166` | `if s.metrics != nil` | observeRefresh 包装体内 |
| 3 | `service/realtime_data.go:172` | `if s.metrics != nil` | observeCampusFailure 包装体内 |
| 4 | `service/refresh_coordinator.go:123` | `if s.metrics != nil` | ObserveRefreshSuppressed 内联调用（startClassroomRefresh 抑制分支） |
| 5 | `service/refresh_coordinator.go:133` | `if s.metrics != nil` | SetRefreshInFlight(true) 内联 |
| 6 | `service/refresh_coordinator.go:161` | `if s.metrics != nil` | SetRefreshInFlight(false) 内联（finishClassroomRefresh） |
| 7 | `service/token_manager.go:206` | `if m == nil \|\| m.metrics == nil` | observeLogin 包装体内 |

### 2b. observeXxx 包装方法 — 共 4 个（与 PRD "4 个" 一致）

| 方法 | 位置 | 调用点 |
|---|---|---|
| `(*ClassroomService).observeCacheServe` | `service/realtime_data.go:95-99` | realtime_data.go:50,61,63,67,74,83,85,87（共 8 处调用） |
| `(*ClassroomService).observeRefresh` | `service/realtime_data.go:165-169` | realtime_data.go:142,149,159 |
| `(*ClassroomService).observeCampusFailure` | `service/realtime_data.go:171-175` | realtime_data.go:144,151 |
| `(*TokenManager).observeLogin` | `service/token_manager.go:205-210` | token_manager.go:168,179 |

### 2c. PrometheusMetrics nil-receiver 检查 — 共 7 处（全部方法都有）

| 方法 | nil 检查行 |
|---|---|
| `Registry()` | `service/prometheus_metrics.go:84-86` |
| `ObserveRefresh` | `service/prometheus_metrics.go:91-93` |
| `ObserveRefreshSuppressed` | `service/prometheus_metrics.go:100-102` |
| `SetRefreshInFlight` | `service/prometheus_metrics.go:107-109` |
| `ObserveCacheServe` | `service/prometheus_metrics.go:118-120` |
| `ObserveLogin` | `service/prometheus_metrics.go:125-127` |
| `ObserveCampusFailure` | `service/prometheus_metrics.go:133-135` |

无任何测试对 nil `*PrometheusMetrics` receiver 调用方法（prometheus_metrics_test.go 无 nil-receiver 用例）→ 7 处 nil-receiver 检查可安全删除。

### 2d. R2 落地关键 caveat：TokenManager 直构测试

TokenManager 在测试中被 **13 处** 以 `&TokenManager{...}` 直接构造（绕过 NewClassroomService 的默认注入）：
- `service/token_manager_test.go:140,173,206,264` — 全部无 metrics、无 clock
- `service/token_manager_login_metrics_test.go:70,110,137,172,200,253,291,314,335` — 其中 335（TestLoginMetricsNilMetricsSafe）**显式断言 nil metrics 安全**，70/291 有 clock，其余无 clock

若只在 NewClassroomService 默认注入 Noop 并删掉 token_manager.go:206 判空，这 13 处直构中无 metrics 的测试会 nil panic。design 需三选一：
(a) 更新直构测试补 `metrics: NoopMetrics{}`（并删 TestLoginMetricsNilMetricsSafe:334-345）；
(b) TokenManager 也在使用点保留判空（与 R2 目标矛盾）；
(c) 加 TokenManager 构造帮助函数。方案 (a) 与 PRD 精神一致。

另：`handler.go:100` 有 `server.metricsHandler == nil`（HTTP 层 /metrics 404 分支），这是 handler 判空不是 RuntimeMetrics 判空，R2 不涉及。

## 3. 死代码逐项核实（R3）

| 项 | 定义位置 | 引用数（grep 全仓 *.go 含测试） | 结论 |
|---|---|---|---|
| `model.QueryResponse` | `service/model/realtime_data.go:26-30` | 0 引用（解析实际用 `jwResponseEnvelope`，jw_client.go:134-138；parseJWQueryResponse 在 jw_client.go:140） | 死代码，可删 |
| `logs.LogIDKey` | `logs/log_util.go:19-21`（const "K_LOGID"） | 0 Go 引用；仅 `.trellis/spec/backend/logging-guidelines.md:42` 文档提及（删后需主代理同步 spec） | 死代码，可删 |
| `classifyError` ctx 分支 | `service/jw_error.go:66-68` | if 分支与 fallthrough（line 69）都返回 `string(jwErrorUpstream)` → 分支无语义 | 死分支，可删（行号与审计 66-69 一致，无漂移） |

## 4. Login/QueryOne/QueryAll/forceRefresh/loginPerformed 调用图（R3）

### 4a. 调用图

```
ClassroomService.Login (realtime_data.go:31-34)
  └─> TokenManager.EnsureToken(ctx, true)          ← 唯一生产调用 forceRefresh=true 的入口
调用者: 仅 TestLogin (realtime_data_test.go:1404-1411, 需 JW_USERNAME/PASSWORD 凭据)

ClassroomService.QueryOne (realtime_data.go:36-38)
  └─> s.queryCampus(ctx, id)
调用者: 仅 TestQueryOne (realtime_data_test.go:1413-1423, 需凭据)

ClassroomService.QueryAll (realtime_data.go:40-42)
  └─> classroomResponseFromRefresh(s.refreshTodayClassrooms(ctx))   ← 同步驱动一次刷新
调用者（全部为测试，无生产调用）:
  - 单测(白盒): realtime_data_test.go:443(TestQueryAllBuildsTodayClassroomsFromJWFixture),
    796(TestQueryAllQueriesCampusesConcurrently), 916(TestDoRefreshPartialCampusSuccess),
    972(TestPartialRefreshWritesWarningLog), 999,1003(TestFullRefreshClearsPartialRuntimeWarning),
    1103(TestDoRefreshPartialCampusMergesPreviousCache), 1121(TestDoRefreshAllCampusesFail)
  - 集成: realtime_data_test.go:1428(TestQueryAll, 需凭据)

生产可达路径（对照）: refreshTodayClassrooms 的生产入口是
startClassroomRefresh 的 worker goroutine (refresh_coordinator.go:147)；
queryCampus 的生产入口是 doRefreshTodayClassrooms (realtime_data.go:190)。

EnsureToken(ctx, false): 生产调用仅 queryCampus (realtime_data.go:102)；
  测试直接调用: realtime_data_test.go:246,889; token_manager_test.go:150,224,234;
  token_manager_login_metrics_test.go:80,100,119,147,220,300,322,342（全为 false）
EnsureToken(ctx, true) 测试直接调用: realtime_data_test.go:255
  (TestEnsureTokenUsesOverrideOnlyWithoutForceRefresh)

loginPerformed 字段 (token_manager.go:23):
  写: loginAndStore (token_manager.go:185)
  读: 仅 token_manager.go:94 —— 删 forceRefresh 后字段可删
```

**所有 service 测试均为白盒（package service）**，因此 QueryOne/QueryAll 改非导出不会破坏任何现有测试装配；删除 `ClassroomService.Login` 只需改 TestLogin（PRD 已定为改直调 JWClient）。

### 4b. EnsureToken for-continue 重试环逻辑摘要（token_manager.go:60-99）

1. 非 force 快路径（66-70）：cachedTokenState 有 token 直接返回。
2. `for` 环每轮：检查 ctx.Err → `tokenGroup.DoChan("jw-token", ...)` 合流；闭包内非 force 再查缓存→尝试 installOverrideToken（JW_TOKEN 环境覆盖，未被 invalidate 时装载）→否则 loginAndStore(ctx,"login") 真登录。
3. 环唯一的 continue 条件（94）：`forceRefresh && !result.loginPerformed` —— force 调用者可能合流进别人"只装了 override/缓存 token"的航班，此时重转一圈保证 force 语义 = 真登录。
4. **forceRefresh=false 时环恒单次通过**（continue 不可达）。删除 Login/forceRefresh 后：环可坍缩为一次 DoChan+wait，loginPerformed 字段、tokenOperationResult.loginPerformed、authRecoveryDecision 不受影响（RefreshAfterAuthFailure 不读 loginPerformed）。

## 5. 三处 reflect 判空工具（R6）

| 工具 | 定义 | 调用点 | 删除后失效的测试 |
|---|---|---|---|
| `isNilClassroomService` | `handler.go:29-40` | `handler.go:43`（NewHTTPServer） | `handler_test.go:58-69` TestNewHTTPServerRejectsNilService 的 typed-nil 用例（62-65 行 `var typedNil *fakeClassroomService`） |
| `isNilDependency` | `service/dependency.go:5-16` | `service/classroom_service.go:93,96`；`service/jw_client.go:30` | `service/construction_test.go:29,31`（"typed nil cache" / "typed nil JW client" 子用例，typedNil 变量在 19-20 行）；`service/jw_client_test.go:64-77` TestNewJWClientRejectsNilDoerWithoutLeakingCredentials（typedNilDoer 在 67-68 行） |
| `isNilHTTPDoer` | `utils/http.go:109-120` | `utils/http.go:62`（httpRequest） | `utils/http_test.go:156-164` TestHTTPHelpersRejectNilDoer（typedNilDoer 在 157-158 行） |

三份实现逐字节等价（同一 switch Kind 列表）。收敛为一份的 import 约束：service→utils 已存在（jw_client.go:12），main→service/utils 均可；utils 不能 import service（会成环）→ 唯一单点候选是 utils（或复制进将来的 internal 包）。若直接删除改 `== nil`，上表 4 个 typed-nil 测试用例失效（普通 nil 用例仍过）。

## 6. 两套退避阶梯对比（R5）

### 数值

| 输入 n | `totalFailureBackoffBase(n)` refresh_coordinator.go:55-63（steps 定义 41-46） | `warmupFailureDelay(n)` warmup.go:30-41（常量 10-13） |
|---|---|---|
| 0 | 30s（clamp <1→1） | 30s（case 0,1） |
| 1 | 30s | 30s |
| 2 | 1m | 1m |
| 3 | 2m | 2m |
| ≥4 | 5m（clamp >4→4） | 5m（default） |

**全定义域逐点相等** → 数值上可无损合并为单一 `backoffLadder(n)`。

### 消费方式差异（合并时只合"基础阶梯"，不合消费语义）

- 协调器侧：finishClassroomRefresh（refresh_coordinator.go:166-175）在 refreshMu 下按 `consecutiveTotalFailures` 取 base，叠加 `jitteredBackoff` ±min(10%·base, 5s)（87-103，样本来自 BackoffRandom），存为**绝对时刻** `nextRefreshAllowed` 闸门。partial 结果走固定 `staleRefreshBackoff=30s`（realtime_data.go:24, refresh_coordinator.go:180），与阶梯无关。
- warmup 侧：nextWarmupDelay（warmup.go:81-83）把 `warmupFailureDelay(failures)` 当**相对睡眠时长**目标，再与 `nextRefreshAllowedAt()` 取 max；**不加 jitter**（warmupJitter 1-5s 只用于午夜滚动，warmup.go:14-15,26-28,63）。failures 计数独立（nextWarmupFailureCount, warmup.go:43-51）。
- 结论：合并安全。需同步改两个测试：`refresh_backoff_test.go:14-31`（TestTotalFailureBackoffBaseLadder，含 n=0 clamp 断言）与 `warmup_test.go:23-39`（TestWarmupFailureDelayCapsAtFiveMinutes）。其余引用（refresh_backoff_test.go:111,270; realtime_data_test.go:1061）只调 totalFailureBackoffBase 计算期望值，函数换名即可。

## 7. S12 小项当前状态核实（R7）

| 项 | 当前状态 | 备注 |
|---|---|---|
| `now()` 冗余判空 | `ClassroomService.now`（classroom_service.go:133-138）：`s == nil \|\| s.clock == nil` **完全冗余** —— 构造函数 103-106 保证 clock 非 nil，且全仓无 `&ClassroomService{}` 结构体字面量直构（grep 仅构造函数 111 行一处） | `TokenManager.now`（token_manager.go:53-58）的 clock 判空**当前是测试承重的**：13 处 `&TokenManager{}` 直构中 11 处无 clock（见 §2d 清单）。`m != nil` 部分冗余。删 clock 判空需同步给直构测试补 clock 或加默认 |
| runtime_status cloneTime / `copy` 遮蔽 | `cloneTime` 定义 `service/runtime_status.go:89-92`，`copy := t` 遮蔽内建 copy；调用点 5 处：24,37,45,81,84 | 字段 `LastLoginSuccessAt/LastRefreshSuccessAt *time.Time`（runtime_status.go:8,10, omitempty）。go.mod 为 `go 1.25.12`，`omitzero` 可用。改值类型后 snapshotRuntimeStatus（76-87）的指针深拷贝也可退化为纯结构体赋值。JSON 影响面：/readyz 响应（handler.go:91-96）与 handler_test.go:129-135 反序列化进 service.RuntimeStatus——omitzero 语义（零值省略）与现 omitempty(nil 省略) 对外等价 |
| `addQuery` | 定义 `service/urlutil.go:17-28`；**唯一调用点** `service/jw_client.go:48`；无直接测试 | 可安全内联 |
| `hasJWCredentials func() bool` | 字段 `handler.go:25`；调用 `handler.go:84`（Readyz）；nil 兜底 `handler.go:46-48`；生产注入 `main.go:69` 传方法值 `runtimeConfig.HasJWCredentials`（config/config.go:90-93，纯函数，启动后配置不可变→bool 等价） | **caveat**：`handler_test.go:71-104` TestReadyzRequiresConfiguredCredentialsAndUsableCache 在两次请求间改闭包捕获变量 `credentialsConfigured`（73-76, 88 行），依赖晚绑定；改 bool 需重构该测试（分两个 server 实例） |
| `interface{}` 出现次数 | **6 处，全在生产代码**：service/jw_error.go:54（variadic 参数）、service/token_manager.go:76,110,134（DoChan 闭包签名，受 singleflight API 约束但可写 `any`）、service/token_manager.go:301（waitSingleflightResult 返回值）、service/classroom_service.go:36（注释文字） | 测试文件 0 处 |

## Related Specs

- `.trellis/spec/backend/logging-guidelines.md:42` — 引用了 `logs.LogIDKey`，删除后需主代理用 update-spec 同步。

## Caveats / Not Found

- 审计 S11 称 "20+ 处 `!= nil` 判空"：当前实测为 **7 处 metrics 判空 + 7 处 PrometheusMetrics nil-receiver = 14 处防御检查**（外加 4 个包装方法）。可能审计把包装方法调用点也计入；不影响结论，仅数字口径。
- 审计/任务描述称接口为 "ClassroomMetrics"，实际名为 **RuntimeMetrics**（service/metrics.go:7）。
- handler.go / main.go 虽被上个子任务改过，但本任务涉及的行号（isNilClassroomService 29-40、jw_error 66-69 等）与审计**一致，无漂移**。
- `warmupJitter` 字段（classroom_service.go:68，构造 117 行赋 randomWarmupJitter）在 `warmup_test.go:92` 被构造后无同步直写（O8/R8 佐证），R8 改 Options 字段时该行是唯一测试写点。
- refresh_backoff_test.go 多处直写未导出字段（102-103,133-134,203-204,332-335 等 `svc.refreshInFlight/refreshAttempt/consecutiveTotalFailures`）——R8 export_test.go 种子函数的主要客户。
