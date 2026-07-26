# Research: 测试耦合地图（O8/O9 测试收敛前置研究）

- **Query**: service 包测试对 ClassroomService/TokenManager 未导出字段的直接读写清单；realtime_data_test.go 用例分布与拆分方案；export_test.go 种子函数设计；warmupJitter 写入分析；现有测试基建清单
- **Scope**: internal
- **Date**: 2026-07-26
- **行号基准**: 当前工作区代码（git 无未提交 Go 改动，最近触及 main.go/handler.go 的提交是 01bfa3d；audit 中的行号与当前一致，未发现漂移）

---

## 1. 未导出字段直接读写清单（按字段分组）

约定：`(w)` 写、`(r)` 读、`(locked)` 表示访问时持有了对应互斥锁，未标注即无锁裸访问。

### ClassroomService.refreshInFlight（定义 classroom_service.go:55）

全部为写 `= true`，用于伪造"有一次刷新在途"以便调用 `finishClassroomRefresh` 驱动真实状态迁移：

| 位置 | 锁 | 所在测试 |
|---|---|---|
| `service/refresh_backoff_test.go:102` (w) | 无锁 | TestFinishClassroomRefreshSamplesOnceAndAppliesJitter |
| `service/refresh_backoff_test.go:133` (w) | 无锁 | TestTotalFailureLadderEscalatesAndFullResets（循环内） |
| `service/refresh_backoff_test.go:150` (w) | 无锁 | 同上（partial 分支） |
| `service/refresh_backoff_test.go:165` (w) | 无锁 | 同上（full 分支） |
| `service/refresh_backoff_test.go:203` (w) | 无锁 | TestRefreshSuppressedDoesNotStartWorkerAndCountsOncePerCall |
| `service/refresh_backoff_test.go:252` (w) | locked（refreshMu :251-254） | TestConcurrentCallersShareNextRefreshAllowed |
| `service/refresh_backoff_test.go:334` (w) | 无锁 | TestBackoffCrossingMidnightRejectsOldCacheThenAllowsNewDayRefresh |
| `service/refresh_backoff_test.go:397` (w) | locked（refreshMu :396-399） | TestFakeClockAndCoordinatorRace |
| `service/realtime_data_test.go:1025` (w) | 无锁 | TestFinishClassroomRefreshTransitionsBackoffByOutcome（partial） |
| `service/realtime_data_test.go:1042` (w) | 无锁 | 同上（full） |
| `service/realtime_data_test.go:1053` (w) | 无锁 | 同上（failed） |

### ClassroomService.refreshAttempt（classroom_service.go:56）

与 refreshInFlight 成对写入（同一行号 +1）：`refresh_backoff_test.go:103, 134, 151, 166, 204, 253(locked), 335, 398(locked)`；`realtime_data_test.go:1026, 1043, 1054`。均为 `(w)` 写入测试自建的 `&classroomRefreshAttempt{done: make(chan struct{})}`。

### ClassroomService.nextRefreshAllowed（classroom_service.go:57）

全部为 `(r)` 读断言（无锁；生产读法是 `nextRefreshAllowedAt()`，refresh_coordinator.go:195-199）：

- `service/refresh_backoff_test.go:112-113`（断言 jitter 后的截止点）
- `service/refresh_backoff_test.go:140-141`（阶梯逐级断言）
- `service/refresh_backoff_test.go:160-161`（partial → staleRefreshBackoff）
- `service/refresh_backoff_test.go:171-172`（full → IsZero 复位）
- `service/refresh_backoff_test.go:271-272`（并发后共享截止点）
- `service/realtime_data_test.go:1031-1032, 1048-1049, 1061-1062`（三种 outcome 迁移）

### ClassroomService.lastRefreshErr（classroom_service.go:58）

全部 `(r)`：`service/realtime_data_test.go:1034-1035`（partial 清空）、`:1048-1049`（full 清空）、`:1064-1065`（failed 记录 errors.Is）。

