# Research: cache 包消除映射（R1 精确使用点清单）

- **Query**: cache/ 包 + go-cache → atomic.Pointer 替换所需的全部使用点、等价条件与目标形态
- **Scope**: internal
- **Date**: 2026-07-26
- 行号以当前工作区代码为准（已核实：上个子任务改动 main.go/handler.go 后，审计与 PRD 引用的 cache 相关行号均未漂移）。

## 1. cache/cache.go 的 API 面与 go-cache 实际用法

文件共 51 行（cache/cache.go:1-51），倒置依赖 `import "BUPT_EC/service/model"` 在 cache/cache.go:6。

| 成员 | 位置 | 说明 |
|---|---|---|
| `const todayKey = "TODAY_CLASSROOMS_CACHE"` | cache.go:12 | 唯一 key，全包只有这一个 |
| `type TodayClassroomsStore struct{ inner *gocache.Cache }` | cache.go:17-19 | 唯一类型 |
| `New() *TodayClassroomsStore` | cache.go:22-27 | `gocache.New(gocache.NoExpiration, time.Minute)` — janitor goroutine 每分钟清理一次（go-cache 内部 runJanitor + finalizer） |
| `Load() (*model.TodayClassrooms, bool)` | cache.go:30-43 | nil-receiver 防御（:31）、`inner.Get` miss、类型断言失败三条错误路径都返回 `(nil,false)` |
| `Store(value, expiration time.Duration)` | cache.go:46-51 | nil-receiver / nil-value 直接忽略（:47）；`inner.Set(todayKey, value, expiration)` |

go-cache 只用到 `New/Get/Set` 三个函数；`Delete/Flush/Items` 等一律未用。关键事实：**go-cache 的过期判断用真实 `time.Now()`，与 service 注入的 Clock 无关**——fake clock 测试里 TTL 从未真正生效，跨天拒绝一直由 Date 校验完成（见 §5）。

依赖：go.mod:9 `github.com/patrickmn/go-cache v2.1.0+incompatible`（无其他包引用它，grep 全仓仅 cache/cache.go:8 一处 import）。

## 2. TodayClassroomCache 接口：实现者与消费者

接口定义：service/classroom_service.go:37-40（注释 :35-36）。**唯一实现者**：`cache.TodayClassroomsStore`。

消费者（生产代码）：
| 位置 | 用途 |
|---|---|
| service/classroom_service.go:46 | `ClassroomService.cache` 字段 |
| service/classroom_service.go:92 | `NewClassroomService(options, store TodayClassroomCache, client)` 构造参数 |
| service/classroom_service.go:93-95 | `isNilDependency(store)` 校验（reflect 判空，R6 相关） |
| service/classroom_service.go:112 | 赋值 `cache: store` |
| main.go:14 | `import "BUPT_EC/cache"` |
| main.go:46 | `store := cache.New()` |
| main.go:60 | 传入 `NewClassroomService(..., store, jwClient)` |

生产代码 Load/Store 调用点（全部）：
| 位置 | 调用 | expiration 来源 |
|---|---|---|
| service/realtime_data.go:273 | `s.cache.Store(today, cacheExpiration(now, today.StaleUntil))` | **唯一生产 Store**；expiration = StaleUntil−now（钳到 ≥1s） |
| service/realtime_data.go:324 | `s.cache.Load()`（在 getCachedTodayClassroomsAt 内） | **唯一生产 Load** |

