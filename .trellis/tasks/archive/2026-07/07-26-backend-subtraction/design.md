# Design：后端减法

依据：`research/cache-removal-map.md`、`research/metrics-deadcode-map.md`、`research/test-coupling-map.md`（行号均已按当前代码核实）。

## 边界与不变量

**对外契约完全不变**：`/api/get_data`、`/healthz`、`/readyz`、`/metrics` 的响应结构、字段、指标名与标签、日志格式均不动。`model.TodayClassrooms` 的 `Date`/`ExpiresAt`/`StaleUntil`/`Stale` 四字段语义不动（过期模型收敛属批次④）。

**研究修正的关键设计约束**（与审计/PRD 初稿的偏差）：
1. 跨天守卫保留 `Date` 字符串比较（realtime_data.go:328）——`UpdatedAt` 在生产代码无任何读者，不引入新判断。
2. metrics 接口实名 `RuntimeMetrics`（service/metrics.go:7-14），非审计所称 ClassroomMetrics。
3. `QueryAll` 被 8 个纯单测当作同步刷新入口——改非导出 `queryAll`，不能删。
4. 删 TokenManager 的 metrics 判空（token_manager.go:206）前必须先落 `newTokenManagerForTest`（13 处 `&TokenManager{}` 裸构中 5 处会走 observeLogin → 运行时 panic）。
5. `TokenManager.now` 的 clock 判空当前被 11 处无 clock 测试直构承重——同样依赖 4 的种子函数先行。

## 目标形态

### R1 存储层（删 cache/ 包）

```go
// classroom_service.go
type ClassroomService struct {
    ...
    todayCache atomic.Pointer[model.TodayClassrooms]  // 替换 cache TodayClassroomCache 字段
}
```
- `getCachedTodayClassroomsAt`：只换读取来源（`s.todayCache.Load()`），`Date` 判断原样保留（等价性依据：生产 Store 时 TTL 到期时刻 == Date 失配时刻，见 cache-removal-map §3）。
- 写入点 realtime_data.go:273：`s.todayCache.Store(&data)`，删 `cacheExpiration` 与其测试。
- `NewClassroomService` 去掉 store 参数与判空；main.go:46,60 随改。
- 删除：`cache/` 整目录、`TodayClassroomCache` 接口、go.mod 的 go-cache。
- `TestGetCachedTodayClassroomsRejectsCrossDayCache` 升格为存储替换守卫测试。

### R2 Metrics 默认 Noop

- `NewClassroomService`：`if options.Metrics == nil { options.Metrics = NoopMetrics{} }`（与 clock/backoffRandom 同风格）。
- TokenManager 构造同样保证非 nil（经 ClassroomService 注入路径）。
- 删除：7 处 `!= nil` 判空（realtime_data.go:96,166,172；refresh_coordinator.go:123,133,161；token_manager.go:206）、4 个 observeXxx 包装、PrometheusMetrics 全部 7 个 nil-receiver 检查（无测试依赖）。
- 删除 `TestLoginMetricsNilMetricsSafe`（语义随判空消失）。

### R3 死代码

| 目标 | 处置 | 连带 |
|---|---|---|
| `model.QueryResponse` | 删（0 引用，解析走 jwResponseEnvelope） | — |
| `logs.LogIDKey` | 删（0 Go 引用） | 同步更新 .trellis/spec/backend/logging-guidelines.md:42 |
| jw_error.go:66-68 ctx 死分支 | 删 | — |
| `ClassroomService.Login` + `forceRefresh` + `loginPerformed` | 删；`EnsureToken(ctx)` 坍缩为无循环单次路径 | TestLogin 改直接调 JWClient.Login |
| `QueryOne`/`QueryAll` | 改非导出 | 白盒测试同包，零破坏 |

### R4-R7 机械清理

- `errgroupNoCancel` → 内联 `sync.WaitGroup`；清理 Go 1.22 前循环变量重绑定。
- 退避阶梯合并为 `backoffLadder(attempt int) time.Duration`（研究确认两套全定义域逐点相等 0/1→30s,2→1m,3→2m,≥4→5m；消费侧差异——协调器加 jitter、warmup 相对睡眠——保留在各自调用处）。
- reflect 判空：直接删三处，构造校验退化为 `== nil`；4 个 typed-nil 测试用例同步删除（防御对象是仓库内部调用方，测试断言的是已不存在的承诺）。
- 小项：删 `ClassroomService.now` 判空（保留 TokenManager.now 判空直到种子函数落地后一并删）；`RuntimeStatus` 指针字段改 `time.Time` + `omitzero`（go 1.25 支持，JSON 输出等价：零值时字段省略 vs 现在 null——**注意**：这是 /readyz 可见变化，须核对前端/文档无消费者后再做，否则回退为保留指针只删 cloneTime 遮蔽命名）；`addQuery` 内联；`hasJWCredentials func() bool` → `bool`（handler_test.go:71-104 的闭包晚绑定测试改为直接构造两种配置各测一次）；`interface{}` → `any`（6 处）。

### R8 测试收敛

- `export_test.go` 落 12 个种子函数（签名见 test-coupling-map §4）：seedCache / completeRefresh / forceFailureState / backoffState / installToken / installAPIURL / newTokenManagerForTest / tokenState / invalidateOverride / warmupSchedulerDone / isBackgroundStopping；`warmupJitter` 改 `ClassroomServiceOptions.WarmupJitter`（生产 3 行 + 测试 1 处）。
- `realtime_data_test.go`（1438 行 36 用例）按研究的方案拆分；3 个真网络用例移入 `//go:build integration` 文件（helper `newHTTPJWClientForTest` 留在非 tag 文件）。

## 实施顺序（风险最小化）

种子函数必须最先落（R2/R3 的前置）；cache 替换次之（改动面小、守卫测试明确）；metrics 与死代码再次；机械清理与测试拆分收尾。详见 implement.md。

## 风险与回滚

- 每个阶段独立提交，任一阶段 `go test -race ./...` 不绿即回滚该阶段。
- 最大风险点：R2 的测试运行时 panic（已用种子函数前置化解）；R7 的 RuntimeStatus omitzero 属 /readyz 可见变化，单独小步提交，发现前端/运维依赖立即回退。
- `/metrics` 全程用 metrics_endpoint_test.go 的现有断言作为守卫。