### ClassroomService.consecutiveTotalFailures（classroom_service.go:59）

- `(r)`：`refresh_backoff_test.go:144-145, 152, 157-158, 171-172, 266-267`；`realtime_data_test.go:1037-1038, 1067-1068`
- `(w)`：`refresh_backoff_test.go:332`（`svc.consecutiveTotalFailures = 3`，无锁预置阶梯位——唯一一处对该字段的测试写入）

### ClassroomService.warmupJitter（classroom_service.go:68）

- `(w)`：`service/warmup_test.go:92`（`svc.warmupJitter = func() time.Duration { return 0 }`，无锁）——唯一访问点，详见 §5。

### ClassroomService.cache（classroom_service.go:46，TodayClassroomCache 接口字段）

全部为 `(r)` 字段后调 `Store(value, ttl)` 种缓存（TTL 实参 time.Minute/time.Hour 均为凑数，不承载断言语义）：

| 位置 | 所在测试 |
|---|---|
| `service/realtime_data_test.go:270, 281` | TestGetCachedTodayClassroomsRejectsCrossDayCache |
| `service/realtime_data_test.go:382` | TestGetTodayClassroomsReturnsFreshCacheWithoutJWQuery |
| `service/realtime_data_test.go:586` | TestGetTodayClassroomsReturnsStaleWhileRefreshContinues |
| `service/realtime_data_test.go:746` | TestGetTodayClassroomsBacksOffAfterStaleRefreshFailure |
| `service/realtime_data_test.go:1087` | TestDoRefreshPartialCampusMergesPreviousCache |
| `service/realtime_data_test.go:1146` | TestStalePartialCacheUsesLatestTotalRefreshFailure |
| `service/realtime_data_test.go:1214` | TestGetTodayClassroomsRetriesPartialErrorWithinFreshTTL |
| `service/realtime_data_test.go:1299` | TestGetTodayClassroomsPartialErrorRefreshCanRecoverFailedCampus |
| `service/realtime_data_test.go:1359, 1370` | TestRuntimeStatusCacheStaleOnlyWhenPastFreshTTL |
| `service/refresh_backoff_test.go:323` | TestBackoffCrossingMidnightRejectsOldCacheThenAllowsNewDayRefresh |

共 12 处。另有构造耦合：`realtime_data_test.go:83`（`cache.New()` 作为 NewClassroomService 第 2 参）、`construction_test.go:17, 19, 55`（`cache.New()`、`*cache.TodayClassroomsStore` typed-nil）。

### ClassroomService.clock

测试**没有**直接访问 `svc.clock`；统一走 `ClassroomServiceOptions.Clock` 注入 fakeClock。无需种子函数。

### 其他 ClassroomService 字段

| 字段 | 位置 | 说明 |
|---|---|---|
| refreshMu | `refresh_backoff_test.go:251/254, 396/399` (lock/unlock) | 并发测试里模拟生产加锁次序 |
| backgroundMu | `warmup_test.go:168/170, 172/174, 215/217` (lock/unlock) | 保护下面两个字段的读 |
| warmupDone | `warmup_test.go:169, 173` (r, locked) | 比较两次 StartWarmup 返回同一调度器 |
| backgroundStopping | `warmup_test.go:216` (r, locked) | 轮询 WaitBackground 已进入停止态 |
| campuses | `construction_test.go:62` (r) | 断言构造函数拷贝了 slice |
| tokenManager | `realtime_data_test.go:246, 254, 255, 826, 889`；`token_manager_test.go:52, 118` (r 字段后调方法) | 见下 |

### TokenManager 侧

**通过 `svc.tokenManager` 调未导出/导出方法：**

- `setToken(...)`（未导出）：`realtime_data_test.go:826`、`token_manager_test.go:52, 118`；直接对裸 manager：`token_manager_test.go:184`、`token_manager_login_metrics_test.go:180, 261`
- `setAPIURL(...)`（未导出）：`realtime_data_test.go:254`
- `EnsureToken(...)`（导出方法但经私有字段链）：`realtime_data_test.go:246, 255, 889`

