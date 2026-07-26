# Implement：后端减法（执行清单）

每个 Step 结束必须通过门禁：`gofmt -l .`（空）、`go vet ./...`、`go test -race ./...`。每个 Step 一个提交（消息见各步）。

## Step 0：测试脚手架（R8 前半）
- [ ] 新建 `service/export_test.go`：12 个种子函数（签名按 research/test-coupling-map §4）。
- [ ] `warmupJitter` 改 `ClassroomServiceOptions.WarmupJitter`；warmup_test.go:92 改走 Options。
- [ ] 13 处 `&TokenManager{}` 裸构迁移到 `newTokenManagerForTest`（默认注入 NoopMetrics 与测试 clock）。
- [ ] 白盒字段直访（refreshInFlight/refreshAttempt 11 组、backoff 三字段 16 处、svc.cache.Store 12 处、consecutiveTotalFailures 1 处）全部改经种子函数。
- 提交：`test(service): add export_test seams for internal state access`
- 回滚点：纯测试改动，生产代码零变化，可整步 revert。

## Step 1：删 cache 包（R1）
- [ ] `ClassroomService` 字段改 `atomic.Pointer[model.TodayClassrooms]`；Load/Store 两处生产调用点迁移；`Date` 守卫不动。
- [ ] 删 `cacheExpiration` + `TestCacheExpirationAlwaysPositive` + realtime_data_test.go:355 断言。
- [ ] `NewClassroomService` 去 store 参数；main.go 随改；construction_test.go 删 missing-cache/typed-nil-cache 两用例。
- [ ] 删 `cache/` 目录；`go mod tidy`（go-cache 消失）；`go mod tidy -diff` 干净。
- [ ] `TestGetCachedTodayClassroomsRejectsCrossDayCache` 确认通过（守卫测试）。
- 提交：`refactor(service): replace go-cache with atomic.Pointer single-value store`

## Step 2：Metrics 默认 Noop（R2）
- [ ] `NewClassroomService` 默认注入 `NoopMetrics{}`；确保 TokenManager 经此路径拿到非 nil。
- [ ] 删 7 处判空、4 个 observeXxx、7 个 nil-receiver 检查；删 `TestLoginMetricsNilMetricsSafe`。
- [ ] `go test -race ./service`（重点：token_manager_login_metrics_test.go 无 panic）。
- 提交：`refactor(service): default to NoopMetrics and drop nil guards`

## Step 3：死代码（R3）
- [ ] 删 `model.QueryResponse`、`logs.LogIDKey`（同步改 .trellis/spec/backend/logging-guidelines.md:42）、jw_error.go ctx 死分支。
- [ ] 删 `Login`/`forceRefresh`/`loginPerformed`；`EnsureToken` 坍缩单次路径；TestLogin 改调 JWClient.Login。
- [ ] `QueryOne`/`QueryAll` → `queryOne`/`queryAll`。
- 提交：`refactor(service): remove test-only exports and dead code`

## Step 4：机械清理（R4-R7）
- [ ] `errgroupNoCancel` → WaitGroup；循环变量重绑定清理（service 全包 + 测试）。
- [ ] 退避阶梯合并 `backoffLadder`；改 2 个阶梯测试。
- [ ] 删三处 reflect 判空 + 4 个 typed-nil 测试用例。
- [ ] 小项：now() 判空（Classroom+Token 两处）、addQuery 内联、hasJWCredentials 改 bool（handler_test.go:71-104 重写为双配置用例）、`interface{}`→`any`。
- [ ] RuntimeStatus `*time.Time`→`time.Time`+`omitzero` 单独评估：先 grep 前端与 docs 是否消费 null 值语义；有消费者则只删 cloneTime 命名遮蔽。
- 提交：`refactor: mechanical cleanups (backoff ladder, reflect guards, loop vars)`（RuntimeStatus 如做则单独 `refactor(service): use omitzero for RuntimeStatus timestamps`）

## Step 5：测试文件拆分（R8 后半）
- [ ] realtime_data_test.go 按 research 方案拆分；3 个真网络用例移 `//go:build integration`；`newHTTPJWClientForTest` 留非 tag 文件。
- [ ] `go test ./service`（默认跳过集成）与 `go test -tags integration ./service`（无凭据时 skip）均可运行。
- 提交：`test(service): split realtime_data_test and tag integration tests`

## 收尾
- [ ] 全量门禁 + `pnpm --dir frontend test`（防御性，前端不应受影响）。
- [ ] CHANGELOG：本任务对外不可见，仅在内部有 /readyz omitzero 变化时补一条。
- [ ] trellis-check 全量核对 + 对抗验证（契约镜头：/metrics 与 /readyz 字节级对比改造前后；净删镜头：diffstat 确认净删为正）。
- [ ] Phase 3.3 spec 更新：runtime-state-and-cache.md 需重写 cache 一节（存储形态变了）。