测试中的 Load/Store 与构造调用点：
| 位置 | 测试 | expiration |
|---|---|---|
| service/realtime_data_test.go:83 | `newTestServiceWithOptions` 里 `cache.New()` — 全部 service 单测的装配点 | — |
| service/realtime_data_test.go:270,281 | TestGetCachedTodayClassroomsRejectsCrossDayCache | time.Minute |
| service/realtime_data_test.go:382 | TestGetTodayClassroomsReturnsFreshCacheWithoutJWQuery | time.Hour |
| service/realtime_data_test.go:586 | TestGetTodayClassroomsReturnsStaleWhileRefreshContinues | time.Hour |
| service/realtime_data_test.go:746 | TestGetTodayClassroomsBacksOffAfterStaleRefreshFailure | time.Hour |
| service/realtime_data_test.go:1087 | TestDoRefreshPartialCampusMergesPreviousCache | time.Hour |
| service/realtime_data_test.go:1146 | TestStalePartialCacheUsesLatestTotalRefreshFailure | time.Hour |
| service/realtime_data_test.go:1214 | TestGetTodayClassroomsRetriesPartialErrorWithinFreshTTL | time.Hour |
| service/realtime_data_test.go:1299 | TestGetTodayClassroomsPartialErrorRefreshCanRecoverFailedCampus | time.Hour |
| service/realtime_data_test.go:1359,1370 | TestRuntimeStatusCacheStaleOnlyWhenPastFreshTTL | time.Hour |
| service/refresh_backoff_test.go:323 | TestBackoffCrossingMidnightRejectsOldCacheThenAllowsNewDayRefresh | time.Hour |
| service/construction_test.go:7,17,19,25,28-31,55 | TestNewClassroomServiceValidatesDependencies…（含 typed-nil store 用例）+ TestNewClassroomServiceCopiesCampusOptions | — |

结论：**所有测试传的 expiration 都是哑值**（time.Minute/time.Hour，测试运行远短于此，从不依赖 TTL 真正过期）。删掉 expiration 参数不改变任何测试语义。

## 3. cacheExpiration 逻辑与调用点

定义：service/realtime_data.go:285-291（注释 :281-284，与 PRD 引用的 281-291 一致）。逻辑：`d = staleUntil.Sub(now)`；`d < 1s` 时钳为 `1s`（因 go-cache 把非正 duration 视作永不过期）。

调用点：
- 生产：realtime_data.go:273（唯一）。
- 测试：realtime_data_test.go:295,304,307,310（TestCacheExpirationAlwaysPositive，292-313 整个测试专测此函数）；realtime_data_test.go:355（TestDoRefreshStampsCacheAtCompletionAcrossMidnight 内一条断言 `cacheExpiration(afterMidnight, resp.StaleUntil) > 0`）。

删除 cacheExpiration 后：TestCacheExpirationAlwaysPositive 整体删除；:355 一行断言删除（该测试其余部分保留，见 §6）。

## 4. Date/ExpiresAt/StaleUntil/Stale 完整引用图

字段定义：service/model/realtime_data.go:68-77（含 UpdatedAt :70）。JSON 契约被前端消费（reloadSchedule.js / classroomDataValidity.js / App.jsx），不可动。

### Date（string "2006-01-02"）
- 写（生产）：realtime_data.go:263（doRefresh 完成时刻盖章）。
- 读（生产）：**realtime_data.go:328 跨天拒绝（核心守卫）**；runtime_status.go:66（/readyz 的 CacheDate）。
- 读（测试）：realtime_data_test.go:287,342-343,363-364；refresh_backoff_test.go:348,364；cache/cache_test.go:13,16（随包删除）。

### ExpiresAt（= now + classroomFreshTTL 5m）
- 写（生产）：realtime_data.go:265。
- 读（生产）：realtime_data.go:47（fresh 判定）；runtime_status.go:61（CacheFresh）。
- 读（前端）：reloadSchedule 用 expires_at 定轮询间隔。
- 测试：handler_test.go:152（fake service，不经过 cache）；realtime_data_test.go:348-349 及各 seed。

### StaleUntil（= endOfDay(now)，恒等于当日次日零点）
- 写（生产）：realtime_data.go:266（唯一写入点，佐证审计"StaleUntil 恒等于 endOfDay(Date)"）。
- 读（生产）：realtime_data.go:57（soft-stale 窗口判定）；realtime_data.go:273（经 cacheExpiration → **随替换消失**）；warmup.go:159（warmupCacheState）；runtime_status.go:63（CacheStale）；runtime_status.go:73（HasUsableTodayCache → handler.go:85 /readyz 就绪判定）。
- 读（前端）：classroomDataValidity.js:25-30（stale_until 缺失/非法即判无效）。
- 测试：refresh_backoff_test.go:327,341-342；realtime_data_test.go:351-353,366-367 及各 seed。