**直接读写 TokenManager 未导出字段（均 locked，持 `manager.mu`）：**

| 位置 | 字段 | 读/写 | 所在测试 |
|---|---|---|---|
| `service/token_manager_test.go:161` | overrideInvalidated | r | TestAuthFailureInvalidatesOnlyRejectedOverrideSource |
| `service/token_manager_test.go:162` | tokenSource | r | 同上 |
| `service/token_manager_test.go:182` | overrideInvalidated | **w** | TestLoginTokenFailurePreservesOverrideInvalidationState |
| `service/token_manager_test.go:190` | overrideInvalidated | r | 同上 |
| `service/token_manager_test.go:191` | tokenSource | r | 同上 |

**`&TokenManager{...}` 复合字面量直接填未导出字段（jwClient/overrideToken/metrics/clock），共 13 处：**

- `token_manager_test.go:140-148`（overrideToken+jwClient，**metrics=nil**）
- `token_manager_test.go:173-180`（overrideToken+jwClient，**metrics=nil**）
- `token_manager_test.go:206-219`（jwClient，**metrics=nil**）
- `token_manager_test.go:264-277`（jwClient，**metrics=nil**，仅走 APIURL 路径）
- `token_manager_login_metrics_test.go:70-78`（metrics+clock+jwClient）、`:110-117`、`:137-145`、`:172-179`、`:200-214`、`:253-260`、`:291-299`（metrics+clock）、`:314-321`、`:335-341`（**故意 metrics=nil**，TestLoginMetricsNilMetricsSafe）

---

## 2. 各访问在测什么 & 改造后的影响

### 各组访问对应的被测行为

- **refreshInFlight/refreshAttempt 写 + finishClassroomRefresh 调用**：绕过真实 worker goroutine，直接驱动 `finishClassroomRefresh`（refresh_coordinator.go:153-187）的状态机——jitter 只采样一次（backoff:89-115）、阶梯升级/复位（backoff:117-174, realtime:1016-1070）、抑制计数（backoff:185-229）、跨午夜 backoff（backoff:294-377）、并发 finish 一致性（backoff:231-292, 379-407）。
- **nextRefreshAllowed/lastRefreshErr/consecutiveTotalFailures 读**：断言上述状态机的三种 outcome（failed/partial/full）迁移结果。
- **consecutiveTotalFailures=3 写（backoff:332）**：预置阶梯位到第 4 级（5m 基线）以构造跨午夜场景。
- **cache.Store 种子**：预置"今天有缓存"的前置态（fresh/soft-stale/stale/隔天/partial-error 各变体），断言 GetTodayClassrooms 的分支选择与 RuntimeStatus 派生。
- **tokenManager.setToken/setAPIURL**：预置"已有过期 token"/"已知 API URL"，测认证失败恢复与 EnsureToken(true) 不落网。
- **TokenManager.mu + overrideInvalidated/tokenSource**：断言 override 失效标记的单向性（一旦失效不再重装）。
- **backgroundMu/warmupDone/backgroundStopping**：断言 StartWarmup 幂等与 WaitBackground 拦截新 worker。

### R1（cache → atomic.Pointer[model.TodayClassrooms]）后的编译失败

- **12 处 `svc.cache.Store(v, ttl)`**（§1 cache 组）：字段消失/签名变化（atomic.Pointer.Store 单参数）→ 编译失败。用 `seedCache` 种子函数收敛。
- `realtime_data_test.go:83`（`NewClassroomService(options, cache.New(), client)`）与 `construction_test.go:17, 19, 28-31, 37, 53-57`：构造函数去掉 store 参数后编译失败；"missing cache"/"typed nil cache" 两个用例**语义消失，应删除**。
- `cache/cache_test.go`（30 行）随包删除。
- **行为断言失效**：`TestCacheExpirationAlwaysPositive`（realtime_data_test.go:292-313）测的 `cacheExpiration`（realtime_data.go:281-291）被删 → 整测删除。`TestDoRefreshStampsCacheAtCompletionAcrossMidnight` 中 `:355-357` 一行断言 cacheExpiration 为正 → 删该断言，其余（完成日戳）保留。其余种缓存测试不受影响——跨天拒绝靠 `getCachedTodayClassroomsAt` 的 Date 比较（realtime_data.go:323-332），不靠 go-cache TTL；没有任何测试依赖 go-cache 的真实过期驱逐。

### R2（Noop 默认注入 + 删 `!= nil` 判空）后的影响

- ClassroomService 侧无影响：所有实例都经 `NewClassroomService`，测试注入的 `countingSuppressedMetrics`/`recordingLoginMetrics` 都嵌入 NoopMetrics，非 nil。
- **风险在 TokenManager 裸字面量**：若删掉 `observeLogin` 的 nil 判空（token_manager.go:205-210），以下 metrics=nil 且会走到 `loginAndStore → observeLogin` 的测试将 **运行时 panic（非编译失败）**：
  - `token_manager_test.go:140`（RefreshAfterAuthFailure → login）
  - `token_manager_test.go:173`（同上）
  - `token_manager_test.go:206`（EnsureToken → login）
  - `token_manager_login_metrics_test.go:335`（TestLoginMetricsNilMetricsSafe——该测试的存在意义就是 nil 判空，**判空删除后此测试应删除或改为"构造函数默认 Noop"断言**）
  - `token_manager_test.go:264` 只走 APIURL 路径，不触 ObserveLogin，安全（但防御起见也应统一）。
  - 收敛方式：export_test.go 提供 `newTokenManagerForTest`（默认 metrics=NoopMetrics{}），13 处字面量改走它；或 TokenManager 构造统一由 NewClassroomService 注入（现状 classroom_service.go:120-127 已传 options.Metrics，改为传默认化后的 metrics 即可覆盖生产路径）。
- `m.now()` 的 nil 判空（token_manager.go:53-58）：`token_manager_login_metrics_test.go` 多数字面量不设 clock，删 `m.clock == nil` 分支前必须保证测试构造默认注入 clock（seed 函数做）或保留该判空。

### 批次④预告（不在本任务，但设计种子函数时要预留）

refreshInFlight/refreshAttempt 若被 singleflight 替换（S3），所有"手工置位 + finishClassroomRefresh"的测试都需重写——因此种子函数应封装**语义**（"完成一次结果为 X 的刷新"）而非字段。

---

## 3. realtime_data_test.go（1438 行）用例分布

共 36 个测试函数 + 8 个共享 helper。

### 走真实网络的集成测试（3 个，文件尾部）

| 测试 | 行 | Skip 条件 |
|---|---|---|
| TestLogin | 1404-1411 | `requireJWLoginCredentials`（:124-129）：`JW_USERNAME` 或 `JW_PASSWORD` 任一为空即 skip（JW_TOKEN 无效，因 Login 强制真登录） |
| TestQueryOne | 1413-1423 | `requireJWCredentials`（:114-122）：`JW_TOKEN` 非空则跑；否则 `JW_USERNAME`/`JW_PASSWORD` 任一为空即 skip |
| TestQueryAll | 1425-1438 | 同 TestQueryOne |

三者经 `newIntegrationService`（:106-112）→ `NewJWClient(env 凭据, utils.NewHTTPClient())` 真连 jwglweixin.bupt.edu.cn。env key 定义在 config/config.go:14-16。

### 特殊边界用例（1 个）