### Stale（bool，响应期字段）
- 写（生产）：realtime_data.go:267（Store 时恒 false）；realtime_data.go:349（classroomResponse 按响应场景在副本上改写——缓存内的值从不被就地修改）。
- 读（生产）：后端无读者（只写）。前端 App.jsx:103 `resp.data?.stale` 决定横幅。
- 测试：handler_test.go:183；realtime_data_test.go:396,447,602,660,717,758,770,1163,1233。

### UpdatedAt（附带核查，PRD R1 措辞涉及）
- 写（生产）：realtime_data.go:264。读（生产）：**无**——跨天判断用的是 Date 字符串比较（realtime_data.go:328），不是 UpdatedAt。读（测试）：realtime_data_test.go:345。JSON `updated_at` 暴露给前端。
- ⚠️ PRD R1 写"改用 atomic.Pointer（含 UpdatedAt 判断）"与现状不符：现有守卫是 Date 判断，且 PRD 自己声明四字段语义不动 → design.md 应保留 Date 判断原样，不引入 UpdatedAt 判断。

## 5. getCachedTodayClassroomsAt 跨天逻辑与 atomic.Pointer 等价条件

现状（realtime_data.go:319-332）：
```go
cached, ok := s.cache.Load()          // :324
if !ok || cached == nil { return nil, false }
if cached.Date != now.In(businessLocation).Format("2006-01-02") { return nil, false }  // :328
return cached, true
```

当前 Load miss 的三种来源：(a) 从未 Store；(b) go-cache TTL 到期（真实墙钟越过 Store 时的 StaleUntil）；(c) 类型断言失败（typed API 下不可能）。替换后 (b) 必须被 :328 的 Date 校验完全覆盖。

**等价性论证**（替换为 atomic.Pointer 后行为不变所需条件，均已满足）：
1. 生产唯一 Store 点用同一个 `now` 写 Date 与 StaleUntil=endOfDay(now)（realtime_data.go:262-273）→ TTL 到期时刻 == 次日零点 == Date 失配时刻。任何 t ≥ StaleUntil 必有 Date(t) ≠ cached.Date → Date 校验拒绝，与 TTL miss 等价。
2. 时钟源差异反而消除：go-cache 用真实 time.Now()，Date 校验用注入 Clock。生产两者同为墙钟（等价）；fake-clock 测试里 TTL 本来就永不触发，Date 校验一直是唯一生效守卫（refresh_backoff_test.go:347-349 正是靠它通过）→ 替换后更确定，无测试依赖 TTL 行为。
3. cacheExpiration 的 1s 钳位路径（StaleUntil ≤ now 的防御分支）在生产不可达（完成时刻盖章保证 StaleUntil 在未来）；测试也未构造"同日但已过 StaleUntil"且依赖 1s 窗口的用例。可安全消失。
4. 并发/不可变性：go-cache 内部 RWMutex → atomic.Pointer 的原子 Load/Store 等价，但前提是**缓存值 Store 后不被就地修改**——现状已满足（classroomResponse 在副本上改 Stale/Error，realtime_data.go:344-357；doRefresh 只读 prev.Campuses，realtime_data.go:230-234）。design 应把该不变量写明。
5. nil 语义：现 Store 忽略 nil（cache.go:47），生产从不传 nil；`atomic.Pointer.Store(nil)` 会清空缓存——靠调用纪律即可（唯一生产 Store 点 today 恒非 nil），Load 侧 `cached == nil` 判断保留（对应现 :325 的 `!ok || cached == nil`）。
6. 内存：无 janitor 后，昨日 payload 会驻留到下次成功刷新覆盖为止（约一份双校区 JSON，可忽略；janitor goroutine 与 finalizer 随之消失，测试更干净）。

附带核实审计声明：realtime_data.go:57 `now.Before(cached.StaleUntil)` 对生产写入的条目恒真（同日 ⇒ 早于 endOfDay）✓；但注意 :72-93 的 fall-through 并非整体死代码——它是**缓存完全 miss（冷启动/跨天）** 的必经路径，死的只是"cached 存在但已过 StaleUntil"这一进入方式。本次不动（批次④）。

## 6. cache_test.go 用例处置