- `TestEnsureTokenUsesOverrideOnlyWithoutForceRefresh`（:243-265）：用**真实 JWClient**（`newHTTPJWClientForTest(t, "", "")`）但**不落网、不 skip**——先 `setAPIURL(DefaultAPIURL)` 跳过 serverconfig 拉取，空凭据在发请求前就报 jwErrorConfig（jw_client_test.go:79-92 佐证该 fail-fast）。拆分时它留在单测，但依赖的 `newHTTPJWClientForTest` 不能被挪进带 build tag 的文件。

### 纯单测（32 个，全部 mockJWClient/fixture 驱动）

按主题聚类（即拆分建议的依据）：

- **纯函数/协议解析（8）**：TestEncryptJWPassword:131、TestParseRoom:144、TestBuildCampusInfoDeduplicatesRooms:170、TestValidateJWAPIURL:197、TestClassifyErrorHandlesJoinedJWError:535、TestParseJWQueryResponseClassifiesBusinessAuthCode:542、TestClassifyJWHTTPErrorUsesBusinessAuthCode:552、TestIsAuthFailureMessageDoesNotMatchBareExpiry:1382 (+TestSafeErrorMessageForNoTodayCache:1397)
- **缓存/日界策略（4）**：TestGetCachedTodayClassroomsRejectsCrossDayCache:267、TestCacheExpirationAlwaysPositive:292（R1 删）、TestDoRefreshStampsCacheAtCompletionAcrossMidnight:315、TestEndOfDayIsNextMidnightShanghai:1336
- **GetTodayClassrooms 主流程（7）**：:371, 562, 627, 682, 735, 1185, 1273
- **partial 刷新族（5）**：TestDoRefreshPartialCampusSuccess:901、TestPartialRefreshWritesWarningLog:952、TestFullRefreshClearsPartialRuntimeWarning:982、TestDoRefreshPartialCampusMergesPreviousCache:1072、TestDoRefreshAllCampusesFail:1113、TestStalePartialCacheUsesLatestTotalRefreshFailure:1134
- **QueryAll fixture/并发（2）**：:410, 778
- **token 认证恢复（2）**：TestQueryCampusRefreshesTokenAfterAuthFailure:809、TestEnsureTokenDoesNotReapplyInvalidatedJWToken:855（+:243 边界用例）
- **协调器状态机（1）**：TestFinishClassroomRefreshTransitionsBackoffByOutcome:1016（与 refresh_backoff_test.go 重复主题，应并入）
- **RuntimeStatus（1）**：TestRuntimeStatusCacheStaleOnlyWhenPastFreshTTL:1354

### 建议拆分方案

| 新文件 | 内容 | 来源行 |
|---|---|---|
| `service/testsupport_test.go` | init(logs)、mockJWClient、newTestService*、newHTTPJWClientForTest、waitFor、requireCampusByID/Building/Room、tokenTestRows | :25-104, 502-533, 843-853；token_manager_test.go:15-21 |
| `service/integration_test.go`（`//go:build integration`） | TestLogin、TestQueryOne、TestQueryAll、newIntegrationService、requireJWCredentials、requireJWLoginCredentials | :106-129, 1404-1438 |
| `service/classroom_builder_test.go` | TestParseRoom、TestBuildCampusInfoDeduplicatesRooms | :144-195 |
| 并入现有 `service/jw_protocol_test.go`（或 jw_error_test.go） | TestValidateJWAPIURL、TestClassifyError*、TestParseJWQueryResponse*、TestClassifyJWHTTPError*、TestIsAuthFailureMessage*、TestSafeErrorMessage* | :197-241, 535-560, 1382-1402 |
| 并入现有 `service/crypto_test.go` | TestEncryptJWPassword | :131-142 |
| `service/cache_policy_test.go` | 跨天拒绝、完成日戳、endOfDay | :267-290, 315-369, 1336-1352 |
| `service/realtime_data_test.go`（保留，缩至 ~500 行） | GetTodayClassrooms 主流程 7 个 + QueryAll fixture/并发 2 个 | :371-408, 562-807, 1185-1334 |
| `service/partial_refresh_test.go` | partial 族 6 个 | :901-1014, 1072-1180 |
| 并入现有 `service/refresh_backoff_test.go` | TestFinishClassroomRefreshTransitionsBackoffByOutcome | :1016-1070 |
| 并入现有 `service/token_manager_test.go` | TestQueryCampusRefreshesTokenAfterAuthFailure、TestEnsureTokenDoesNotReapplyInvalidatedJWToken、TestEnsureTokenUsesOverrideOnlyWithoutForceRefresh | :243-265, 809-841, 855-899 |
| 并入现有的 runtime status 测试位置（可新建 `runtime_status_test.go`） | TestRuntimeStatusCacheStaleOnlyWhenPastFreshTTL | :1354-1380 |

注意：R3 若把 `QueryAll` 改非导出（`queryAll`），单测 8 处调用点（:443, 796, 916, 972, 999, 1003, 1103, 1121）同包改名即可；`Login` 生产零调用、仅 TestLogin 用（PRD 方案：集成测试改直调 JWClient.Login）；`QueryOne` 仅 TestQueryOne 用。

---

## 4. export_test.go 种子函数集设计（对应 PRD R8）

设计原则：封装**行为语义**而非字段，让批次④（singleflight/freshness）只改 export_test.go 一个文件。全部放 `service/export_test.go`（同包白盒，小写命名即可）。

| 建议签名 | 替代的直接访问 | 说明 |
|---|---|---|
| `func seedCache(t *testing.T, svc *ClassroomService, today *model.TodayClassrooms)` | §1 cache 组全部 12 处 `svc.cache.Store(v, ttl)` | R1 后内部即 `svc.today.Store(today)`；t.Helper 可校验 Date 非空。TTL 参数消失（原本就是凑数） |
| `func completeRefresh(svc *ClassroomService, result classroomRefreshResult)` | 11 组 "refreshInFlight=true; refreshAttempt=attempt; finishClassroomRefresh(...)" 三连（refresh_backoff_test.go:101-107, 132-138, 149-155, 164-170, 202-208, 248-259, 333-339, 395-403；realtime_data_test.go:1024-1030, 1041-1047, 1052-1059） | 内部持 refreshMu 置位后调 finishClassroomRefresh；自建 attempt 并返回前 `<-attempt.done` 已闭。批次④换 singleflight 时只改此函数 |
| `func forceFailureState(svc *ClassroomService, consecutive int, next time.Time)` | `refresh_backoff_test.go:332`（consecutiveTotalFailures=3 预置） | 持 refreshMu 写 consecutiveTotalFailures 与 nextRefreshAllowed（PRD 中 forceFailureState(svc, n, until) 形态） |
| `func backoffState(svc *ClassroomService) (next time.Time, consecutive int, lastErr error)` | nextRefreshAllowed/lastRefreshErr/consecutiveTotalFailures 全部读点（§1 对应组，共 16 处断言位） | 持 refreshMu 快照；`nextRefreshAllowedAt()`（refresh_coordinator.go:195）可继续用或并入 |
| `func installToken(svc *ClassroomService, token string)` | `realtime_data_test.go:826`、`token_manager_test.go:52, 118` 的 `svc.tokenManager.setToken(x, tokenSourceLogin)` | 也可直接保留 setToken 调用（同包方法），但集中后批次④改 token 存储不扩散 |
| `func installAPIURL(svc *ClassroomService, apiURL string)` | `realtime_data_test.go:254` | 同上 |
| `func newTokenManagerForTest(client JWClient, mutate func(*TokenManager)) *TokenManager` 或 options 形态 `newTokenManagerForTest(tmOpts{Override string; Metrics RuntimeMetrics; Clock Clock}, client)` | 13 处 `&TokenManager{...}` 字面量（§1 末） | **默认注入 NoopMetrics{}（R2 关键）与默认 clock**，使删 nil 判空后不 panic |
| `func tokenState(m *TokenManager) (source tokenSource, overrideInvalidated bool)` | `token_manager_test.go:160-163, 189-192` | 持 m.mu 读 |
| `func invalidateOverride(m *TokenManager)` | `token_manager_test.go:181-183` | 持 m.mu 写 |
| `func warmupSchedulerDone(svc *ClassroomService) chan struct{}` | `warmup_test.go:168-170, 172-174` | 持 backgroundMu 读 warmupDone；O2 Run/Shutdown 改造（批次④）时只改这里 |
| `func isBackgroundStopping(svc *ClassroomService) bool` | `warmup_test.go:215-217` | 同上 |
| （不做函数，改 Options 字段） | `warmup_test.go:92` warmupJitter 写 | 见 §5 |