| 用例 | 位置 | 处置 |
|---|---|---|
| TestTodayClassroomsStoreIsInstanceLocal | cache_test.go:10-22 | **删除，无需迁移**。实例隔离由"每个 ClassroomService 自持 atomic.Pointer 字段"在构造上保证，全部 service 单测并行创建独立实例即隐式覆盖 |
| TestTodayClassroomsStoreRejectsWrongTypes | cache_test.go:24-30 | **删除，无需迁移**。类型安全由 `atomic.Pointer[model.TodayClassrooms]` 编译期保证，interface{} 断言路径不复存在 |

service 侧受影响测试：
- **删除**：TestCacheExpirationAlwaysPositive（realtime_data_test.go:292-313）——被测函数消失。
- **删一行**：realtime_data_test.go:355（cacheExpiration 断言），TestDoRefreshStampsCacheAtCompletionAcrossMidnight 其余保留（它是完成时刻盖章 + Date/StaleUntil 一致性的关键回归测试）。
- **改装配**：construction_test.go 删 "missing cache"/"typed nil cache" 两个 case（:28-29）与 `BUPT_EC/cache` import；realtime_data_test.go:83 与 :22 import 同理。
- **改 seed 方式**（12 处 `svc.cache.Store(v, dur)` → seed 函数，见 §7）：§2 表中所有测试 Store 点。
- **升格为守卫测试**：TestGetCachedTodayClassroomsRejectsCrossDayCache（realtime_data_test.go:267-290）替换后原样保留（去掉 duration 参数），它就是 §5 等价条件 1 的存储层回归测试。

## 7. 目标形态建议

- **类型**：service/classroom_service.go 字段 `cache TodayClassroomCache`（:46）→ `todayCache atomic.Pointer[model.TodayClassrooms]`（import `sync/atomic`）；删除接口定义 :35-40。不新建文件。
- **构造**：`NewClassroomService(options ClassroomServiceOptions, client JWClient)` —— 去掉 store 参数与 :93-95 的 store 判空。调用方同步改：main.go:46（删 `store := cache.New()`）、main.go:60、construction_test.go:37,53、realtime_data_test.go:83。main.go:14 import 删除。
- **Store**：realtime_data.go:273 → `s.todayCache.Store(today)`；删 cacheExpiration（:281-291）。
- **Load**：realtime_data.go:323-332 保持函数名与 Date 判断不变，仅存储层替换：
  ```go
  func (s *ClassroomService) getCachedTodayClassroomsAt(now time.Time) (*model.TodayClassrooms, bool) {
      cached := s.todayCache.Load()
      if cached == nil {
          return nil, false
      }
      if cached.Date != now.In(businessLocation).Format("2006-01-02") {
          return nil, false
      }
      return cached, true
  }
  ```
- **测试 seed**（配合 PRD R8）：在 service 包测试侧提供 `func (s *ClassroomService) seedTodayCache(v *model.TodayClassrooms)`（或 export_test.go 的 seedCache 函数）内部 `s.todayCache.Store(v)`；12 处调用点去掉 duration 实参。
- **删除**：`cache/` 整目录（cache.go + cache_test.go）；go.mod:9 与 go.sum:64-65 经 `go mod tidy` 清除。
- **验收 grep**：替换后 `patrickmn|go-cache|gocache` 在 `*.go` 应零命中（当前命中仅 cache/cache.go:8,15,18,25 + realtime_data.go:281-282 注释 + realtime_data_test.go:303 注释——后两处注释也要随删随改）。

## Caveats / Not Found

- ⚠️ PRD R1 "（含 UpdatedAt 判断）"与代码不符：现行跨天守卫是 Date 字符串比较（realtime_data.go:328），UpdatedAt 生产无读者。design.md 应按"保留 Date 判断"落地（详见 §4 UpdatedAt 小节）。
- 文档/规格引用 cache 包，需主代理在收尾时同步：docs/development.md:92、.trellis/spec/backend/runtime-state-and-cache.md:15、.trellis/spec/backend/directory-structure.md:19（research 无权修改）。
- handler.go 通过接口消费 service（handler.go:18-20），handler_test.go 用 fakeClassroomService（handler_test.go:148-158），HTTP 层与本替换零耦合，无需改动。
- 审计所有 cache 相关行号核实无漂移（cache.go:1-51、classroom_service.go:37-40、realtime_data.go:57/273/281-291/328、model/realtime_data.go:68-77 均与当前代码一致）。