覆盖检查：§1 清单中每一处访问都有归宿（seedCache 12、completeRefresh 22+11、forceFailureState 1、backoffState 16、installToken 3、installAPIURL 1、TokenManager 字面量 13、tokenState 2、invalidateOverride 1、warmup 3、Options 字段 1）；`construction_test.go:62`（campuses 拷贝断言）与 refreshMu 显式锁两处随 completeRefresh 消失，无需函数。R8 可执行性成立。

---

## 5. warmup_test.go:92 的 warmupJitter 无同步写入

- **写点**：`service/warmup_test.go:92` `svc.warmupJitter = func() time.Duration { return 0 }`，在 `newTestService`（:91）之后、`svc.StartWarmup(ctx)`（:94）之前，无任何锁。
- **构造默认值**：classroom_service.go:117（`warmupJitter: randomWarmupJitter`，warmup.go:26-28 为 1s + [0,4s) 随机）。
- **唯一读点**：warmup.go:136（`s.warmupJitter()`，warmupLoop goroutine 内，每轮调度调用）。
- **现状为何没被 race detector 抓**：写发生在 `StartWarmup` 的 `go func()`（warmup.go:112-115）之前，Go 内存模型下 goroutine 创建构成 happens-before，**当前不是真数据竞争**；但这是靠"测试作者记得先赋值再 StartWarmup"的纪律保证的隐式契约——字段在结构体里与 backgroundMu 保护的 4 个生命周期字段（classroom_service.go:63-68）混排，却唯独它不受锁保护，audit O8 的指摘点即此脆弱性。
- **改为 `ClassroomServiceOptions.WarmupJitter func() time.Duration` 的影响面**：
  - 生产：classroom_service.go 增加 options 处理（nil → randomWarmupJitter，与 Clock/BackoffRandom 同款三行）；字段可从"可变状态"降为构造后只读，甚至从 struct 移出到局部捕获。
  - 测试：仅 `warmup_test.go:91-92` 一处改为 `newTestServiceWithOptions(t, client, ClassroomServiceOptions{WarmupJitter: func() time.Duration { return 0 }})`。其余 warmup 测试（:127, 158, 198）不设 jitter 也不受影响——它们只依赖"首轮立即刷新"（warmup.go:94 注释），jitter 只影响第二轮之后的间隔且测试在此之前已 cancel。
  - 无其他读写点，无连锁影响。

---

## 6. 现有测试基建清单（设计时勿重复发明）

### mock / fake / stub

| 设施 | 位置 | 说明 |
|---|---|---|
| `mockJWClient` | `service/realtime_data_test.go:31-56` | 3 个函数字段（queryCampus/login/fetchAPIURL）；默认：Login→"mock-token"、FetchAPIURL→DefaultAPIURL、QueryCampus 未配置则报错。全 service 包共用 |
| `fakeClock` | `service/clock_test.go:10-35` | RWMutex 保护的 Now/Set/Advance，注入 Options.Clock |
| `sequenceClock` | `service/token_manager_login_metrics_test.go:43-61` | 按序列返回时刻（测 duration 计算与时钟回拨钳位） |
| `recordingLoginMetrics` | `service/token_manager_login_metrics_test.go:19-41` | 嵌入 NoopMetrics + mutex 记录 ObserveLogin；`snapshot()` 取样 |
| `countingSuppressedMetrics` | `service/refresh_backoff_test.go:176-183` | 嵌入 NoopMetrics + atomic 计数 ObserveRefreshSuppressed |
| `serviceHTTPDoerFunc` | `service/jw_client_test.go:15-19` | `func(*http.Request)(*http.Response,error)` 适配 utils.HTTPDoer；jw_protocol_test.go 全靠它做离线协议 fixture |
| `httpDoerFunc` | `utils/http_test.go:13-17` | 同款适配器（utils 包内独立一份） |
| `fakeClassroomService` | `handler_test.go:25-42`（package main） | 实现 handler 所需接口（GetTodayClassrooms/GetRuntimeStatus/HasUsableTodayCache）；HTTP 层测试不依赖 service 内部 |

### 构造/装配 helper

| 设施 | 位置 | 说明 |
|---|---|---|
| `newTestService` / `newTestServiceWithOverride` / `newTestServiceWithOptions` | `service/realtime_data_test.go:58-95` | 默认双校区 01/04、BackoffRandom=0.5、`cache.New()`（R1 改造点）、t.Cleanup 里 WaitBackground(5s) |
| `newHTTPJWClientForTest` | `service/realtime_data_test.go:97-104` | 真 JWClient + utils.NewHTTPClient（集成 + 1 个离线边界用例共用） |
| `newIntegrationService` | `service/realtime_data_test.go:106-112` | env 凭据装配 |
| `requireJWCredentials` / `requireJWLoginCredentials` | `service/realtime_data_test.go:114-129` | t.Skip 守卫（§3） |
| `newTestHTTPServer` | `handler_test.go:44-56` | main 包 HTTP 装配 |

### 断言/等待 helper

| 设施 | 位置 |
|---|---|
| `waitFor(t, timeout, cond)` | `service/realtime_data_test.go:843-853` |
| `requireCampusByID` / `requireBuildingByName` / `requireRoomByDisplayName` | `service/realtime_data_test.go:502-533` |
| `tokenTestRows(campusID)` | `service/token_manager_test.go:15-21` |
| Prometheus 断言：`gatherFamilies` / `counterValue` / `histogramSample` / `labelsMatch` | `service/prometheus_metrics_test.go:123-182` |
| `init(){ logs.Init(false,false) }` | `service/realtime_data_test.go:25-29`（拆分时须随 helper 迁到 testsupport 文件，且只保留一份） |

---

## Caveats / Not Found

- audit 说 token_manager.go:60-99 的 for 重试环 + forceRefresh 唯一调用链是测试专用 Login——核实：`EnsureToken(ctx, true)` 生产调用仅 `ClassroomService.Login`（realtime_data.go:31-34），Login 生产零调用、仅 TestLogin（集成）使用。属实。但注意 `QueryAll` **不是**只有集成测试可达：8 个单测把它当"同步触发一次刷新"的入口（§3 末行号），R3 改非导出即可，不能直接删。
- audit O8 说 warmup_test.go:92 "无同步写入"——精确说法是**无锁但有 happens-before**（go 语句之前），当前非 race，是脆弱契约而非现行 bug（§5）。
- 主包测试（handler_test/metrics_endpoint_test/router_test/main_test）经 fakeClassroomService 与导出接口工作，不触 service 未导出字段，R1/R2 对它们零影响（与 PRD 验收标准"handler_test 不改断言即通过"一致）。
- `TestFakeClockAndCoordinatorRace`（refresh_backoff_test.go:379-407）依赖 `-race` 下并发调 completeRefresh 语义，种子函数必须保持内部加锁次序与生产一致，否则该测试失去意义。
- 未逐行核对 safe_remote_message_test.go / crypto_test.go / config、logs、utils 包测试的字段访问——grep 证实它们无 ClassroomService/TokenManager 未导出字段访问，仅按主题归档。
